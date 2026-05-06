package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/atlas/trace"
	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/maturity"
	"github.com/iodesystems/zdx-go/internal/workflowhints"
)

type soloCandidate struct {
	Key           string
	Title         string
	Description   string
	Text          string
	Kind          string
	TargetType    string
	TargetID      string
	IssueRef      string
	TargetBranch  string
	Priority      int32
	Blocked       bool
	BlockedReason string
	Persona       string
}

// foldIssuePriority lowers a base candidate priority (lower=wins) by the
// user-facing issue priority so a P1/P2 impl with a ready dev task outranks
// low-value triage. issuePriority is the string form from zdx_issues.priority
// ("1".."4" or ""). Unknown/empty is treated as P4 (no adjustment).
func foldIssuePriority(base int32, issuePriority string) int32 {
	p := 4
	switch issuePriority {
	case "1":
		p = 1
	case "2":
		p = 2
	case "3":
		p = 3
	}
	return base - int32(5-p)*5
}

func (h *Handler) generateSoloQueue(ctx context.Context, projectID int32, issueFilter string, autonomousMode bool) ([]soloCandidate, error) {
	var candidates []soloCandidate

	issues, err := h.Q.ListOpenIssues(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Filter to specific issue if requested
	if issueFilter != "" {
		var filtered []db.ZdxIssue
		for _, iss := range issues {
			if iss.ID == issueFilter {
				filtered = append(filtered, iss)
				break
			}
		}
		issues = filtered
	}

	// Autonomous agents skip interactive-only issues.
	if autonomousMode {
		var filtered []db.ZdxIssue
		for _, iss := range issues {
			if !iss.InteractiveOnly {
				filtered = append(filtered, iss)
			}
		}
		issues = filtered
	}

	// Exclude tracker issues — they are closed by their children, never actionable directly.
	// Closable trackers (all blockers closed) still surface via the close:tracker candidate below.
	var trackerIssues []db.ZdxIssue
	{
		var filtered []db.ZdxIssue
		for _, iss := range issues {
			if iss.IssueType == "tracker" {
				trackerIssues = append(trackerIssues, iss)
			} else {
				filtered = append(filtered, iss)
			}
		}
		issues = filtered
	}

	// Build ancestor-sequencing-blocked set: an issue is gated if any of its
	// composition ancestors (or itself) has an open sequencing blocker. This is
	// what makes `dx issue block IS-tracker --by IS-blocker` actually gate the
	// tracker's children's claim-eligibility — without it, blocking a tracker
	// has no operational effect on the queue. (IS-656 Half B / TK-1435.)
	ancestorBlocked := map[string]string{} // issue_id -> reason
	if rows, err := h.Q.ListAncestorSequencingBlockers(ctx, projectID); err == nil {
		for _, r := range rows {
			if _, exists := ancestorBlocked[r.ChildID]; exists {
				continue // first blocker wins for the reason string
			}
			if r.GatedAncestor == r.ChildID {
				ancestorBlocked[r.ChildID] = fmt.Sprintf("blocked by %s", r.BlockerID)
			} else {
				ancestorBlocked[r.ChildID] = fmt.Sprintf("ancestor %s blocked by %s", r.GatedAncestor, r.BlockerID)
			}
		}
	}

	// Build blocked-by-BQ set
	bqBlocked := map[string]bool{}
	pendingBQs, _ := h.Q.ListPendingBlockerQuestions(ctx, projectID)
	for _, q := range pendingBQs {
		if q.TargetType == "issue" {
			for _, iss := range issues {
				if iss.ID == q.TargetID {
					if issueFilter != "" {
						candidates = append(candidates, soloCandidate{
							Key:           fmt.Sprintf("bq-%d", q.ID),
							Title:         fmt.Sprintf("Answer BQ-%d on %s", q.ID, iss.ID),
							Description:   q.Context,
							Text:          q.Context,
							Kind:          "clarify",
							TargetType:    "blocker_question",
							TargetID:      fmt.Sprintf("BQ-%d", q.ID),
							IssueRef:      iss.ID,
							Priority:      5,
							Blocked:       true,
							BlockedReason: fmt.Sprintf("Blocker question BQ-%d pending: owner must answer before this issue can proceed", q.ID),
							Persona:       "owner",
						})
					} else {
						bqBlocked[q.TargetID] = true
					}
					break
				}
			}
		}
	}

	// Filter BQ-blocked issues in global mode
	if issueFilter == "" && len(bqBlocked) > 0 {
		var filtered []db.ZdxIssue
		for _, iss := range issues {
			if !bqBlocked[iss.ID] {
				filtered = append(filtered, iss)
			}
		}
		issues = filtered
	}

	// Surface / filter ancestor-sequencing-blocked issues. In issueFilter mode emit
	// a candidate so the user sees why their target is gated; in global mode filter
	// the issue out so unclaimable work doesn't bubble up.
	if issueFilter != "" && len(ancestorBlocked) > 0 {
		for _, iss := range issues {
			if reason, blocked := ancestorBlocked[iss.ID]; blocked {
				candidates = append(candidates, soloCandidate{
					Key:           fmt.Sprintf("ancestor-blocked-%s", iss.ID),
					Title:         fmt.Sprintf("%s gated: %s", iss.ID, reason),
					Description:   reason,
					Text:          fmt.Sprintf("%s cannot be claimed: %s. Resolve the upstream blocker, then this work re-enters the queue.", iss.ID, reason),
					Kind:          "ancestor-blocked",
					TargetType:    "issue",
					TargetID:      iss.ID,
					IssueRef:      iss.ID,
					Priority:      99,
					Blocked:       true,
					BlockedReason: reason,
					Persona:       "owner",
				})
			}
		}
	}
	if issueFilter == "" && len(ancestorBlocked) > 0 {
		var filtered []db.ZdxIssue
		for _, iss := range issues {
			if _, blocked := ancestorBlocked[iss.ID]; !blocked {
				filtered = append(filtered, iss)
			}
		}
		issues = filtered
	}

	features, _ := h.Q.ListFeatures(ctx, projectID)

	// Surface read:comments for issues with unread comments (newer than the
	// per-target seen high-water mark), and for features only when the last
	// comment has no author_alias (human posted last, agent hasn't replied).
	//
	// IS-1040: prior to per-target read tracking this listed every issue with
	// any comment, so the synthetic todo regenerated every iteration and the
	// agent could never clear it. The seen high-water mark advances when
	// solo/release resolves a read:comments todo, so subsequent regens skip
	// targets the agent has already processed.
	{
		targetsWithComments, _ := h.Q.ListTargetsWithUnreadComments(ctx, projectID)
		allIssues, _ := h.Q.ListIssues(ctx, projectID)
		allIssuesByID := map[string]db.ZdxIssue{}
		for _, iss := range allIssues {
			allIssuesByID[iss.ID] = iss
		}
		for _, t := range targetsWithComments {
			if t.TargetType != "issue" {
				continue
			}
			if issueFilter != "" && t.TargetID != issueFilter {
				continue
			}
			iss, ok := allIssuesByID[t.TargetID]
			if !ok {
				continue
			}
			hint := workflowhints.UnreadCommentsText(iss.ID, iss.Title)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("comment-issue-%s", iss.ID),
				Title:       hint.Title,
				Description: hint.Description,
				Text:        hint.Instructions,
				Kind:        "read:comments",
				TargetType:  "issue",
				TargetID:    iss.ID,
				IssueRef:    iss.ID,
				Priority:    5,
				Persona:     "dev",
			})
		}
		if issueFilter == "" {
			pendingFeatures, _ := h.Q.ListFeaturesWithPendingComments(ctx, projectID)
			for _, featureID := range pendingFeatures {
				hint := workflowhints.UnreadFeatureCommentsText(featureID)
				candidates = append(candidates, soloCandidate{
					Key:         fmt.Sprintf("comment-feature-%s", featureID),
					Title:       hint.Title,
					Description: hint.Description,
					Text:        hint.Instructions,
					Kind:        "read:comments",
					TargetType:  "feature",
					TargetID:    featureID,
					Priority:    8,
					Persona:     "dev",
				})
			}
		}
	}

	// Unanswered QA questions
	if issueFilter == "" {
		unanswered, _ := h.Q.ListUnansweredQuestions(ctx, projectID)
		for _, q := range unanswered {
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("qa-%d", q.ID),
				Title:       fmt.Sprintf("Answer QA-%d", q.ID),
				Description: q.Question,
				Text:        q.Question,
				Kind:        "answer",
				TargetType:  "qa",
				TargetID:    fmt.Sprintf("QA-%d", q.ID),
				Priority:    10,
				Persona:     "dev",
			})
		}
	}

	// Active discussions whose tail message is from the user — surface a todo
	// so an agent picks up the dangling thread.
	if issueFilter == "" {
		pending, _ := h.Q.ListDiscussionsAwaitingResponse(ctx, projectID)
		for _, d := range pending {
			title := d.Title
			if title == "" {
				title = "Untitled discussion"
			}
			dsID := fmt.Sprintf("DS-%d", d.ID)
			dh := workflowhints.RespondDiscussionText(dsID, title, d.Content)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("discussion-%d", d.ID),
				Title:       dh.Title,
				Description: dh.Description,
				Text:        dh.Instructions,
				Kind:        "respond:discussion",
				TargetType:  "discussion",
				TargetID:    dsID,
				Priority:    10,
				Persona:     "dev",
			})
		}
	}

	// Project health checks (global only)
	if issueFilter == "" {
		goalCount, _ := h.Q.CountProjectGoals(ctx, projectID)
		if goalCount == 0 {
			gh := workflowhints.NoGoalsText()
			candidates = append(candidates, soloCandidate{
				Key: "health-goals", Title: gh.Title, Description: gh.Description, Text: gh.Instructions,
				Kind: "owner:goals", TargetType: "project", Priority: 15, Persona: "owner",
			})
		}
		closedTaskCount, _ := h.Q.CountClosedTasks(ctx, projectID)
		if closedTaskCount > 0 {
			var ownerDate, techDate string
			if oe, err := h.Q.GetLatestJournalEntry(ctx, db.GetLatestJournalEntryParams{ProjectID: projectID, Role: "owner"}); err == nil {
				ownerDate = oe.Date
			}
			if te, err := h.Q.GetLatestJournalEntry(ctx, db.GetLatestJournalEntryParams{ProjectID: projectID, Role: "tech"}); err == nil {
				techDate = te.Date
			}
			now := time.Now()
			if ownerDate != "" {
				if t, err := time.Parse("2006-01-02", ownerDate); err == nil && now.Sub(t) > 7*24*time.Hour {
					oh := workflowhints.StandupOverdueText("owner")
					candidates = append(candidates, soloCandidate{
						Key: "health-owner-standup", Title: oh.Title, Description: oh.Description, Text: oh.Instructions,
						Kind: "owner:standup", TargetType: "project", Priority: 18, Persona: "owner",
					})
				}
			}
			if techDate != "" {
				if t, err := time.Parse("2006-01-02", techDate); err == nil && now.Sub(t) > 7*24*time.Hour {
					th := workflowhints.StandupOverdueText("tech")
					candidates = append(candidates, soloCandidate{
						Key: "health-tech-standup", Title: th.Title, Description: th.Description, Text: th.Instructions,
						Kind: "tech:standup", TargetType: "project", Priority: 18, Persona: "tech",
					})
				}
			}
		}

		for _, r := range []string{"owner", "tech"} {
			if _, err := h.Q.GetUnreviewedJournalEntry(ctx, db.GetUnreviewedJournalEntryParams{ProjectID: projectID, Role: r}); err == nil {
				jh := workflowhints.JournalReviewText(r)
				candidates = append(candidates, soloCandidate{
					Key: fmt.Sprintf("journal-review-%s", r), Title: jh.Title, Description: jh.Description, Text: jh.Instructions,
					Kind: r + ":journal-review", TargetType: "project", Priority: 20, Persona: r,
				})
			}
		}
	}

	// Untriaged issues (no priority)
	for _, iss := range issues {
		if iss.Priority == "" {
			th := workflowhints.TriageText(iss.ID, iss.Title)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("triage-%s", iss.ID),
				Title:       th.Title,
				Description: th.Description,
				Text:        th.Instructions,
				Kind:        "triage",
				TargetType:  "issue",
				TargetID:    iss.ID,
				IssueRef:    iss.ID,
				Priority:    20,
				Persona:     "owner",
			})
		}
	}

	// Cross-cutting checks (global only)
	if issueFilter == "" {
		for _, f := range features {
			specs, _ := h.Q.ListSpecs(ctx, f.ID)
			if len(specs) == 0 {
				sh := workflowhints.NoSpecsText(f.Name)
				candidates = append(candidates, soloCandidate{
					Key:         fmt.Sprintf("spec-missing-%s", f.Name),
					Title:       sh.Title,
					Description: sh.Description,
					Text:        sh.Instructions,
					Kind:        "owner:spec",
					TargetType:  "feature",
					TargetID:    f.Name,
					Priority:    25,
					Persona:     "owner",
				})
			}
		}

		staleFeatures, _ := h.Q.ListStaleFeatures(ctx, db.ListStaleFeaturesParams{ProjectID: projectID, StaleDays: 30})
		for _, f := range staleFeatures {
			sfh := workflowhints.StaleFeatureText(f.Name)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("review-feature-%s", f.Name),
				Title:       sfh.Title,
				Description: sfh.Description,
				Text:        sfh.Instructions,
				Kind:        "owner:review",
				TargetType:  "feature",
				TargetID:    f.Name,
				Priority:    28,
				Persona:     "owner",
			})
		}

		uncoveredSpecs, _ := h.Q.ListUncoveredSpecs(ctx, projectID)
		for _, sp := range uncoveredSpecs {
			// IS-495: skip the nudge if a wip/ready/active task already
			// exists for this spec. Without this the nudge re-fires each
			// session and (without server-side dedupe) historically spawned
			// duplicate tasks per spec.
			existing, _ := h.Q.ListOpenTasksByTitlePrefix(ctx, db.ListOpenTasksByTitlePrefixParams{
				ProjectID: projectID,
				Prefix:    fmt.Sprintf("Test spec %d:", sp.ID),
			})
			if len(existing) > 0 {
				continue
			}
			nth := workflowhints.NoTestRefsText(sp.ID, sp.Description, sp.FeatureName)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("test-ref-%d", sp.ID),
				Title:       nth.Title,
				Description: nth.Description,
				Text:        nth.Instructions,
				Kind:        "tech:test-ref",
				TargetType:  "spec",
				TargetID:    fmt.Sprintf("%d", sp.ID),
				Priority:    30,
				Persona:     "tech",
			})
		}

		demoGaps, _ := h.Q.ListSpecsWithoutDemos(ctx, projectID)
		for _, sp := range demoGaps {
			dh := workflowhints.DemoGapText(sp.ID, sp.Description, sp.FeatureName)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("demo-gap-%d", sp.ID),
				Title:       dh.Title,
				Description: dh.Description,
				Text:        dh.Instructions,
				Kind:        "owner:demo-gap",
				TargetType:  "spec",
				TargetID:    fmt.Sprintf("%d", sp.ID),
				Priority:    32,
				Persona:     "owner",
			})
		}

		// ── Maturity nudges (stamped items) ─────────────────────────────
		// Source of truth is zdx_maturity_items, populated by the
		// questionnaire engine (IS-358). Ensure baseline items exist on
		// first contact for pre-questionnaire projects, then surface every
		// open item — including snoozed items whose snooze_until has passed.
		if hasAnswers, err := maturity.HasAnyAnswers(ctx, h.Q, projectID); err == nil && !hasAnswers {
			_, _ = maturity.StampBaseline(ctx, h.Q, projectID)
		}
		_ = h.Q.FlipExpiredMaturityItems(ctx, projectID)

		openItems, _ := h.Q.ListOpenMaturityItems(ctx, projectID)
		for _, it := range openItems {
			mh := workflowhints.MaturityTextForKind(it.Kind, it.Title, it.Description)
			targetType := it.TargetType
			if targetType == "" {
				targetType = "project"
			}
			targetID := ""
			if it.TargetID.Valid {
				targetID = fmt.Sprintf("%d", it.TargetID.Int32)
			}
			persona := "owner"
			if strings.HasPrefix(it.Kind, "tech:") {
				persona = "tech"
			}
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("maturity-%d", it.ID),
				Title:       mh.Title,
				Description: mh.Description,
				Text:        mh.Instructions,
				Kind:        it.Kind,
				TargetType:  targetType,
				TargetID:    targetID,
				Priority:    it.PriorityHint,
				Persona:     persona,
			})
		}
	}

	// Tracker issues: nudge to decompose if no children, close if all children done.
	// Use composition edges only — sequencing blockers are real waits-for deps and
	// don't count as "children" of the tracker.
	for _, iss := range trackerIssues {
		children, err := h.Q.ListIssueCompositionChildrenWithStatus(ctx, iss.ID)
		if err != nil {
			continue
		}
		if len(children) == 0 {
			// Tracker has no children — needs decomposition
			dth := workflowhints.DecomposeTrackerText(iss.ID)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("decompose-tracker-%s", iss.ID),
				Title:       dth.Title,
				Description: dth.Description,
				Text:        dth.Instructions,
				Kind:        "owner:decompose-tracker",
				TargetType:  "issue",
				TargetID:    iss.ID,
				IssueRef:    iss.ID,
				Priority:    11,
				Persona:     "owner",
			})
			continue
		}
		allClosed := true
		for _, b := range children {
			if b.Status != "closed" {
				allClosed = false
				break
			}
		}
		if !allClosed {
			continue
		}
		cth := workflowhints.CloseTrackerText(iss.ID)
		candidates = append(candidates, soloCandidate{
			Key:         fmt.Sprintf("close-tracker-%s", iss.ID),
			Title:       cth.Title,
			Description: cth.Description,
			Text:        cth.Instructions,
			Kind:        "close:tracker",
			TargetType:  "issue",
			TargetID:    iss.ID,
			IssueRef:    iss.ID,
			Priority:    foldIssuePriority(36, iss.Priority),
			Persona:     "owner",
		})
	}

	// Deferred specs whose blockers are all closed — resurface for re-evaluation.
	// Global only: not meaningful when scoped to a specific issue.
	if issueFilter == "" {
		readySpecs, _ := h.Q.ListSpecsWithAllBlockersClosed(ctx)
		for _, sp := range readySpecs {
			rds := workflowhints.ReviewDeferredSpecText(sp.ID, sp.Description)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("review-deferred-spec-%d", sp.ID),
				Title:       rds.Title,
				Description: rds.Description,
				Text:        rds.Instructions,
				Kind:        "review:deferred-spec",
				TargetType:  "spec",
				TargetID:    fmt.Sprintf("%d", sp.ID),
				Priority:    37,
				Persona:     "owner",
			})
		}
	}

	// Issues with no pending tasks
	for _, iss := range issues {
		tasks, _ := h.Q.ListTasksByIssue(ctx, db.ListTasksByIssueParams{ProjectID: projectID, Issue: iss.ID})
		hasPending := false
		allDone := true
		for _, t := range tasks {
			if t.Status == "ready" || t.Status == "active" {
				hasPending = true
				allDone = false
				break
			}
			if t.Status != "done" {
				allDone = false
			}
		}
		if !hasPending {
			if len(tasks) > 0 && allDone {
				clh := workflowhints.ClosableIssueText(iss.ID, iss.Title)
				candidates = append(candidates, soloCandidate{
					Key:          fmt.Sprintf("closable-%s", iss.ID),
					Title:        clh.Title,
					Description:  clh.Description,
					Text:         clh.Instructions,
					Kind:         "closable",
					TargetType:   "issue",
					TargetID:     iss.ID,
					IssueRef:     iss.ID,
					TargetBranch: iss.TargetBranch,
					Priority:     foldIssuePriority(35, iss.Priority),
					Persona:      "dev",
				})
			} else if len(tasks) == 0 {
				dih := workflowhints.DecomposeIssueText(iss.ID, iss.Title)
				candidates = append(candidates, soloCandidate{
					Key:          fmt.Sprintf("add-%s", iss.ID),
					Title:        dih.Title,
					Description:  dih.Description,
					Text:         dih.Instructions,
					Kind:         "add",
					TargetType:   "issue",
					TargetID:     iss.ID,
					IssueRef:     iss.ID,
					TargetBranch: iss.TargetBranch,
					Priority:     foldIssuePriority(38, iss.Priority),
					Persona:      "dev",
				})
			}
		}
	}

	// Pending tasks
	for _, iss := range issues {
		tasks, _ := h.Q.ListTasksByIssue(ctx, db.ListTasksByIssueParams{ProjectID: projectID, Issue: iss.ID})
		for _, t := range tasks {
			if t.Status == "ready" {
				dth := workflowhints.DevTaskText(t.ID, t.Title, iss.ID)
				// IS-825: a backport task points at a named branch while
				// its source issue still targets dev — task-level branch
				// wins so the queue surfaces it under the right branch.
				targetBranch := t.TargetBranch
				if targetBranch == "" || targetBranch == "dev" {
					targetBranch = iss.TargetBranch
				}
				candidates = append(candidates, soloCandidate{
					Key:          fmt.Sprintf("dev-%s", t.ID),
					Title:        dth.Title,
					Description:  dth.Description,
					Text:         dth.Instructions,
					Kind:         "dev",
					TargetType:   "task",
					TargetID:     t.ID,
					IssueRef:     iss.ID,
					TargetBranch: targetBranch,
					Priority:     foldIssuePriority(40, iss.Priority),
					Persona:      "dev",
				})
			}
		}
	}

	// Orphan tasks — ready but no parent issue, invisible to the issue-based loop above.
	if issueFilter == "" {
		orphans, _ := h.Q.ListOrphanReadyTasks(ctx, projectID)
		for _, t := range orphans {
			oth := workflowhints.OrphanTaskText(t.ID, t.Title)
			candidates = append(candidates, soloCandidate{
				Key:         fmt.Sprintf("orphan-%s", t.ID),
				Title:       oth.Title,
				Description: oth.Description,
				Text:        oth.Instructions,
				Kind:        "owner:orphan-task",
				TargetType:  "task",
				TargetID:    t.ID,
				Priority:    42,
				Persona:     "owner",
			})
		}
	}

	// Sort by priority ascending; within equal priority, dev-targeted items lead
	// and same-branch items stay contiguous (spec 178).
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority < candidates[j].Priority
		}
		bi, bj := candidates[i].TargetBranch, candidates[j].TargetBranch
		if bi == "" {
			bi = "dev"
		}
		if bj == "" {
			bj = "dev"
		}
		if (bi == "dev") != (bj == "dev") {
			return bi == "dev"
		}
		return bi < bj
	})

	return candidates, nil
}

type SoloQueueItem struct {
	Key             string `json:"key"`
	Title           string `json:"title,omitempty"`
	Description     string `json:"description,omitempty"`
	Text            string `json:"text"`
	Kind            string `json:"kind"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	IssueRef        string `json:"issue_ref"`
	TargetBranch    string `json:"target_branch,omitempty"`
	Priority        int32  `json:"priority"`
	Blocked         bool   `json:"blocked"`
	BlockedReason   string `json:"blocked_reason,omitempty"`
	Persona         string `json:"persona"`
	Status          string `json:"status"`
	SuggestedAction string `json:"suggested_action,omitempty"`
}

// suggestedActionForKind returns a ready-to-run CLI command for the given queue item
// so the caller can act without further discovery.
func suggestedActionForKind(kind, targetType, targetID string) string {
	switch kind {
	case "triage":
		return "dx todo owner triage " + targetID + " --priority=<1-4> --type=<ops|impl|ask|tracker>"
	case "add":
		return "dx todo tech add --issue=" + targetID + " --title=<outcome> --text=<plan> --test-plan=<verification>"
	case "dev":
		return "dx todo dev start " + targetID
	case "closable":
		return "dx issue close " + targetID + " --reason=done"
	case "close:tracker":
		return "dx issue close " + targetID + " --reason=done"
	case "owner:decompose-tracker":
		return "dx issue add --title=<subtask> --type=impl"
	case "owner:spec":
		return "dx feature spec add " + targetID
	case "owner:review":
		return "dx feature review " + targetID
	case "tech:test-ref":
		return "dx spec link " + targetID + " <test-id>"
	case "owner:demo-gap":
		return "dx spec link " + targetID + " <test-id>"
	case "clarify":
		return "dx question answer " + targetID + " --answer=\"...\""
	case "answer":
		return "dx qa answer " + targetID + " --answer=\"...\""
	case "respond:discussion":
		return "dx discussion show " + targetID + " && dx discussion reply " + targetID + " --message=\"...\""
	case "read:comments":
		return "dx comment mark-read " + targetType + " " + targetID + " --role=llm"
	case "respond:stale":
		return "dx comment mark-read " + targetType + " " + targetID + " --role=llm"
	case "owner:goals":
		return "dx goal add <title>"
	case "owner:standup":
		return "dx standup checkin --owner"
	case "tech:standup":
		return "dx standup checkin --tech"
	case "owner:orphan-task":
		return "dx issue add --title=<issue> && dx todo tech add --issue=<IS-N>"
	case "review:deferred-spec":
		return "dx spec show " + targetID
	}
	if strings.HasSuffix(kind, ":journal-review") {
		role := strings.TrimSuffix(kind, ":journal-review")
		return "dx standup checkin --" + role
	}
	return ""
}

type EvaluateChange struct {
	Before TodoItem      `json:"before"`
	After  SoloQueueItem `json:"after"`
}

type EvaluateDiff struct {
	Added     []SoloQueueItem  `json:"added"`
	Removed   []TodoItem       `json:"removed"`
	Changed   []EvaluateChange `json:"changed"`
	Unchanged []SoloQueueItem  `json:"unchanged"`
}

// loadExistingBlockedByKey returns maps of key→blocked and key→blocked_reason
// for every currently-blocked todo on a project. Used by claim and refresh
// paths to propagate prior block state onto regenerated synthetic candidates,
// whose c.Blocked is always false (the regenerator has no source-of-truth for
// it). Without this, UpsertTodo's ON CONFLICT would clobber the cycle-detector
// or churn-guard's blocked=true with the regenerator's false on every claim,
// re-opening the very cycles those guards exist to prevent (IS-1041 — the
// deeper layer of IS-1040, which only fixed read:comments by side-channel
// tracking unread state).
func loadExistingBlockedByKey(ctx context.Context, q *db.Queries, projectID int32) (blocked map[string]bool, reason map[string]string) {
	blocked = map[string]bool{}
	reason = map[string]string{}
	rows, err := q.ListTodos(ctx, projectID)
	if err != nil {
		return
	}
	for _, r := range rows {
		if r.Blocked {
			blocked[r.Key] = true
			reason[r.Key] = r.BlockedReason
		}
	}
	return
}

// refreshQueueAsync regenerates and applies the solo queue for a project in the background.
// Call fire-and-forget after any state mutation that affects queue composition.
func (h *Handler) refreshQueueAsync(projectID int32) {
	go func() {
		ctx := context.Background()
		proposed, err := h.generateSoloQueue(ctx, projectID, "", false)
		if err != nil {
			return
		}
		existingBlocked, existingReason := loadExistingBlockedByKey(ctx, h.Q, projectID)
		keys := make([]string, 0, len(proposed))
		for _, c := range proposed {
			keys = append(keys, c.Key)
			blocked := c.Blocked
			blockedReason := c.BlockedReason
			if existingBlocked[c.Key] {
				blocked = true
				if blockedReason == "" {
					blockedReason = existingReason[c.Key]
				}
			}
			_, _ = h.Q.UpsertTodo(ctx, db.UpsertTodoParams{
				ProjectID:     projectID,
				Title:         c.Title,
				Description:   c.Description,
				Text:          c.Text,
				Key:           c.Key,
				Persona:       c.Persona,
				Priority:      c.Priority,
				Status:        "open",
				TargetType:    c.TargetType,
				TargetID:      c.TargetID,
				Kind:          c.Kind,
				IssueRef:      c.IssueRef,
				Blocked:       blocked,
				BlockedReason: blockedReason,
			})
		}
		if len(keys) > 0 {
			_ = h.Q.ResolveTodosNotInKeys(ctx, db.ResolveTodosNotInKeysParams{
				ProjectID: projectID,
				Keys:      keys,
			})
		}
	}()
}

func (h *Handler) registerSoloRoutes(api huma.API) {

	// GET /api/dx/solo — return persisted todo queue
	huma.Register(api, huma.Operation{OperationID: "list-solo-queue", Method: http.MethodGet, Path: "/api/dx/solo"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			Issue   string `query:"issue"`
			Blocked string `query:"blocked"`
			Status  string `query:"status"`
		}) (*struct {
			Body []TodoItem
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			issueRef := pgtype.Text{}
			if in.Issue != "" {
				issueRef = pgtype.Text{String: in.Issue, Valid: true}
			}
			blocked := pgtype.Bool{}
			if in.Blocked == "true" {
				blocked = pgtype.Bool{Bool: true, Valid: true}
			} else if in.Blocked == "false" {
				blocked = pgtype.Bool{Bool: false, Valid: true}
			}
			status := pgtype.Text{String: "open", Valid: true}
			if in.Status == "all" {
				status = pgtype.Text{}
			} else if in.Status != "" {
				status = pgtype.Text{String: in.Status, Valid: true}
			}

			rows, err := h.Q.ListTodosFiltered(ctx, db.ListTodosFilteredParams{
				ProjectID:  p.ID,
				Blocked:    blocked,
				TargetType: pgtype.Text{},
				IssueRef:   issueRef,
				Status:     status,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TodoItem, len(rows))
			for i, r := range rows {
				out[i] = toTodoItemFromFiltered(r)
			}
			return &struct{ Body []TodoItem }{Body: out}, nil
		})

	// POST /api/dx/solo/evaluate — regenerate queue, diff against persisted
	huma.Register(api, huma.Operation{OperationID: "solo-evaluate", Method: http.MethodPost, Path: "/api/dx/solo/evaluate"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Issue string `json:"issue"`
			}
		}) (*struct {
			Body EvaluateDiff
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			proposed, err := h.generateSoloQueue(ctx, p.ID, in.Body.Issue, false)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			current, err := h.Q.ListTodos(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			currentByKey := map[string]db.ListTodosRow{}
			for _, r := range current {
				if r.Status == "open" {
					currentByKey[r.Key] = r
				}
			}

			diff := EvaluateDiff{
				Added:     []SoloQueueItem{},
				Removed:   []TodoItem{},
				Changed:   []EvaluateChange{},
				Unchanged: []SoloQueueItem{},
			}
			proposedKeys := map[string]bool{}
			for _, c := range proposed {
				proposedKeys[c.Key] = true
				item := SoloQueueItem{
					Key: c.Key, Title: c.Title, Description: c.Description, Text: c.Text, Kind: c.Kind,
					TargetType: c.TargetType, TargetID: c.TargetID,
					IssueRef: c.IssueRef, TargetBranch: c.TargetBranch, Priority: c.Priority,
					Blocked: c.Blocked, BlockedReason: c.BlockedReason, Persona: c.Persona, Status: "open",
					SuggestedAction: suggestedActionForKind(c.Kind, c.TargetType, c.TargetID),
				}
				if existing, ok := currentByKey[c.Key]; ok {
					if existing.Priority != c.Priority || existing.Text != c.Text || existing.Blocked != c.Blocked {
						diff.Changed = append(diff.Changed, EvaluateChange{
							Before: toTodoItemFromRow(existing), After: item,
						})
					} else {
						diff.Unchanged = append(diff.Unchanged, item)
					}
				} else {
					diff.Added = append(diff.Added, item)
				}
			}
			for key, r := range currentByKey {
				if !proposedKeys[key] {
					diff.Removed = append(diff.Removed, toTodoItemFromRow(r))
				}
			}

			return &struct{ Body EvaluateDiff }{Body: diff}, nil
		})

	// POST /api/dx/solo/apply — apply an evaluated queue (upsert proposed, resolve stale)
	huma.Register(api, huma.Operation{OperationID: "solo-apply", Method: http.MethodPost, Path: "/api/dx/solo/apply"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string          `json:"slug"`
				Items []SoloQueueItem `json:"items"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			keys := make([]string, 0, len(in.Body.Items))
			for _, item := range in.Body.Items {
				keys = append(keys, item.Key)
				_, err := h.Q.UpsertTodo(ctx, db.UpsertTodoParams{
					ProjectID:     p.ID,
					Title:         item.Title,
					Description:   item.Description,
					Text:          item.Text,
					Key:           item.Key,
					Persona:       item.Persona,
					Priority:      item.Priority,
					Status:        "open",
					TargetType:    item.TargetType,
					TargetID:      item.TargetID,
					Kind:          item.Kind,
					IssueRef:      item.IssueRef,
					Blocked:       item.Blocked,
					BlockedReason: item.BlockedReason,
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
			}

			if len(keys) > 0 {
				if err := h.Q.ResolveTodosNotInKeys(ctx, db.ResolveTodosNotInKeysParams{
					ProjectID: p.ID,
					Keys:      keys,
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			}

			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// POST /api/dx/solo/claim — generate queue, merge, claim next unclaimed todo
	type soloClaimBody struct {
		TodoItem
		Debug *DebugOutput `json:"debug,omitempty"`
	}
	huma.Register(api, huma.Operation{OperationID: "solo-claim", Method: http.MethodPost, Path: "/api/dx/solo/claim"},
		func(ctx context.Context, in *struct {
			Debug       string `query:"debug" required:"false"`
			XAtlasDebug string `header:"X-Atlas-Debug" required:"false"`
			Body        struct {
				Slug         string `json:"slug"`
				AgentID      string `json:"agent_id"`
				LeaseMinutes int32  `json:"lease_minutes" required:"false"`
				Mode         string `json:"mode" required:"false"`
			}
		}) (*struct{ Body soloClaimBody }, error) {
			var err error
			ctx, _, err = debugStart(ctx, in.Debug, in.XAtlasDebug)
			if err != nil {
				return nil, err
			}

			leaseMin := in.Body.LeaseMinutes
			if leaseMin == 0 {
				leaseMin = 10
			}
			autonomous := in.Body.Mode == "autonomous"
			trace.Note(ctx, "lease_minutes", leaseMin)

			claimFromProject := func(p db.ZdxProject) (*TodoItem, error) {
				if expired, _ := h.Q.ReclaimExpiredTodos(ctx, p.ID); len(expired) > 0 {
					for _, t := range expired {
						_ = h.Q.ReleaseReservation(ctx, db.ReleaseReservationParams{
							ProjectID:  t.ProjectID,
							TargetType: "todo",
							TargetID:   fmt.Sprintf("%d", t.ID),
						})
					}
				}
				proposed, err := h.generateSoloQueue(ctx, p.ID, "", autonomous)
				if err != nil {
					return nil, err
				}
				for _, c := range proposed {
					trace.Note(ctx, "candidate", map[string]any{
						"key":      c.Key,
						"kind":     c.Kind,
						"priority": c.Priority,
						"blocked":  c.Blocked,
					})
				}
				existingBlocked, existingReason := loadExistingBlockedByKey(ctx, h.Q, p.ID)
				for _, c := range proposed {
					blocked := c.Blocked
					blockedReason := c.BlockedReason
					if existingBlocked[c.Key] {
						blocked = true
						if blockedReason == "" {
							blockedReason = existingReason[c.Key]
						}
					}
					_, _ = h.Q.UpsertTodo(ctx, db.UpsertTodoParams{
						ProjectID:     p.ID,
						Title:         c.Title,
						Description:   c.Description,
						Text:          c.Text,
						Key:           c.Key,
						Persona:       c.Persona,
						Priority:      c.Priority,
						Status:        "open",
						TargetType:    c.TargetType,
						TargetID:      c.TargetID,
						Kind:          c.Kind,
						IssueRef:      c.IssueRef,
						Blocked:       blocked,
						BlockedReason: blockedReason,
					})
				}
				baseSha, baseBranch := resolveGitHead()
				row, err := h.Q.ClaimNextTodo(ctx, db.ClaimNextTodoParams{
					ProjectID:       p.ID,
					AgentID:         in.Body.AgentID,
					LeaseMinutes:    leaseMin,
					ClaimBaseSha:    baseSha,
					ClaimBaseBranch: baseBranch,
				})
				if err != nil {
					return nil, err
				}
				trace.Note(ctx, "claimed_atomic", map[string]any{
					"id":    row.ID,
					"key":   row.Key,
					"kind":  row.Kind,
					"title": row.Title,
				})
				_, _ = h.Q.InsertReservation(ctx, db.InsertReservationParams{
					ProjectID:      row.ProjectID,
					TargetType:     "todo",
					TargetID:       fmt.Sprintf("%d", row.ID),
					ClaimedBy:      row.ClaimedBy,
					LeaseExpiresAt: row.LeaseExpiresAt,
				})
				item := toTodoItemFromClaim(row)
				item.ProjectSlug = p.Slug
				return &item, nil
			}

			// Global/srcless mode: empty slug means iterate all projects and claim from the first available.
			if in.Body.Slug == "" {
				projects, err := h.Q.ListProjects(ctx)
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				for _, p := range projects {
					item, err := claimFromProject(p)
					if err != nil {
						continue
					}
					return &struct{ Body soloClaimBody }{Body: soloClaimBody{TodoItem: *item, Debug: debugOutput(ctx)}}, nil
				}
				return nil, apiErr(404, "no claimable todo items")
			}

			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			item, err := claimFromProject(p)
			if err != nil {
				h.maybeAutoFileQueueStall(ctx, p.ID, p.Slug)
				return nil, apiErr(404, "no claimable todo items")
			}
			return &struct{ Body soloClaimBody }{Body: soloClaimBody{TodoItem: *item, Debug: debugOutput(ctx)}}, nil
		})

	// POST /api/dx/solo/release — release or resolve a claimed todo
	//
	// When resolve=true the todo is marked resolved. Two guards prevent a
	// resolve from sticking when the underlying work isn't actually done:
	//   - Pre-resolve (IS-514): for triage todos, verify the issue has a
	//     priority. If not, downgrade to release + auto-block.
	//   - Post-resolve cycle check: regenerate the queue and look for the
	//     same key. If it reappears, auto-block.
	// In either case cycle_detected is returned so the agent surfaces the
	// discrepancy instead of re-claiming.
	//
	// The reopen_count churn guard (auto-block at 3+ reopens) remains as a
	// secondary safety net in UpsertTodo.
	huma.Register(api, huma.Operation{OperationID: "solo-release", Method: http.MethodPost, Path: "/api/dx/solo/release"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID          int32        `json:"id"`
				AgentID     string       `json:"agent_id"`
				Resolve     bool         `json:"resolve" required:"false"`
				SessionID   string       `json:"session_id" required:"false"`
				BranchState *BranchState `json:"branch_state,omitempty" required:"false"`
				Force       *bool        `json:"force,omitempty" required:"false"`
			}
		}) (*struct {
			Body struct {
				OK              bool `json:"ok"`
				ChurnDowngraded bool `json:"churn_downgraded" required:"false"`
				CycleDetected   bool `json:"cycle_detected" required:"false"`
			}
		}, error) {
			resolve := in.Body.Resolve
			force := in.Body.Force != nil && *in.Body.Force
			todo, _ := h.Q.GetTodoByID(ctx, in.Body.ID)

			// IS-915: claim contract validation on resolve.
			// IS-916/TK-1611: --force bypasses the contract check; emits audit event.
			if resolve && in.Body.BranchState != nil && todo.ID != 0 {
				if err := validateClaimContract(todo.Kind, todo.ClaimBaseSha, todo.ClaimBaseBranch, in.Body.BranchState, force); err != nil {
					return nil, err
				}
				if force {
					h.emitClaimContractBypass(ctx, todo.ProjectID, todo.ID, in.Body.AgentID)
				}
			}

			// IS-514: pre-resolve guard for triage todos. Resolve only succeeds
			// if the underlying issue has actually been triaged (priority set).
			// Otherwise the agent's "session succeeded" path silently flips the
			// todo to resolved even though no triage level was applied.
			triageIncomplete := false
			if resolve && todo.ID != 0 && todo.Kind == "triage" && todo.TargetType == "issue" && todo.TargetID != "" {
				iss, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: todo.ProjectID, ID: todo.TargetID})
				if err == nil && iss.Priority == "" && iss.Status == "open" {
					triageIncomplete = true
					resolve = false
				}
			}

			if resolve {
				_ = h.Q.ResolveTodoByID(ctx, in.Body.ID)
			} else if in.Body.AgentID == "" {
				_ = h.Q.ReleaseTodoAdmin(ctx, in.Body.ID)
			} else {
				_ = h.Q.ReleaseTodo(ctx, db.ReleaseTodoParams{
					ID:        in.Body.ID,
					ClaimedBy: in.Body.AgentID,
				})
			}
			if todo.ID != 0 {
				_ = h.Q.ReleaseReservation(ctx, db.ReleaseReservationParams{
					ProjectID:  todo.ProjectID,
					TargetType: "todo",
					TargetID:   fmt.Sprintf("%d", todo.ID),
				})
			}

			// IS-1040: when a read:comments todo concludes (resolve OR release —
			// the agent has examined the comments either way), advance the
			// per-target seen watermark. Without this the regenerator emits the
			// same synthetic candidate next iteration and the loop spins. We
			// run it before the post-resolve cycle check below so generateSoloQueue
			// reflects the new watermark and won't false-positive a cycle.
			if todo.ID != 0 && todo.Kind == "read:comments" && todo.TargetType != "" && todo.TargetID != "" {
				_ = h.Q.MarkTargetCommentsSeen(ctx, db.MarkTargetCommentsSeenParams{
					ProjectID:  todo.ProjectID,
					TargetType: todo.TargetType,
					TargetID:   todo.TargetID,
				})
			}

			cycleDetected := false
			switch {
			case triageIncomplete:
				// Agent reported the triage todo done but the issue still has
				// no priority. Auto-block and surface as cycle_detected so the
				// agent logs the discrepancy rather than re-claiming.
				cycleDetected = true
				blocked, bErr := h.Q.BlockTodoByKey(ctx, db.BlockTodoByKeyParams{
					ProjectID: todo.ProjectID,
					Key:       todo.Key,
					Reason:    "Triage incomplete: set a priority on " + todo.TargetID + " before resolving",
				})
				if bErr == nil {
					h.maybeAutoFileBlockIssue(ctx, blocked)
				}
			case resolve && todo.Key != "" && todo.ProjectID != 0:
				// Post-resolve cycle check: if we just resolved a todo, regenerate
				// the queue and see if the same key would come back. If so, the
				// agent cannot fix this — auto-block to prevent infinite loops.
				candidates, err := h.generateSoloQueue(ctx, todo.ProjectID, "", true)
				if err == nil {
					for _, c := range candidates {
						if c.Key == todo.Key {
							cycleDetected = true
							blocked, bErr := h.Q.BlockTodoByKey(ctx, db.BlockTodoByKeyParams{
								ProjectID: todo.ProjectID,
								Key:       todo.Key,
								Reason:    "Cycle detected: todo reappears in queue after resolve — manual intervention required",
							})
							if bErr == nil {
								h.maybeAutoFileBlockIssue(ctx, blocked)
							}
							break
						}
					}
				}
			}

			return &struct {
				Body struct {
					OK              bool `json:"ok"`
					ChurnDowngraded bool `json:"churn_downgraded" required:"false"`
					CycleDetected   bool `json:"cycle_detected" required:"false"`
				}
			}{Body: struct {
				OK              bool `json:"ok"`
				ChurnDowngraded bool `json:"churn_downgraded" required:"false"`
				CycleDetected   bool `json:"cycle_detected" required:"false"`
			}{OK: true, CycleDetected: cycleDetected}}, nil
		})

	// GET /api/dx/solo/claims — list all active todo + task claims (unexpired leases)
	huma.Register(api, huma.Operation{OperationID: "solo-list-claims", Method: http.MethodGet, Path: "/api/dx/solo/claims"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Todos []TodoItem      `json:"todos"`
				Tasks []AgentTaskItem `json:"tasks"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			todoRows, err := h.Q.ListActiveTodoClaims(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			taskRows, err := h.Q.ListActiveTaskClaims(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			todos := make([]TodoItem, len(todoRows))
			for i, r := range todoRows {
				todos[i] = TodoItem{
					ID:               r.ID,
					Text:             r.Text,
					Title:            r.Title,
					Description:      r.Description,
					Key:              r.Key,
					Persona:          r.Persona,
					Priority:         r.Priority,
					Status:           r.Status,
					TargetType:       r.TargetType,
					TargetID:         r.TargetID,
					Kind:             r.Kind,
					IssueRef:         r.IssueRef,
					Blocked:          r.Blocked,
					BlockedReason:    r.BlockedReason,
					CycleCount:       r.CycleCount,
					ReferenceIssueID: r.ReferenceIssueID,
					SuggestedAction:  suggestedActionForKind(r.Kind, r.TargetType, r.TargetID),
					ClaimBaseSha:     r.ClaimBaseSha,
					ClaimBaseBranch:  r.ClaimBaseBranch,
					ClaimedBy:        r.ClaimedBy,
					ClaimedAt:        fmtTS(r.ClaimedAt),
					CreatedAt:        fmtTS(r.CreatedAt),
					ResolvedAt:       fmtTS(r.ResolvedAt),
				}
			}
			tasks := make([]AgentTaskItem, len(taskRows))
			for i, r := range taskRows {
				tasks[i] = AgentTaskItem{
					ID:             r.ID,
					Title:          r.Title,
					Text:           r.Text,
					Feature:        r.Feature,
					Status:         r.Status,
					Issue:          r.Issue,
					TaskGroup:      r.TaskGroup,
					CreatedAt:      fmtTS(r.CreatedAt),
					ClaimedAt:      fmtTS(r.ClaimedAt),
					LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
				}
			}
			return &struct {
				Body struct {
					Todos []TodoItem      `json:"todos"`
					Tasks []AgentTaskItem `json:"tasks"`
				}
			}{Body: struct {
				Todos []TodoItem      `json:"todos"`
				Tasks []AgentTaskItem `json:"tasks"`
			}{Todos: todos, Tasks: tasks}}, nil
		})

	// POST /api/dx/solo/unblock-all — clear blocked flag on all open blocked todos
	huma.Register(api, huma.Operation{OperationID: "solo-unblock-all", Method: http.MethodPost, Path: "/api/dx/solo/unblock-all"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug" required:"true"`
			}
		}) (*struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UnblockAllTodos(ctx, p.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.refreshQueueAsync(p.ID)
			return &struct {
				Body struct {
					OK bool `json:"ok"`
				}
			}{Body: struct {
				OK bool `json:"ok"`
			}{OK: true}}, nil
		})

	// GET /api/dx/solo/reservations — list historical + active reservations for a project
	// Optional issue_id param filters to reservations whose todo targets that issue.
	huma.Register(api, huma.Operation{OperationID: "solo-list-reservations", Method: http.MethodGet, Path: "/api/dx/solo/reservations"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			IssueID string `query:"issue_id" required:"false"`
			Limit   int32  `query:"limit" required:"false"`
		}) (*struct {
			Body struct {
				Reservations []ReservationItem `json:"reservations"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			var items []ReservationItem
			if in.IssueID != "" {
				rows, err := h.Q.ListReservationsByIssue(ctx, db.ListReservationsByIssueParams{
					ProjectID: p.ID,
					IssueID:   in.IssueID,
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				items = make([]ReservationItem, len(rows))
				for i, r := range rows {
					item := ReservationItem{
						ID:             r.ID,
						TargetType:     r.TargetType,
						TargetID:       r.TargetID,
						ClaimedBy:      r.ClaimedBy,
						ClaimedAt:      fmtTS(r.ClaimedAt),
						ReleasedAt:     fmtTS(r.ReleasedAt),
						LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
						TodoText:       r.TodoText,
					}
					if r.SessionID.Valid {
						item.SessionID = r.SessionID.Int64
						item.SessionStatus = r.SessionStatus.String
						item.SessionClosedAt = fmtTS(r.SessionClosedAt)
						item.SessionHeader = r.SessionHeader.String
						item.SessionAlias = r.SessionAlias.String
					}
					items[i] = item
				}
			} else {
				lim := in.Limit
				if lim <= 0 {
					lim = 100
				}
				rows, err := h.Q.ListReservations(ctx, db.ListReservationsParams{
					ProjectID: p.ID,
					Lim:       lim,
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				items = make([]ReservationItem, len(rows))
				for i, r := range rows {
					items[i] = ReservationItem{
						ID:             r.ID,
						TargetType:     r.TargetType,
						TargetID:       r.TargetID,
						ClaimedBy:      r.ClaimedBy,
						ClaimedAt:      fmtTS(r.ClaimedAt),
						ReleasedAt:     fmtTS(r.ReleasedAt),
						LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
					}
				}
			}
			return &struct {
				Body struct {
					Reservations []ReservationItem `json:"reservations"`
				}
			}{Body: struct {
				Reservations []ReservationItem `json:"reservations"`
			}{Reservations: items}}, nil
		})

	// POST /api/dx/solo/renew — extend lease on a claimed todo
	huma.Register(api, huma.Operation{OperationID: "solo-renew", Method: http.MethodPost, Path: "/api/dx/solo/renew"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID           int32  `json:"id"`
				AgentID      string `json:"agent_id"`
				LeaseMinutes int32  `json:"lease_minutes" required:"false"`
			}
		}) (*struct{ Body OKBody }, error) {
			leaseMin := in.Body.LeaseMinutes
			if leaseMin == 0 {
				leaseMin = 10
			}
			_ = h.Q.RenewTodoLease(ctx, db.RenewTodoLeaseParams{
				ID:           in.Body.ID,
				AgentID:      in.Body.AgentID,
				LeaseMinutes: leaseMin,
			})
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}

func toTodoItemFromClaim(r db.ClaimNextTodoRow) TodoItem {
	return TodoItem{
		ID:               r.ID,
		Text:             r.Text,
		Title:            r.Title,
		Description:      r.Description,
		Key:              r.Key,
		Persona:          r.Persona,
		Priority:         r.Priority,
		Status:           r.Status,
		TargetType:       r.TargetType,
		TargetID:         r.TargetID,
		Kind:             r.Kind,
		IssueRef:         r.IssueRef,
		TargetBranch:     r.TargetBranch,
		Blocked:          r.Blocked,
		BlockedReason:    r.BlockedReason,
		CycleCount:       r.CycleCount,
		ReferenceIssueID: r.ReferenceIssueID,
		SuggestedAction:  suggestedActionForKind(r.Kind, r.TargetType, r.TargetID),
		ClaimBaseSha:     r.ClaimBaseSha,
		ClaimedBy:        r.ClaimedBy,
		ClaimedAt:        fmtTS(r.ClaimedAt),
		CreatedAt:        fmtTS(r.CreatedAt),
		ResolvedAt:       fmtTS(r.ResolvedAt),
	}
}

func toTodoItemFromFiltered(r db.ListTodosFilteredRow) TodoItem {
	return TodoItem{
		ID:               r.ID,
		Text:             r.Text,
		Title:            r.Title,
		Description:      r.Description,
		Key:              r.Key,
		Persona:          r.Persona,
		Priority:         r.Priority,
		Status:           r.Status,
		TargetType:       r.TargetType,
		TargetID:         r.TargetID,
		Kind:             r.Kind,
		IssueRef:         r.IssueRef,
		Blocked:          r.Blocked,
		BlockedReason:    r.BlockedReason,
		CycleCount:       r.CycleCount,
		ReferenceIssueID: r.ReferenceIssueID,
		SuggestedAction:  suggestedActionForKind(r.Kind, r.TargetType, r.TargetID),
		ClaimedBy:        r.ClaimedBy,
		ClaimedAt:        fmtTS(r.ClaimedAt),
		CreatedAt:        fmtTS(r.CreatedAt),
		ResolvedAt:       fmtTS(r.ResolvedAt),
	}
}

// resolveGitHead returns the HEAD SHA and branch of the current working directory.
// Returns empty strings if git is unavailable or CWD is not a git repo (non-fatal).
func resolveGitHead() (sha, branch string) {
	if out, err := exec.Command("git", "rev-parse", "HEAD").Output(); err == nil {
		sha = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		branch = strings.TrimSpace(string(out))
	}
	return
}

// maybeAutoFileBlockIssue creates a system-gap issue when a todo hits its second
// cycle detection. The issue ID is stored on the todo so the queue UI can link to it.
// Threshold: cycle_count >= 2 and no reference issue already filed.
func (h *Handler) maybeAutoFileBlockIssue(ctx context.Context, blocked db.BlockTodoByKeyRow) {
	// Auto-file is gated off — when a todo source is broken (e.g. read:comments before
	// IS-677), this fans out to one System gap issue per affected target, which then
	// become new targets and recurse. Re-enable with AUTO_FILE_AGENT_FAILURES=true once
	// the upstream sources are reliable.
	if os.Getenv("AUTO_FILE_AGENT_FAILURES") != "true" {
		return
	}
	if blocked.CycleCount < 2 || blocked.ReferenceIssueID != "" {
		return
	}
	p, err := h.Q.GetProjectByID(ctx, blocked.ProjectID)
	if err != nil {
		return
	}
	issueID, err := h.Q.NextIssueID(ctx)
	if err != nil {
		return
	}
	area := blocked.TargetType
	if blocked.TargetID != "" {
		area = blocked.TargetType + " " + blocked.TargetID
	}
	title := fmt.Sprintf("System gap: todo %q cannot be satisfied by agents", blocked.Key)
	context := fmt.Sprintf(
		"Todo %q (key: %s) has been cycle-detected %d times — agents resolve it but the queue regenerates it unchanged. "+
			"This may indicate a system gap in [%s]: the satisfaction criteria are broken, a check is incorrect, or the todo describes work that requires a capability zdx does not yet have. "+
			"Investigate the area and fix the root cause. When this issue is closed the todo will automatically unblock and re-enter the queue.",
		blocked.Title, blocked.Key, blocked.CycleCount, area,
	)
	// Priority must be non-empty: open + empty-priority issues are themselves triage
	// candidates, so an auto-filed gap with no priority would spawn its own triage cycle
	// and recursively auto-file another gap (IS-546 cascade).
	issue, err := h.Q.CreateIssue(ctx, db.CreateIssueParams{
		ID:        issueID,
		ProjectID: p.ID,
		Title:     title,
		Context:   context,
		IssueType: "impl",
		Status:    "open",
		Priority:  "2",
	})
	if err != nil {
		return
	}
	_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{
		IssueID: issue.ID,
		Agent:   "system",
		Note: fmt.Sprintf("[auto-filed] Cycle detection fired %d times on todo %q. Blocked todo will auto-unblock when this issue is closed.",
			blocked.CycleCount, blocked.Key),
	})
	_ = h.Q.SetTodoReferenceIssue(ctx, db.SetTodoReferenceIssueParams{
		ProjectID:        p.ID,
		Key:              blocked.Key,
		ReferenceIssueID: issue.ID,
	})
}

const stallIssuePrefix = "System gap: todo queue fully stalled"

// maybeAutoFileQueueStall fires when the claim handler finds no claimable todos.
// If total_open > 0 and unblocked_count == 0, the queue is fully stalled: every
// open todo is blocked. Auto-files a system-gap issue so agents can self-heal.
// On recovery (unblocked_count > 0), closes any existing stall issue and unblocks
// todos tied to it.
// Gated on AUTO_FILE_AGENT_FAILURES=true (same as maybeAutoFileBlockIssue, IS-677).
func (h *Handler) maybeAutoFileQueueStall(ctx context.Context, projectID int32, slug string) {
	if os.Getenv("AUTO_FILE_AGENT_FAILURES") != "true" {
		return
	}
	health, err := h.Q.GetTodoQueueHealth(ctx, projectID)
	if err != nil {
		return
	}

	// Find any existing open stall issue for dedup and recovery.
	var existingStallID string
	issues, err := h.Q.ListIssues(ctx, projectID)
	if err == nil {
		for _, iss := range issues {
			if iss.Status == "open" && len(iss.Title) >= len(stallIssuePrefix) && iss.Title[:len(stallIssuePrefix)] == stallIssuePrefix {
				existingStallID = iss.ID
				break
			}
		}
	}

	if health.UnblockedCount > 0 {
		// Queue recovered — close the stall issue if one is open.
		if existingStallID != "" {
			_ = h.Q.CloseIssue(ctx, db.CloseIssueParams{
				ProjectID:   projectID,
				ID:          existingStallID,
				CloseReason: "Queue stall resolved: unblocked todos are now claimable",
			})
			_ = h.Q.UnblockTodosByReferenceIssue(ctx, db.UnblockTodosByReferenceIssueParams{
				ProjectID:        projectID,
				ReferenceIssueID: existingStallID,
			})
		}
		return
	}

	// Queue fully stalled: open todos exist but none are claimable.
	if health.TotalOpen == 0 || existingStallID != "" {
		return
	}

	dominantReason := health.DominantBlockedReason
	issueID, err := h.Q.NextIssueID(ctx)
	if err != nil {
		return
	}
	title := fmt.Sprintf("%s (%d/%d blocked: %s)", stallIssuePrefix, health.BlockedCount, health.TotalOpen, dominantReason)
	description := fmt.Sprintf(
		"Every open todo in project %q is blocked (%d/%d). No agent work can proceed until at least one todo is unblocked. "+
			"Dominant blocked reason: %q. "+
			"Investigate and resolve the blocked todos. When this issue is closed the affected todos will automatically unblock. "+
			"(See IS-956 for the queue-stall self-healing design.)",
		slug, health.BlockedCount, health.TotalOpen, dominantReason,
	)
	issue, err := h.Q.CreateIssue(ctx, db.CreateIssueParams{
		ID:        issueID,
		ProjectID: projectID,
		Title:     title,
		Context:   description,
		IssueType: "impl",
		Status:    "open",
		Priority:  "1",
	})
	if err != nil {
		return
	}
	_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{
		IssueID: issue.ID,
		Agent:   "system",
		Note: fmt.Sprintf("[auto-filed] Queue fully stalled: %d/%d todos blocked. Dominant reason: %q. Unblock todos or close this issue to resume agent work.",
			health.BlockedCount, health.TotalOpen, dominantReason),
	})
}
