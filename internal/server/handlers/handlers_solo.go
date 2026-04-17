package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type soloCandidate struct {
	Key        string
	Text       string
	Kind       string
	TargetType string
	TargetID   string
	IssueRef   string
	Priority   int32
	Blocked    bool
	Persona    string
}

func (h *Handler) generateSoloQueue(ctx context.Context, projectID int32, issueFilter string) ([]soloCandidate, error) {
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

	// Exclude tracker issues — they are closed by their children, never actionable directly
	{
		var filtered []db.ZdxIssue
		for _, iss := range issues {
			if iss.IssueType != "tracker" {
				filtered = append(filtered, iss)
			}
		}
		issues = filtered
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
							Key:        fmt.Sprintf("bq-%d", q.ID),
							Text:       q.Context,
							Kind:       "clarify",
							TargetType: "blocker_question",
							TargetID:   fmt.Sprintf("BQ-%d", q.ID),
							IssueRef:   iss.ID,
							Priority:   5,
							Blocked:    true,
							Persona:    "owner",
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

	// Check for unread LLM comments on issues (any status, not just open)
	unreadIssues, _ := h.Q.ListIssuesWithUnreadComments(ctx, db.ListIssuesWithUnreadCommentsParams{
		ProjectID: projectID, Role: "llm",
	})
	for _, ui := range unreadIssues {
		if issueFilter != "" && ui.ID != issueFilter {
			continue
		}
		candidates = append(candidates, soloCandidate{
			Key:        fmt.Sprintf("comment-issue-%s", ui.ID),
			Text:       fmt.Sprintf("Unread comments on %s: %s", ui.ID, ui.Title),
			Kind:       "read:comments",
			TargetType: "issue",
			TargetID:   ui.ID,
			IssueRef:   ui.ID,
			Priority:   5,
			Persona:    "dev",
		})
	}

	// Check for unread LLM comments on features
	features, _ := h.Q.ListFeatures(ctx, projectID)
	if issueFilter == "" {
		for _, f := range features {
			hasUnread, _ := h.Q.HasUnreadCommentsForTarget(ctx, db.HasUnreadCommentsForTargetParams{
				ProjectID: projectID, TargetType: "feature", TargetID: f.Name, Role: "llm",
			})
			if hasUnread {
				candidates = append(candidates, soloCandidate{
					Key:        fmt.Sprintf("comment-feature-%s", f.Name),
					Text:       fmt.Sprintf("Unread comments on feature %q", f.Name),
					Kind:       "read:comments",
					TargetType: "feature",
					TargetID:   f.Name,
					Priority:   8,
					Persona:    "dev",
				})
			}
		}
	}

	// Unanswered QA questions
	if issueFilter == "" {
		unanswered, _ := h.Q.ListUnansweredQuestions(ctx, projectID)
		for _, q := range unanswered {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("qa-%d", q.ID),
				Text:       q.Question,
				Kind:       "answer",
				TargetType: "question",
				TargetID:   fmt.Sprintf("QA-%d", q.ID),
				Priority:   10,
				Persona:    "dev",
			})
		}
	}

	// Stale unread comments
	if issueFilter == "" {
		stale, _ := h.Q.ListStaleUnreadComments(ctx, db.ListStaleUnreadCommentsParams{
			ProjectID: projectID, Role: "llm", AgeHours: 24,
		})
		for _, c := range stale {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("stale-comment-%d", c.ID),
				Text:       fmt.Sprintf("Stale unread comment on %s %s", c.TargetType, c.TargetID),
				Kind:       "respond:stale",
				TargetType: c.TargetType,
				TargetID:   c.TargetID,
				Priority:   12,
				Persona:    "dev",
			})
		}
	}

	// Project health checks (global only)
	if issueFilter == "" {
		goalCount, _ := h.Q.CountProjectGoals(ctx, projectID)
		if goalCount == 0 {
			candidates = append(candidates, soloCandidate{
				Key: "health-goals", Text: "Project has no goals defined",
				Kind: "owner:goals", TargetType: "project", Priority: 15, Persona: "owner",
			})
		}
		constraintCount, _ := h.Q.CountProjectConstraints(ctx, projectID)
		if constraintCount == 0 {
			candidates = append(candidates, soloCandidate{
				Key: "health-constraints", Text: "Project has no constraints defined",
				Kind: "owner:constraints", TargetType: "project", Priority: 15, Persona: "owner",
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
					candidates = append(candidates, soloCandidate{
						Key: "health-owner-standup", Text: "Owner standup check-in overdue",
						Kind: "owner:standup", TargetType: "project", Priority: 18, Persona: "owner",
					})
				}
			}
			if techDate != "" {
				if t, err := time.Parse("2006-01-02", techDate); err == nil && now.Sub(t) > 7*24*time.Hour {
					candidates = append(candidates, soloCandidate{
						Key: "health-tech-standup", Text: "Tech standup check-in overdue",
						Kind: "tech:standup", TargetType: "project", Priority: 18, Persona: "tech",
					})
				}
			}
		}

		for _, r := range []string{"owner", "tech"} {
			if _, err := h.Q.GetUnreviewedJournalEntry(ctx, db.GetUnreviewedJournalEntryParams{ProjectID: projectID, Role: r}); err == nil {
				candidates = append(candidates, soloCandidate{
					Key: fmt.Sprintf("journal-review-%s", r), Text: fmt.Sprintf("Review generated %s check-in", r),
					Kind: r + ":journal-review", TargetType: "project", Priority: 20, Persona: r,
				})
			}
		}
	}

	// Untriaged issues (no priority)
	for _, iss := range issues {
		if iss.Priority == "" {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("triage-%s", iss.ID),
				Text:       fmt.Sprintf("Triage: %s", iss.Title),
				Kind:       "triage",
				TargetType: "issue",
				TargetID:   iss.ID,
				IssueRef:   iss.ID,
				Priority:   20,
				Persona:    "owner",
			})
		}
	}

	// Cross-cutting checks (global only)
	if issueFilter == "" {
		for _, f := range features {
			specs, _ := h.Q.ListSpecs(ctx, f.ID)
			if len(specs) == 0 {
				candidates = append(candidates, soloCandidate{
					Key:        fmt.Sprintf("spec-missing-%s", f.Name),
					Text:       fmt.Sprintf("Feature %q has no specs", f.Name),
					Kind:       "owner:spec",
					TargetType: "feature",
					TargetID:   f.Name,
					Priority:   25,
					Persona:    "owner",
				})
			}
		}

		staleFeatures, _ := h.Q.ListStaleFeatures(ctx, db.ListStaleFeaturesParams{ProjectID: projectID, StaleDays: 30})
		for _, f := range staleFeatures {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("review-feature-%s", f.Name),
				Text:       fmt.Sprintf("Feature %q not reviewed in >30 days", f.Name),
				Kind:       "owner:review",
				TargetType: "feature",
				TargetID:   f.Name,
				Priority:   28,
				Persona:    "owner",
			})
		}

		uncoveredSpecs, _ := h.Q.ListUncoveredSpecs(ctx, projectID)
		for _, sp := range uncoveredSpecs {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("test-ref-%d", sp.ID),
				Text:       fmt.Sprintf("Spec %d (%s) on %q has no test refs", sp.ID, sp.Description, sp.FeatureName),
				Kind:       "tech:test-ref",
				TargetType: "spec",
				TargetID:   fmt.Sprintf("%d", sp.ID),
				Priority:   30,
				Persona:    "tech",
			})
		}

		demoGaps, _ := h.Q.ListSpecsWithoutDemos(ctx, projectID)
		for _, sp := range demoGaps {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("demo-gap-%d", sp.ID),
				Text:       fmt.Sprintf("Spec %d (%s) on %q has no demo", sp.ID, sp.Description, sp.FeatureName),
				Kind:       "owner:demo-gap",
				TargetType: "spec",
				TargetID:   fmt.Sprintf("%d", sp.ID),
				Priority:   32,
				Persona:    "owner",
			})
		}

		// ── Maturity nudges ──────────────────────────────────────────────

		// Goals without metrics (active > 14 days)
		unquantifiedGoals, _ := h.Q.ListGoalsNeedingMetrics(ctx, db.ListGoalsNeedingMetricsParams{
			ProjectID: projectID, AgeDays: 14,
		})
		for _, g := range unquantifiedGoals {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("quantify-goal-%d", g.ID),
				Text:       fmt.Sprintf("Quantify goal %d: %s — add metric_name and metric_unit", g.ID, g.Title),
				Kind:       "owner:quantify-goal",
				TargetType: "goal",
				TargetID:   fmt.Sprintf("%d", g.ID),
				Priority:   33,
				Persona:    "owner",
			})
		}

		// Features with no goal and no parent (unattributed)
		unattributed, _ := h.Q.ListUnattributedFeatures(ctx, projectID)
		for _, f := range unattributed {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("attribute-feature-%d", f.ID),
				Text:       fmt.Sprintf("Attribute feature %q to a goal or parent feature", f.Name),
				Kind:       "owner:attribute-feature",
				TargetType: "feature",
				TargetID:   f.Name,
				Priority:   34,
				Persona:    "owner",
			})
		}

		// Multiplier features missing instrumentation
		uninstrumented, _ := h.Q.ListFeaturesNeedingInstrumentation(ctx, projectID)
		for _, f := range uninstrumented {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("instrument-feature-%d", f.ID),
				Text:       fmt.Sprintf("Instrument multiplier feature %q — needs baseline, target, and graph", f.Name),
				Kind:       "tech:instrument-feature",
				TargetType: "feature",
				TargetID:   f.Name,
				Priority:   34,
				Persona:    "tech",
			})
		}

		// Over-specced features (decomposition signal)
		overspecced, _ := h.Q.ListOverspeccedFeatures(ctx, db.ListOverspeccedFeaturesParams{
			ProjectID: projectID, Threshold: 8,
		})
		for _, f := range overspecced {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("decompose-feature-%d", f.ID),
				Text:       fmt.Sprintf("Decompose feature %q — %d specs exceeds threshold (8)", f.Name, f.SpecCount),
				Kind:       "owner:decompose-feature",
				TargetType: "feature",
				TargetID:   f.Name,
				Priority:   35,
				Persona:    "owner",
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
				candidates = append(candidates, soloCandidate{
					Key:        fmt.Sprintf("closable-%s", iss.ID),
					Text:       fmt.Sprintf("All tasks done: %s", iss.Title),
					Kind:       "closable",
					TargetType: "issue",
					TargetID:   iss.ID,
					IssueRef:   iss.ID,
					Priority:   35,
					Persona:    "dev",
				})
			} else if len(tasks) == 0 {
				candidates = append(candidates, soloCandidate{
					Key:        fmt.Sprintf("add-%s", iss.ID),
					Text:       fmt.Sprintf("Decompose: %s", iss.Title),
					Kind:       "add",
					TargetType: "issue",
					TargetID:   iss.ID,
					IssueRef:   iss.ID,
					Priority:   38,
					Persona:    "dev",
				})
			}
		}
	}

	// Pending tasks
	for _, iss := range issues {
		tasks, _ := h.Q.ListTasksByIssue(ctx, db.ListTasksByIssueParams{ProjectID: projectID, Issue: iss.ID})
		for _, t := range tasks {
			if t.Status == "ready" {
				candidates = append(candidates, soloCandidate{
					Key:        fmt.Sprintf("dev-%s", t.ID),
					Text:       t.Text,
					Kind:       "dev",
					TargetType: "task",
					TargetID:   t.ID,
					IssueRef:   iss.ID,
					Priority:   40,
					Persona:    "dev",
				})
			}
		}
	}

	// Orphan tasks — ready but no parent issue, invisible to the issue-based loop above.
	if issueFilter == "" {
		orphans, _ := h.Q.ListOrphanReadyTasks(ctx, projectID)
		for _, t := range orphans {
			candidates = append(candidates, soloCandidate{
				Key:        fmt.Sprintf("orphan-%s", t.ID),
				Text:       fmt.Sprintf("Orphan task (no issue): %s", t.Text),
				Kind:       "owner:orphan-task",
				TargetType: "task",
				TargetID:   t.ID,
				Priority:   42,
				Persona:    "owner",
			})
		}
	}

	return candidates, nil
}

type SoloQueueItem struct {
	Key        string `json:"key"`
	Text       string `json:"text"`
	Kind       string `json:"kind"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	IssueRef   string `json:"issue_ref"`
	Priority   int32  `json:"priority"`
	Blocked    bool   `json:"blocked"`
	Persona    string `json:"persona"`
	Status     string `json:"status"`
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

			proposed, err := h.generateSoloQueue(ctx, p.ID, in.Body.Issue)
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

			var diff EvaluateDiff
			proposedKeys := map[string]bool{}
			for _, c := range proposed {
				proposedKeys[c.Key] = true
				item := SoloQueueItem{
					Key: c.Key, Text: c.Text, Kind: c.Kind,
					TargetType: c.TargetType, TargetID: c.TargetID,
					IssueRef: c.IssueRef, Priority: c.Priority,
					Blocked: c.Blocked, Persona: c.Persona, Status: "open",
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
					ProjectID:  p.ID,
					Text:       item.Text,
					Key:        item.Key,
					Persona:    item.Persona,
					Priority:   item.Priority,
					Status:     "open",
					TargetType: item.TargetType,
					TargetID:   item.TargetID,
					Kind:       item.Kind,
					IssueRef:   item.IssueRef,
					Blocked:    item.Blocked,
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
	huma.Register(api, huma.Operation{OperationID: "solo-claim", Method: http.MethodPost, Path: "/api/dx/solo/claim"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug         string `json:"slug"`
				AgentID      string `json:"agent_id"`
				LeaseMinutes int32  `json:"lease_minutes" required:"false"`
			}
		}) (*struct{ Body TodoItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			// Reclaim expired leases first.
			_ = h.Q.ReclaimExpiredTodos(ctx, p.ID)

			// Generate fresh queue and merge into persisted todos (preserving claims).
			proposed, err := h.generateSoloQueue(ctx, p.ID, "")
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			keys := make([]string, 0, len(proposed))
			for _, c := range proposed {
				keys = append(keys, c.Key)
				_, _ = h.Q.UpsertTodo(ctx, db.UpsertTodoParams{
					ProjectID:  p.ID,
					Text:       c.Text,
					Key:        c.Key,
					Persona:    c.Persona,
					Priority:   c.Priority,
					Status:     "open",
					TargetType: c.TargetType,
					TargetID:   c.TargetID,
					Kind:       c.Kind,
					IssueRef:   c.IssueRef,
					Blocked:    c.Blocked,
				})
			}
			if len(keys) > 0 {
				_ = h.Q.ResolveTodosNotInKeys(ctx, db.ResolveTodosNotInKeysParams{
					ProjectID: p.ID, Keys: keys,
				})
			}

			// Claim the next available item.
			leaseMin := in.Body.LeaseMinutes
			if leaseMin == 0 {
				leaseMin = 10
			}
			row, err := h.Q.ClaimNextTodo(ctx, db.ClaimNextTodoParams{
				ProjectID:    p.ID,
				AgentID:      in.Body.AgentID,
				LeaseMinutes: leaseMin,
			})
			if err != nil {
				return nil, apiErr(404, "no claimable todo items")
			}
			item := toTodoItemFromClaim(row)
			return &struct{ Body TodoItem }{Body: item}, nil
		})

	// POST /api/dx/solo/release — release a claimed todo
	// When resolve=true and a session_id is provided, the handler verifies the
	// session actually recorded at least one revision. Sessions that exit cleanly
	// without applying any durable mutation release without marking resolved —
	// this prevents UpsertTodo from reopening the same key on the next claim and
	// burning tokens in a churn loop.
	huma.Register(api, huma.Operation{OperationID: "solo-release", Method: http.MethodPost, Path: "/api/dx/solo/release"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID        int32  `json:"id"`
				AgentID   string `json:"agent_id"`
				Resolve   bool   `json:"resolve" required:"false"`
				SessionID string `json:"session_id" required:"false"`
			}
		}) (*struct {
			Body struct {
				OK              bool `json:"ok"`
				ChurnDowngraded bool `json:"churn_downgraded" required:"false"`
			}
		}, error) {
			resolve := in.Body.Resolve
			churnDowngraded := false
			if resolve && in.Body.SessionID != "" {
				todo, err := h.Q.GetTodoByID(ctx, in.Body.ID)
				if err == nil {
					n, _ := h.Q.CountRevisionsBySession(ctx, db.CountRevisionsBySessionParams{
						ProjectID: todo.ProjectID,
						SessionID: in.Body.SessionID,
					})
					if n == 0 {
						resolve = false
						churnDowngraded = true
					}
				}
			}
			if resolve {
				_ = h.Q.ResolveTodoByID(ctx, in.Body.ID)
			} else {
				_ = h.Q.ReleaseTodo(ctx, db.ReleaseTodoParams{
					ID:        in.Body.ID,
					ClaimedBy: in.Body.AgentID,
				})
			}
			return &struct {
				Body struct {
					OK              bool `json:"ok"`
					ChurnDowngraded bool `json:"churn_downgraded" required:"false"`
				}
			}{Body: struct {
				OK              bool `json:"ok"`
				ChurnDowngraded bool `json:"churn_downgraded" required:"false"`
			}{OK: true, ChurnDowngraded: churnDowngraded}}, nil
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
		ID:         r.ID,
		Text:       r.Text,
		Key:        r.Key,
		Persona:    r.Persona,
		Priority:   r.Priority,
		Status:     r.Status,
		TargetType: r.TargetType,
		TargetID:   r.TargetID,
		Kind:       r.Kind,
		IssueRef:   r.IssueRef,
		Blocked:    r.Blocked,
		ClaimedBy:  r.ClaimedBy,
		ClaimedAt:  fmtTS(r.ClaimedAt),
		CreatedAt:  fmtTS(r.CreatedAt),
		ResolvedAt: fmtTS(r.ResolvedAt),
	}
}

func toTodoItemFromFiltered(r db.ListTodosFilteredRow) TodoItem {
	return TodoItem{
		ID:         r.ID,
		Text:       r.Text,
		Key:        r.Key,
		Persona:    r.Persona,
		Priority:   r.Priority,
		Status:     r.Status,
		TargetType: r.TargetType,
		TargetID:   r.TargetID,
		Kind:       r.Kind,
		IssueRef:   r.IssueRef,
		Blocked:    r.Blocked,
		ClaimedBy:  r.ClaimedBy,
		ClaimedAt:  fmtTS(r.ClaimedAt),
		CreatedAt:  fmtTS(r.CreatedAt),
		ResolvedAt: fmtTS(r.ResolvedAt),
	}
}
