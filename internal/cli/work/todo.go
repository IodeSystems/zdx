package work

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/workflowhints"
)

func printSimilarIssues(cmd *cobra.Command, c *cli.Client, slug string, selfID int32, text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	n := int64(5)
	resp, err := c.SimilarIssuesWithResponse(cmd.Context(), dxclient.SimilarIssuesRequest{
		Slug: slug,
		Text: text,
		N:    &n,
	})
	if err != nil || resp.JSON200 == nil || resp.JSON200.Issues == nil {
		return
	}
	selfStr := clitypes.IssueIDStr(selfID)
	var matches []dxclient.SimilarIssueItem
	for _, s := range *resp.JSON200.Issues {
		if s.Id == selfStr {
			continue
		}
		matches = append(matches, s)
	}
	if len(matches) == 0 {
		return
	}
	fmt.Println("  similar issues (candidates for dup/link):")
	for _, s := range matches {
		fmt.Printf("    %-7s  (%3.0f%%)  [%s]  %s\n", s.Id, s.Score*100, s.Status, s.Title)
	}
}

func printTriageContext(cmd *cobra.Command, c *cli.Client, slug string) {
	if foResp, err := c.ListFocusesWithResponse(cmd.Context(), &dxclient.ListFocusesParams{Slug: slug}); err == nil &&
		foResp.JSON200 != nil && foResp.JSON200.Focuses != nil && len(*foResp.JSON200.Focuses) > 0 {
		fmt.Println("  active focuses:")
		for _, t := range *foResp.JSON200.Focuses {
			if t.Status == "active" {
				fmt.Printf("    FO-%-3d  %s\n", t.Id, t.Name)
			}
		}
	}
	if goalResp, err := c.ListGoalsWithResponse(cmd.Context(), &dxclient.ListGoalsParams{Slug: slug}); err == nil &&
		goalResp.JSON200 != nil && goalResp.JSON200.Goals != nil && len(*goalResp.JSON200.Goals) > 0 {
		fmt.Println("  active goals:")
		for _, g := range *goalResp.JSON200.Goals {
			if g.Status == "active" {
				fmt.Printf("    G-%-3d   %s\n", g.Id, g.Title)
			}
		}
	}
}

// ── TodoCmd ───────────────────────────────────────────────────────────────────

func TodoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Workflow queue",
		RunE:  func(cmd *cobra.Command, args []string) error { return soloRun(cmd, args) },
	}
	cmd.AddCommand(todoTakeCmd(), todoSoloCmd(), todoListCmd(), todoShowCmd(), todoDevCmd(), todoOwnerCmd(), todoTechCmd(), todoReservationsCmd(), todoReleaseCmd())
	return cmd
}

func todoTakeCmd() *cobra.Command {
	var agentIDFlag string
	var leaseMinutes int32
	cmd := &cobra.Command{
		Use:   "take",
		Short: "Claim next todo item (atomic reservation from server)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return todoTakeRun(cmd, agentIDFlag, leaseMinutes)
		},
	}
	cmd.Flags().StringVar(&agentIDFlag, "agent-id", "", "agent ID for claiming")
	cmd.Flags().Int32Var(&leaseMinutes, "lease-minutes", 30, "lease duration in minutes")
	return cmd
}

// todoTakeRun claims the next available todo via the server API.
func todoTakeRun(cmd *cobra.Command, agentID string, leaseMinutes int32) error {
	c := cli.MustClient()
	slug := c.SlugOrDie()
	if agentID == "" {
		agentID = "cli"
	}
	if leaseMinutes <= 0 {
		leaseMinutes = 30
	}
	resp, err := c.SoloClaimWithResponse(cmd.Context(), dxclient.SoloClaimRequest{
		Slug:         slug,
		AgentId:      agentID,
		LeaseMinutes: &leaseMinutes,
	})
	if err != nil || resp.StatusCode() >= 400 || resp.JSON200 == nil {
		fmt.Println("no work available")
		return nil
	}
	todo := resp.JSON200
	title := todo.Text
	if todo.Title != nil && *todo.Title != "" {
		title = *todo.Title
	}
	fmt.Printf("TODO-%d  [%s] %s\n", todo.Id, todo.Kind, title)
	if todo.IssueRef != "" {
		fmt.Printf("  issue: %s\n", todo.IssueRef)
	}
	if todo.TargetType != "" {
		fmt.Printf("  target: %s:%s\n", todo.TargetType, todo.TargetId)
	}
	claimedBy := ""
	if todo.ClaimedBy != nil {
		claimedBy = *todo.ClaimedBy
	}
	fmt.Printf("  claimed by: %s  lease: %d min\n", claimedBy, leaseMinutes)
	if todo.Description != nil && *todo.Description != "" {
		fmt.Printf("\nDescription:\n%s\n", *todo.Description)
	}
	if todo.Instructions != nil && *todo.Instructions != "" {
		fmt.Printf("\nInstructions:\n%s\n", *todo.Instructions)
	}
	return nil
}

// ── solo ──────────────────────────────────────────────────────────────────────

func todoSoloCmd() *cobra.Command {
	var issueFlag, agentIDFlag string
	var evaluateFlag, applyFlag bool
	cmd := &cobra.Command{
		Use:   "solo",
		Short: "Next actionable item",
		RunE: func(cmd *cobra.Command, args []string) error {
			if evaluateFlag || applyFlag {
				return soloEvaluateRun(cmd, evaluateFlag, applyFlag)
			}
			return soloRun(cmd, args)
		},
	}
	cmd.Flags().StringVar(&issueFlag, "issue", "", "filter to specific issue (IS-N)")
	cmd.Flags().StringVar(&agentIDFlag, "agent-id", "", "agent ID for atomic task claiming and heartbeat")
	cmd.Flags().BoolVar(&evaluateFlag, "evaluate", false, "regenerate full queue and show diff")
	cmd.Flags().BoolVar(&applyFlag, "apply", false, "regenerate and apply (implies --evaluate)")
	return cmd
}

func soloRun(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	issueFlag, _ := cmd.Flags().GetString("issue")
	agentID, _ := cmd.Flags().GetString("agent-id")
	c := cli.MustClient()
	slug := c.SlugOrDie()

	// When agent-id is set, validate the agent and extract task_group.
	var agentTaskGroup string
	var heartbeatStop chan struct{}
	if agentID != "" {
		agentResp, err := c.GetAgentWithResponse(ctx, agentID)
		if err != nil {
			return fmt.Errorf("agent %s not found: %w", agentID, err)
		}
		if err := c.CheckStatus(agentResp.StatusCode(), agentResp.Body); err != nil {
			return fmt.Errorf("agent %s not found: %w", agentID, err)
		}
		if agentResp.JSON200 == nil {
			return fmt.Errorf("agent %s not found", agentID)
		}
		agentTaskGroup = agentResp.JSON200.TaskGroup
		heartbeatStop = make(chan struct{})
		go cli.HeartbeatLoop(c, agentID, 60*time.Second, heartbeatStop)
		defer close(heartbeatStop)
	}

	// Fetch issues
	issueResp, err := c.ListIssuesWithResponse(ctx, &dxclient.ListIssuesParams{Slug: slug})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(issueResp.StatusCode(), issueResp.Body); err != nil {
		return err
	}
	var allIssues []dxclient.IssueItem
	if issueResp.JSON200 != nil && issueResp.JSON200.Issues != nil {
		allIssues = *issueResp.JSON200.Issues
	}

	// If --issue given, restrict to that one
	var targetIssues []dxclient.IssueItem
	if issueFlag != "" {
		for _, iss := range allIssues {
			if clitypes.IssueIDStr(iss.Id) == issueFlag {
				if iss.Status == "wip" {
					return fmt.Errorf("issue %s is still in draft (wip) — run `dx issue ready %s` to promote it", issueFlag, issueFlag)
				}
				targetIssues = append(targetIssues, iss)
				break
			}
		}
		if len(targetIssues) == 0 {
			return fmt.Errorf("issue %s not found", issueFlag)
		}
	} else {
		targetIssues = allIssues
	}

	// Exclude tracker issues — they are closed by their children, never actionable directly.
	// Closable trackers (all blockers closed) surface via the [close:tracker] scan below.
	var trackerIssues []dxclient.IssueItem
	{
		var filtered []dxclient.IssueItem
		for _, iss := range targetIssues {
			if iss.IssueType == "tracker" {
				trackerIssues = append(trackerIssues, iss)
			} else {
				filtered = append(filtered, iss)
			}
		}
		targetIssues = filtered
	}

	// 0. Check for unread LLM comments on any issue (regardless of status).
	for _, iss := range targetIssues {
		issIDStr := clitypes.IssueIDStr(iss.Id)
		unreadResp, err := c.CommentUnreadCheckWithResponse(ctx, &dxclient.CommentUnreadCheckParams{
			Slug:       slug,
			TargetType: "issue",
			TargetId:   issIDStr,
			Role:       "llm",
		})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(unreadResp.StatusCode(), unreadResp.Body); err != nil {
			return err
		}
		if unreadResp.JSON200 != nil && unreadResp.JSON200.HasUnread {
			fmt.Printf("[read:comments] %s  %s\n", issIDStr, iss.Title)
			tt, tid, role := "issue", issIDStr, "llm"
			listResp, err := c.ListCommentsWithResponse(ctx, &dxclient.ListCommentsParams{
				Slug: &slug, TargetType: &tt, TargetId: &tid, Role: &role,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(listResp.StatusCode(), listResp.Body); err != nil {
				return err
			}
			fmt.Println()
			if listResp.JSON200 != nil {
				cli.PrintComments(cli.CommentsToCli(listResp.JSON200.Comments))
			}
			_, _ = c.MarkCommentsReadWithResponse(ctx, dxclient.MarkCommentsReadRequest{
				Slug:       slug,
				TargetType: "issue",
				TargetId:   issIDStr,
				Role:       "llm",
			})
			return nil
		}
	}

	// 0b. Check for unread LLM comments on any feature.
	featResp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: slug})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(featResp.StatusCode(), featResp.Body); err != nil {
		return err
	}
	var allFeatures []dxclient.FeatureItem
	if featResp.JSON200 != nil && featResp.JSON200.Features != nil {
		allFeatures = *featResp.JSON200.Features
	}
	for _, f := range allFeatures {
		unreadResp, err := c.CommentUnreadCheckWithResponse(ctx, &dxclient.CommentUnreadCheckParams{
			Slug:       slug,
			TargetType: "feature",
			TargetId:   f.Name,
			Role:       "llm",
		})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(unreadResp.StatusCode(), unreadResp.Body); err != nil {
			return err
		}
		if unreadResp.JSON200 != nil && unreadResp.JSON200.HasUnread {
			fmt.Printf("[read:comments] feature %q\n", f.Name)
			tt, tid, role := "feature", f.Name, "llm"
			listResp, err := c.ListCommentsWithResponse(ctx, &dxclient.ListCommentsParams{
				Slug: &slug, TargetType: &tt, TargetId: &tid, Role: &role,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(listResp.StatusCode(), listResp.Body); err != nil {
				return err
			}
			fmt.Println()
			if listResp.JSON200 != nil {
				cli.PrintComments(cli.CommentsToCli(listResp.JSON200.Comments))
			}
			_, _ = c.MarkCommentsReadWithResponse(ctx, dxclient.MarkCommentsReadRequest{
				Slug:       slug,
				TargetType: "feature",
				TargetId:   f.Name,
				Role:       "llm",
			})
			return nil
		}
	}

	// 0b2. Check for unanswered QA questions (oldest first).
	{
		resp, err := c.ListUnansweredQuestionsWithResponse(ctx, &dxclient.ListUnansweredQuestionsParams{Slug: slug})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		if resp.JSON200 != nil && resp.JSON200.Questions != nil && len(*resp.JSON200.Questions) > 0 {
			q := (*resp.JSON200.Questions)[0]
			fmt.Printf("[answer]  QA-%d  %s\n", q.Id, q.Question)
			if q.Category != "" {
				fmt.Printf("  category: %s\n", q.Category)
			}
			fmt.Printf("  asked: %s\n", q.CreatedAt)
			fmt.Printf("  answer: dx qa answer %d --answer=\"...\"\n", q.Id)
			return nil
		}
	}

	// 0b3. Check for stale unread comments (>24h old, unread for LLM role).
	// Skip in scoped mode — matches server-side gating in handlers_solo.go where
	// cross-cutting checks only run when issueFilter is empty.
	if issueFlag == "" {
		ageHours := int32(24)
		resp, err := c.CommentStaleUnreadWithResponse(ctx, &dxclient.CommentStaleUnreadParams{
			Slug:     slug,
			Role:     "llm",
			AgeHours: &ageHours,
		})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		if resp.JSON200 != nil && resp.JSON200.Comments != nil && len(*resp.JSON200.Comments) > 0 {
			sc := (*resp.JSON200.Comments)[0]
			fmt.Printf("[respond:stale] %s %s\n", sc.TargetType, sc.TargetId)
			fmt.Printf("  from: %s  at: %s\n", sc.Author, sc.CreatedAt)
			fmt.Printf("  %s\n", sc.Body)
			fmt.Printf("  clear: dx comment mark-read %s %s --role=llm\n", sc.TargetType, sc.TargetId)
			return nil
		}
	}

	// 0c. Check for pending blocker-questions on open issues.
	bqBlockedIssues := map[string]bool{}
	{
		status := "pending"
		resp, err := c.ListBlockerQuestionsWithResponse(ctx, &dxclient.ListBlockerQuestionsParams{
			Slug:   slug,
			Status: &status,
		})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
			return err
		}
		var questions []dxclient.BlockerQuestionItem
		if resp.JSON200 != nil && resp.JSON200.Questions != nil {
			questions = *resp.JSON200.Questions
		}
		for _, q := range questions {
			if q.TargetType != "issue" {
				continue
			}
			matched := false
			for _, iss := range targetIssues {
				if clitypes.IssueIDStr(iss.Id) == q.TargetId && iss.Status == "open" {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			if issueFlag != "" {
				// Scoped to a single issue — block until answered.
				fmt.Printf("[clarify] %s  BQ-%d\n", q.TargetId, q.Id)
				fmt.Printf("  %s\n", q.Context)
				if q.Choices != nil {
					for i, ch := range *q.Choices {
						fmt.Printf("    %d. %s\n", i+1, ch)
					}
				}
				fmt.Printf("  answer: dx question answer %d --answer=\"...\"\n", q.Id)
				return nil
			}
			// Global mode — skip this issue, pick other work.
			bqBlockedIssues[q.TargetId] = true
		}
	}
	// Filter out BQ-blocked issues in global mode.
	if issueFlag == "" && len(bqBlockedIssues) > 0 {
		filtered := targetIssues[:0]
		for _, iss := range targetIssues {
			if !bqBlockedIssues[clitypes.IssueIDStr(iss.Id)] {
				filtered = append(filtered, iss)
			}
		}
		targetIssues = filtered
	}

	// 0c1. Check for stale tasks (flagged by recovery sweep, never claimed).
	{
		params := &dxclient.ListStaleTasksParams{Slug: slug}
		if issueFlag != "" {
			v := issueFlag
			params.Issue = &v
		}
		resp, err := c.ListStaleTasksWithResponse(ctx, params)
		if err == nil && resp.JSON200 != nil && resp.JSON200.Tasks != nil && len(*resp.JSON200.Tasks) > 0 {
			t := (*resp.JSON200.Tasks)[0]
			fmt.Printf("[review:stale] %s  %s\n", clitypes.TaskIDStr(t.Id), taskHeadline(t.Title, t.Text))
			if t.IssueId != nil {
				fmt.Printf("  issue: %s\n", clitypes.IssueIDStr(*t.IssueId))
			}
			fmt.Println("  ⚠ This task was flagged stale — it was created but never claimed.")
			fmt.Println("  Before editing, verify the work is still needed by reading the current code.")
			fmt.Println("  If already implemented: dx todo dev done " + clitypes.TaskIDStr(t.Id))
			fmt.Println("  If superseded: dx todo dev done " + clitypes.TaskIDStr(t.Id))
			return nil
		}
	}

	// 0d. Bootstrap: if no issues exist at all, auto-create an onboarding issue so the queue has immediate work.
	if issueFlag == "" && len(allIssues) == 0 && len(allFeatures) == 0 {
		fmt.Printf("[bootstrap] %s\n", slug)
		if err := cli.BootstrapOnboardingIssue(c); err != nil {
			fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
		} else {
			fmt.Println("\nrun dx todo solo again to start the normal triage flow.")
		}
		return nil
	}

	// 0e. Check project health: goals, constraints, journal cadence.
	// Only in global mode — these are cross-cutting owner/tech concerns.
	if issueFlag == "" {
		resp, err := c.SoloHealthWithResponse(ctx, &dxclient.SoloHealthParams{Slug: slug})
		if err == nil && resp.JSON200 != nil {
			health := resp.JSON200
			if health.GoalCount == 0 {
				fmt.Println("[owner:goals]  project has no goals defined — dx goal add <title>")
				return nil
			}
			if health.ConstraintCount == 0 {
				fmt.Println("[owner:constraints]  project has no constraints defined — dx constraint add <title>")
				return nil
			}
			if overdue, role := journalOverdue(health.OwnerJournalDate, health.TechJournalDate, health.ClosedTaskCount); overdue {
				fmt.Printf("[%s:standup]  %s standup check-in overdue — dx standup checkin --%s\n", role, role, role)
				return nil
			}
		}
	}

	// 1. Find untriaged open issue (no priority)
	for _, iss := range targetIssues {
		if iss.Status == "open" && iss.Priority == "" {
			fmt.Printf("[triage] %s  %s\n", clitypes.IssueIDStr(iss.Id), iss.Title)
			if iss.IssueType != "" {
				fmt.Printf("  type:      %s\n", iss.IssueType)
			}
			if iss.Component != "" {
				fmt.Printf("  component: %s\n", iss.Component)
			}
			if iss.Context != "" {
				fmt.Printf("\n%s\n", iss.Context)
			}
			printTriageContext(cmd, c, slug)
			printSimilarIssues(cmd, c, slug, iss.Id, iss.Title+" "+iss.Context)
			fmt.Print(workflowhints.TriageChecklist)
			return nil
		}
	}

	// Cross-cutting owner/tech checks (1b, 1c, 2a) scan all features/specs globally;
	// skip them when --issue is set so vertical scope is preserved.
	if issueFlag == "" {
		// 1b. Check for ANY feature with no specs (reuse allFeatures from 0b).
		for _, f := range allFeatures {
			if f.Specs == nil || len(*f.Specs) == 0 {
				fmt.Printf("[owner:spec]  feature %q has no specs — dx feature spec add %q\n", f.Name, f.Name)
				return nil
			}
		}

		// 1c. Check for features due for periodic owner re-review (>30 days stale).
		staleResp, err := c.ListStaleFeaturesWithResponse(ctx, &dxclient.ListStaleFeaturesParams{Slug: slug})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(staleResp.StatusCode(), staleResp.Body); err != nil {
			return err
		}
		if staleResp.JSON200 != nil && staleResp.JSON200.Features != nil && len(*staleResp.JSON200.Features) > 0 {
			f := (*staleResp.JSON200.Features)[0]
			fmt.Printf("[owner:review]  feature %q not reviewed in >30 days — dx feature review %q\n", f.Name, f.Name)
			return nil
		}

		// 2a. Check for specs with no test_refs — tech lead owns this.
		uncoveredResp, err := c.ListUncoveredSpecsWithResponse(ctx, &dxclient.ListUncoveredSpecsParams{Slug: slug})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(uncoveredResp.StatusCode(), uncoveredResp.Body); err != nil {
			return err
		}
		if uncoveredResp.JSON200 != nil && uncoveredResp.JSON200.Specs != nil && len(*uncoveredResp.JSON200.Specs) > 0 {
			s := (*uncoveredResp.JSON200.Specs)[0]
			fmt.Printf("[tech:test-ref]  feature %q spec %d (%s) has no test refs — add task or link via dx spec link\n", s.FeatureName, s.Id, s.Description)
			return nil
		}

		// 2b. Check for specs linked to tests but missing demo artifacts.
		demoGapResp, err := c.ListSpecsWithoutDemosWithResponse(ctx, &dxclient.ListSpecsWithoutDemosParams{Slug: slug})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(demoGapResp.StatusCode(), demoGapResp.Body); err != nil {
			return err
		}
		if demoGapResp.JSON200 != nil && demoGapResp.JSON200.Specs != nil && len(*demoGapResp.JSON200.Specs) > 0 {
			s := (*demoGapResp.JSON200.Specs)[0]
			fmt.Printf("[owner:demo-gap]  feature %q spec %d (%s) has no demo — write a new demo test OR link an existing one (dx test list --layer=demo → dx spec link %d <test-id>)\n", s.FeatureName, s.Id, s.Description, s.Id)
			return nil
		}
	}

	// 1d. Surface tracker issues that are ready to close (all blockers closed).
	// list-issues omits BlockedByDetail; fetch individual details for tracker issues.
	for i, iss := range trackerIssues {
		if iss.BlockedByDetail == nil || len(*iss.BlockedByDetail) == 0 {
			issIDStr := clitypes.IssueIDStr(iss.Id)
			showResp, err := c.ShowIssueWithResponse(ctx, &dxclient.ShowIssueParams{Slug: slug, Id: issIDStr})
			if err == nil && showResp.JSON200 != nil {
				trackerIssues[i] = showResp.JSON200.Issue
				iss = trackerIssues[i]
			}
		}
		if iss.Status != "open" {
			continue
		}
		if iss.BlockedByDetail == nil || len(*iss.BlockedByDetail) == 0 {
			continue
		}
		allClosed := true
		for _, b := range *iss.BlockedByDetail {
			if b.Status != "closed" {
				allClosed = false
				break
			}
		}
		if !allClosed {
			continue
		}
		issIDStr := clitypes.IssueIDStr(iss.Id)
		fmt.Printf("[close:tracker] %s  %s\n", issIDStr, iss.Title)
		fmt.Println("  All dependency issues are closed — tracker is ready to close.")
		fmt.Printf("  close: dx issue close %s --reason=done\n", issIDStr)
		return nil
	}

	// 1e. Deferred specs whose blockers are all closed — resurface for re-evaluation (global only).
	if issueFlag == "" {
		brResp, err := c.ListSpecsBlockerResolvedWithResponse(ctx, &dxclient.ListSpecsBlockerResolvedParams{Slug: slug})
		if err == nil && brResp.JSON200 != nil && brResp.JSON200.Specs != nil && len(*brResp.JSON200.Specs) > 0 {
			s := (*brResp.JSON200.Specs)[0]
			fmt.Printf("[review:deferred-spec]  spec %d (%s) — all blocking issues closed, re-evaluate\n", s.Id, s.Description)
			fmt.Println("  Options: write/link the test (dx spec link), re-defer (dx spec defer --blocked-by=<IS-N>), or remove.")
			fmt.Printf("  hint: dx spec show %d\n", s.Id)
			return nil
		}
	}

	// 2. Find open issue with no pending tasks
	for _, iss := range targetIssues {
		if iss.Status != "open" {
			continue
		}
		issIDStr := clitypes.IssueIDStr(iss.Id)
		taskResp, err := c.ListTasksForIssueWithResponse(ctx, &dxclient.ListTasksForIssueParams{
			Slug:    slug,
			IssueId: issIDStr,
		})
		if err != nil {
			return err
		}
		if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
			return err
		}
		var tasks []dxclient.TaskItem
		if taskResp.JSON200 != nil && taskResp.JSON200.Tasks != nil {
			tasks = *taskResp.JSON200.Tasks
		}
		hasPending := false
		for _, t := range tasks {
			if t.Status == "ready" || t.Status == "active" {
				hasPending = true
				break
			}
		}
		if !hasPending {
			if len(tasks) > 0 {
				issueRef := issIDStr
				unreviewedResp, err := c.ListUnreviewedTasksWithResponse(ctx, &dxclient.ListUnreviewedTasksParams{
					Slug:  slug,
					Issue: &issueRef,
				})
				if err == nil && unreviewedResp.JSON200 != nil && unreviewedResp.JSON200.Tasks != nil && len(*unreviewedResp.JSON200.Tasks) > 0 {
					t := (*unreviewedResp.JSON200.Tasks)[0]
					fmt.Printf("[review]  %s  %s\n", clitypes.TaskIDStr(t.Id), taskHeadline(t.Title, t.Text))
					fmt.Printf("  issue: %s\n", issIDStr)
					return nil
				}
				fmt.Printf("[closable] %s  %s\n", issIDStr, iss.Title)
			} else {
				fmt.Printf("[add]     %s  %s\n", issIDStr, iss.Title)
				fmt.Fprintln(os.Stderr, "  hint: dx todo tech add --issue="+issIDStr+" --title=<outcome> --text=<plan> --reason=<why> --test-plan=<verification>")
			}
			return nil
		}
	}

	// 3. Find a pending task
	if agentID != "" {
		// Agent mode: atomically claim a task via the ClaimTask API.
		claimResp, err := c.ClaimTaskWithResponse(ctx, dxclient.ClaimTaskRequest{
			Slug:      slug,
			AgentId:   agentID,
			TaskGroup: agentTaskGroup,
			Issue:     issueFlag,
		})
		if err == nil && claimResp.JSON200 != nil {
			claimed := claimResp.JSON200
			printStaleWarning(claimed.CreatedAt, claimed.Id)
			fmt.Printf("[dev]     %s  %s\n", claimed.Id, taskHeadline(claimed.Title, claimed.Text))
			if claimed.Issue != "" {
				fmt.Printf("  issue: %s\n", claimed.Issue)
			}
			fmt.Printf("  claimed_by: %s\n", agentID)
			return nil
		}
	} else {
		type issueTasks struct {
			issue dxclient.IssueItem
			tasks []dxclient.TaskItem
		}
		fetched := make([]issueTasks, 0, len(targetIssues))
		for _, iss := range targetIssues {
			if iss.Status != "open" {
				continue
			}
			taskResp, err := c.ListTasksForIssueWithResponse(ctx, &dxclient.ListTasksForIssueParams{
				Slug:    slug,
				IssueId: clitypes.IssueIDStr(iss.Id),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
				return err
			}
			var tasks []dxclient.TaskItem
			if taskResp.JSON200 != nil && taskResp.JSON200.Tasks != nil {
				tasks = *taskResp.JSON200.Tasks
			}
			fetched = append(fetched, issueTasks{issue: iss, tasks: tasks})
		}
		// Prefer an active task (resume uncommitted work) before any ready pick.
		for _, ft := range fetched {
			for _, t := range ft.tasks {
				if t.Status == "active" {
					fmt.Printf("[dev]     %s  %s\n", clitypes.TaskIDStr(t.Id), taskHeadline(t.Title, t.Text))
					fmt.Printf("  issue: %s\n", clitypes.IssueIDStr(ft.issue.Id))
					fmt.Fprintln(os.Stderr, "  note: resuming active task. Use --agent-id to claim before starting work.")
					return nil
				}
			}
		}
		// Within an issue, pick the lowest-ID ready task — task creation order usually
		// matches execution order, and the API returns tasks sorted by updated_at DESC.
		for _, ft := range fetched {
			pending := make([]dxclient.TaskItem, 0, len(ft.tasks))
			for _, t := range ft.tasks {
				if t.Status == "ready" {
					pending = append(pending, t)
				}
			}
			if len(pending) == 0 {
				continue
			}
			sort.Slice(pending, func(i, j int) bool { return pending[i].Id < pending[j].Id })
			t := pending[0]
			printStaleWarning(t.CreatedAt, clitypes.TaskIDStr(t.Id))
			fmt.Printf("[dev]     %s  %s\n", clitypes.TaskIDStr(t.Id), taskHeadline(t.Title, t.Text))
			fmt.Printf("  issue: %s\n", clitypes.IssueIDStr(ft.issue.Id))
			fmt.Fprintln(os.Stderr, "  note: task not claimed. Use --agent-id to claim before starting work.")
			return nil
		}
	}

	fmt.Fprintln(os.Stderr, "nothing to do")
	os.Exit(2)
	return nil
}

const staleTaskDays = 2

func printStaleWarning(createdAt string, taskID string) {
	if createdAt == "" {
		return
	}
	t, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		t, err = time.Parse(time.RFC3339, createdAt)
		if err != nil {
			return
		}
	}
	age := time.Since(t)
	if age < time.Duration(staleTaskDays)*24*time.Hour {
		return
	}
	days := int(age.Hours() / 24)
	fmt.Printf("  ⚠ state unknown: this task was created %d days ago and has never been worked.\n", days)
	fmt.Println("    Before editing, verify the work is still needed by reading the current code.")
	fmt.Printf("    If already implemented, run: ./bin/dx todo dev done %s\n", taskID)
}

// ── evaluate ─────────────────────────────────────────────────────────────────

// soloQueueItem is the wire shape of one queue item shared between evaluate
// and apply. Aliased to the generated dxclient type so the apply request can
// embed a []soloQueueItem directly and any server-side schema change surfaces
// as a compile error here.
type soloQueueItem = dxclient.SoloQueueItem

func soloEvaluateRun(cmd *cobra.Command, showDiff bool, apply bool) error {
	ctx := cmd.Context()
	issueFlag, _ := cmd.Flags().GetString("issue")
	c := cli.MustClient()
	slug := c.SlugOrDie()

	resp, err := c.SoloEvaluateWithResponse(ctx, dxclient.SoloEvaluateRequest{
		Slug:  slug,
		Issue: issueFlag,
	})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("empty evaluate response")
	}
	diff := resp.JSON200

	added := valueSlice(diff.Added)
	removed := valueSlice(diff.Removed)
	changed := valueSlice(diff.Changed)
	unchanged := valueSlice(diff.Unchanged)

	if len(added) == 0 && len(removed) == 0 && len(changed) == 0 {
		fmt.Printf("queue up to date (%d items)\n", len(unchanged))
		if !apply {
			return nil
		}
	}

	if len(added) > 0 {
		fmt.Printf("+ %d new:\n", len(added))
		for _, a := range added {
			fmt.Printf("  + [%s] %s  %s\n", a.Kind, a.TargetId, soloItemTitle(a))
		}
	}
	if len(removed) > 0 {
		fmt.Printf("- %d removed:\n", len(removed))
		for _, r := range removed {
			title := r.Text
			if r.Title != nil && *r.Title != "" {
				title = *r.Title
			}
			fmt.Printf("  - [%s] %s\n", r.Kind, truncateLine(title, 80))
		}
	}
	if len(changed) > 0 {
		fmt.Printf("~ %d changed:\n", len(changed))
		for _, ch := range changed {
			fmt.Printf("  ~ [%s] %s\n", ch.After.Kind, soloItemTitle(ch.After))
		}
	}

	if !apply {
		fmt.Println("\nrun with --apply to commit changes")
		return nil
	}

	// Collect all items (added + changed + unchanged) and apply
	var items []soloQueueItem
	items = append(items, added...)
	for _, ch := range changed {
		items = append(items, ch.After)
	}
	items = append(items, unchanged...)

	applyResp, err := c.SoloApplyWithResponse(ctx, dxclient.SoloApplyRequest{
		Slug:  slug,
		Items: &items,
	})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(applyResp.StatusCode(), applyResp.Body); err != nil {
		return err
	}
	fmt.Printf("applied: %d items in queue\n", len(items))
	return nil
}

func valueSlice[T any](p *[]T) []T {
	if p == nil {
		return nil
	}
	return *p
}

// ── list ──────────────────────────────────────────────────────────────────────

func todoListCmd() *cobra.Command {
	var issue, feature string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := cli.MustClient()
			slug := c.SlugOrDie()

			if issue != "" {
				resp, err := c.ListTasksForIssueWithResponse(ctx, &dxclient.ListTasksForIssueParams{
					Slug:    slug,
					IssueId: issue,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				printTasksDx(resp.JSON200.Tasks)
				return nil
			}
			if feature != "" {
				resp, err := c.ListTasksByFeatureWithResponse(ctx, &dxclient.ListTasksByFeatureParams{
					Slug:    slug,
					Feature: feature,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				printTasksDx(resp.JSON200.Tasks)
				return nil
			}
			resp, err := c.ListTasksWithResponse(ctx, &dxclient.ListTasksParams{Slug: slug})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			printTasksDx(resp.JSON200.Tasks)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "filter by issue (IS-N)")
	cmd.Flags().StringVar(&feature, "feature", "", "filter by feature name")
	return cmd
}

func printTasksDx(tasks *[]dxclient.TaskItem) {
	if tasks == nil || len(*tasks) == 0 {
		fmt.Println("no tasks")
		return
	}
	for _, t := range *tasks {
		headline := t.Title
		if headline == "" {
			headline = truncateLine(t.Text, 80)
		}
		fmt.Printf("%-8s %-12s %s\n", clitypes.TaskIDStr(t.Id), t.Status, headline)
	}
}

func truncateLine(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

// taskHeadline returns the title if set, otherwise a truncated single-line
// rendering of the implementation plan text.
func taskHeadline(title, text string) string {
	if title != "" {
		return title
	}
	return truncateLine(text, 80)
}

func soloItemTitle(item soloQueueItem) string {
	if item.Title != nil && *item.Title != "" {
		return *item.Title
	}
	return truncateLine(item.Text, 80)
}

// ── show ──────────────────────────────────────────────────────────────────────

func todoShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <IS-N|TK-N|feature-name>",
		Short: "Show issue, task, or feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			c := cli.MustClient()
			slug := c.SlugOrDie()

			switch {
			case len(id) > 3 && id[:3] == "IS-":
				resp, err := c.ShowIssueWithResponse(ctx, &dxclient.ShowIssueParams{Slug: slug, Id: id})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				if resp.JSON200 == nil {
					return fmt.Errorf("empty issue response")
				}
				cli.PrintIssueItem(cli.IssueToCli(resp.JSON200.Issue))
				if resp.JSON200.Work != nil && len(*resp.JSON200.Work) > 0 {
					fmt.Println("\nWork log:")
					for _, w := range *resp.JSON200.Work {
						date := w.CreatedAt
						if len(date) >= 10 {
							date = date[:10]
						}
						fmt.Printf("  [%s] %s: %s\n", date, w.Agent, w.Note)
					}
				}
				tt, tid, role := "issue", id, "llm"
				commResp, err := c.ListCommentsWithResponse(ctx, &dxclient.ListCommentsParams{
					Slug: &slug, TargetType: &tt, TargetId: &tid, Role: &role,
				})
				if err == nil && commResp.JSON200 != nil && commResp.JSON200.Comments != nil && len(*commResp.JSON200.Comments) > 0 {
					fmt.Println("\nComments:")
					cli.PrintComments(cli.CommentsToCli(commResp.JSON200.Comments))
				}
				printSimilarPatterns(cmd, c, slug, resp.JSON200.Issue.Title+" "+resp.JSON200.Issue.Context)
			case len(id) > 3 && id[:3] == "TK-":
				n, _ := strconv.ParseInt(id[3:], 10, 32)
				taskID := int32(n)
				resp, err := c.ListTasksWithResponse(ctx, &dxclient.ListTasksParams{Slug: slug})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				if resp.JSON200 != nil && resp.JSON200.Tasks != nil {
					for _, t := range *resp.JSON200.Tasks {
						if t.Id == taskID {
							printTaskItem(cli.TaskToCli(t))
							if todoResp, err := c.GetTodoDetailWithResponse(ctx, slug, id); err == nil && todoResp.JSON200 != nil {
								td := todoResp.JSON200.Todo
								if td.Description != nil && *td.Description != "" {
									fmt.Printf("\nContext:\n%s\n", indent(*td.Description, "  "))
								}
								if td.Instructions != nil && *td.Instructions != "" {
									fmt.Printf("\nInstructions:\n%s\n", indent(*td.Instructions, "  "))
								}
							}
							tt, tid, role := "task", id, "llm"
							commResp, err := c.ListCommentsWithResponse(ctx, &dxclient.ListCommentsParams{
								Slug: &slug, TargetType: &tt, TargetId: &tid, Role: &role,
							})
							if err == nil && commResp.JSON200 != nil && commResp.JSON200.Comments != nil && len(*commResp.JSON200.Comments) > 0 {
								fmt.Println("\nComments:")
								cli.PrintComments(cli.CommentsToCli(commResp.JSON200.Comments))
							}
							revTT, revTID := "task", id
							revResp, err := c.ListRevisionsWithResponse(ctx, &dxclient.ListRevisionsParams{
								Slug: &slug, TargetType: &revTT, TargetId: &revTID,
							})
							if err == nil && revResp.JSON200 != nil && revResp.JSON200.Revisions != nil && len(*revResp.JSON200.Revisions) > 0 {
								fmt.Println("\nRevisions:")
								for _, r := range *revResp.JSON200.Revisions {
									date := r.CreatedAt
									if len(date) >= 10 {
										date = date[:10]
									}
									fmt.Printf("  [%s] %s: %s → %s (%s)\n", date, r.Field, r.OldVal, r.NewVal, r.Agent)
								}
							}
							printSimilarPatterns(cmd, c, slug, t.Text)
							return nil
						}
					}
				}
				return fmt.Errorf("task %s not found", id)
			default:
				featResp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: slug})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(featResp.StatusCode(), featResp.Body); err != nil {
					return err
				}
				if featResp.JSON200 != nil && featResp.JSON200.Features != nil {
					for _, f := range *featResp.JSON200.Features {
						if f.Name == id {
							cli.PrintFeatureItem(cli.FeatureToCli(f))
							tt, tid, role := "feature", f.Name, "llm"
							commResp, err := c.ListCommentsWithResponse(ctx, &dxclient.ListCommentsParams{
								Slug: &slug, TargetType: &tt, TargetId: &tid, Role: &role,
							})
							if err == nil && commResp.JSON200 != nil && commResp.JSON200.Comments != nil && len(*commResp.JSON200.Comments) > 0 {
								fmt.Println("\nComments:")
								cli.PrintComments(cli.CommentsToCli(commResp.JSON200.Comments))
							}
							revTT, revTID := "feature", f.Name
							revResp, err := c.ListRevisionsWithResponse(ctx, &dxclient.ListRevisionsParams{
								Slug: &slug, TargetType: &revTT, TargetId: &revTID,
							})
							if err == nil && revResp.JSON200 != nil && revResp.JSON200.Revisions != nil && len(*revResp.JSON200.Revisions) > 0 {
								fmt.Println("\nRevisions:")
								for _, r := range *revResp.JSON200.Revisions {
									date := r.CreatedAt
									if len(date) >= 10 {
										date = date[:10]
									}
									fmt.Printf("  [%s] %s: %s → %s (%s)\n", date, r.Field, r.OldVal, r.NewVal, r.Agent)
								}
							}
							return nil
						}
					}
				}
				return fmt.Errorf("feature %q not found", id)
			}
			return nil
		},
	}
}

// ── dev ───────────────────────────────────────────────────────────────────────

func todoDevCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "dev", Short: "Task development actions"}
	cmd.AddCommand(todoDevDoneCmd(), todoDevUndoneCmd(), todoDevStartCmd(), todoDevReviewCmd())
	return cmd
}

func todoDevDoneCmd() *cobra.Command {
	var testPlan, testRefs string
	var files []string
	cmd := &cobra.Command{
		Use:   "done <TK-N>",
		Short: "Mark task done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.RunCloseHooks(); err != nil {
				return err
			}
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			ctx := cmd.Context()
			slug := c.SlugOrDie()

			specs := make([]fileSpec, 0, len(files))
			for _, raw := range files {
				s, err := parseFileSpec(raw)
				if err != nil {
					return fmt.Errorf("--file %q: %w", raw, err)
				}
				specs = append(specs, s)
			}

			taskResp, err := c.GetTaskWithResponse(ctx, &dxclient.GetTaskParams{Slug: slug, Id: id})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
				return err
			}
			if taskResp.JSON200 == nil {
				return fmt.Errorf("task %s not found", id)
			}
			task := taskResp.JSON200

			if strings.TrimSpace(testPlan) == "" && strings.TrimSpace(task.TestPlan) == "" {
				return workflowhints.MissingTestPlanError(id)
			}

			var issueRef string
			if task.IssueId != nil {
				issueRef = clitypes.IssueIDStr(*task.IssueId)
				issueResp, err := c.ShowIssueWithResponse(ctx, &dxclient.ShowIssueParams{Slug: slug, Id: issueRef})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(issueResp.StatusCode(), issueResp.Body); err != nil {
					return err
				}
				if issueResp.JSON200 != nil && issueResp.JSON200.Issue.IssueType == "impl" {
					hasRefs := strings.TrimSpace(testRefs) != "" || strings.TrimSpace(task.TestRefs) != "" || len(specs) > 0
					if !hasRefs {
						return workflowhints.MissingTestRefsError(id, issueRef)
					}
				}
			}

			if len(specs) > 0 && issueRef == "" {
				return fmt.Errorf("task %s has no parent issue — cannot attach code refs", id)
			}

			resp, err := c.MarkTaskDoneWithResponse(ctx, dxclient.MarkTaskDoneRequest{
				Id:       int32(n),
				TestPlan: &testPlan,
				TestRefs: &testRefs,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s done\n", id)

			if len(specs) > 0 {
				head := gitHead()
				for _, s := range specs {
					body := dxclient.AttachCodeRefToIssueRequest{
						Slug:     slug,
						IssueId:  issueRef,
						FilePath: s.path,
					}
					hash := s.hash
					if hash == "" {
						hash = head
					}
					if hash != "" {
						body.GitHash = &hash
					}
					if s.lineStart > 0 {
						body.LineStart = &s.lineStart
					}
					if s.lineEnd > 0 {
						body.LineEnd = &s.lineEnd
					}
					aResp, err := c.AttachCodeRefToIssueWithResponse(ctx, body)
					if err != nil {
						return fmt.Errorf("attach %s: %w", s.path, err)
					}
					if err := c.CheckStatus(aResp.StatusCode(), aResp.Body); err != nil {
						return fmt.Errorf("attach %s: %w", s.path, err)
					}
					if aResp.JSON200 != nil {
						fmt.Print("  ref ")
						printCodeRefInline(*aResp.JSON200)
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&testPlan, "test-plan", "", "test plan description")
	cmd.Flags().StringVar(&testRefs, "test-refs", "", "test references")
	cmd.Flags().StringArrayVar(&files, "file", nil, "attach touched file to parent issue as code ref; repeatable; syntax <path>[:start[-end]][@hash] (empty hash → git HEAD)")
	return cmd
}

type fileSpec struct {
	path      string
	lineStart int32
	lineEnd   int32
	hash      string
}

func parseFileSpec(spec string) (fileSpec, error) {
	out := fileSpec{}
	s := strings.TrimSpace(spec)
	if s == "" {
		return out, fmt.Errorf("empty spec")
	}
	if idx := strings.LastIndex(s, "@"); idx >= 0 {
		out.hash = s[idx+1:]
		s = s[:idx]
	}
	if idx := strings.LastIndex(s, ":"); idx >= 0 {
		suffix := s[idx+1:]
		if dash := strings.Index(suffix, "-"); dash >= 0 {
			start, errA := strconv.ParseInt(suffix[:dash], 10, 32)
			end, errB := strconv.ParseInt(suffix[dash+1:], 10, 32)
			if errA == nil && errB == nil {
				out.lineStart = int32(start)
				out.lineEnd = int32(end)
				s = s[:idx]
			}
		} else if n, err := strconv.ParseInt(suffix, 10, 32); err == nil {
			out.lineStart = int32(n)
			s = s[:idx]
		}
	}
	if s == "" {
		return out, fmt.Errorf("empty path")
	}
	out.path = s
	return out, nil
}

func gitHead() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func printCodeRefInline(r dxclient.CodeRefItem) {
	loc := r.FilePath
	if r.LineStart > 0 {
		if r.LineEnd > 0 && r.LineEnd != r.LineStart {
			loc += fmt.Sprintf(":%d-%d", r.LineStart, r.LineEnd)
		} else {
			loc += fmt.Sprintf(":%d", r.LineStart)
		}
	}
	hash := r.GitHash
	if len(hash) > 8 {
		hash = hash[:8]
	}
	if hash != "" {
		fmt.Printf("[%d] %s @%s\n", r.Id, loc, hash)
	} else {
		fmt.Printf("[%d] %s\n", r.Id, loc)
	}
}

func todoDevUndoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undone <TK-N>",
		Short: "Reopen task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			resp, err := c.MarkTaskUndoneWithResponse(cmd.Context(), dxclient.MarkTaskUndoneRequest{Id: int32(n)})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s undone\n", id)
			return nil
		},
	}
}

func todoDevStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <TK-N>",
		Short: "Mark task in-progress and create a reservation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			req := dxclient.StartTaskRequest{Id: int32(n)}
			if alias := os.Getenv("DX_AUTHOR_ALIAS"); alias != "" {
				req.ClaimedBy = &alias
			}
			resp, err := c.StartTaskWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s started\n", id)
			return nil
		},
	}
	return cmd
}

func todoDevReviewCmd() *cobra.Command {
	var verdict, body string
	cmd := &cobra.Command{
		Use:   "review <TK-N>",
		Short: "Review a completed task or submit review verdict",
		Long:  "Without --verdict: prints task details and review material.\nWith --verdict: marks the task as reviewed and persists verdict + --body on the review record (NOT a task comment).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			slug := c.SlugOrDie()

			if verdict != "" {
				req := dxclient.MarkTaskReviewedRequest{
					Slug:    slug,
					Id:      int32(n),
					Verdict: verdict,
				}
				if body != "" {
					req.Body = &body
				}
				resp, err := c.MarkTaskReviewedWithResponse(ctx, req)
				if err != nil {
					return err
				}
				if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
					return err
				}
				fmt.Printf("%s reviewed [%s]", id, verdict)
				if resp.JSON200 != nil && resp.JSON200.ReviewId != 0 {
					fmt.Printf(" review_id=%d", resp.JSON200.ReviewId)
				}
				fmt.Println()
				return nil
			}

			resp, err := c.GetReviewDataWithResponse(ctx, &dxclient.GetReviewDataParams{
				Slug: slug,
				Id:   int32(n),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty review-data response")
			}
			data := resp.JSON200

			fmt.Printf("Task:       %s\n", id)
			fmt.Printf("Text:       %s\n", data.Task.Text)
			fmt.Printf("Status:     %s\n", data.Task.Status)
			fmt.Printf("Issue type: %s\n", data.IssueType)
			if data.TestPlan != "" {
				fmt.Printf("Test plan:  %s\n", data.TestPlan)
			}
			if data.TestRefs != "" {
				fmt.Printf("Test refs:  %s\n", data.TestRefs)
			}
			switch data.IssueType {
			case "ops":
				fmt.Println("\nReview checklist (ops):")
				fmt.Println("  - Demo recording artifacts reviewed (playwright code, stdout/stderr, browser logs)")
				fmt.Println("  - Test plan coverage verified")
			case "ask":
				fmt.Println("\nReview checklist (ask):")
				fmt.Println("  - Investigation finding recorded in issue body or task notes")
				fmt.Println("  - Any follow-up issues proposed/filed if investigation surfaced concerns")
			default:
				fmt.Println("\nReview checklist (impl):")
				fmt.Println("  - Fix SHAs and diffs reviewed")
				fmt.Println("  - Test plan and code blocks verified")
			}
			fmt.Printf("\nSubmit review: dx todo dev review %s --verdict=<approve|reject> [--body=\"...\"]\n", id)
			fmt.Printf("Discuss the review: dx comment add review <review-id> --body=\"...\"\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&verdict, "verdict", "", "review verdict: approve or reject")
	cmd.Flags().StringVar(&body, "body", "", "review note persisted on the review record (NOT a task comment)")
	return cmd
}

// ── owner ─────────────────────────────────────────────────────────────────────

func todoOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "owner", Short: "Owner actions"}
	cmd.AddCommand(todoOwnerTriageCmd())
	return cmd
}

func todoOwnerTriageCmd() *cobra.Command {
	var priority, title, issueType, context, questions string
	var focusArgs, goalArgs []string
	var clarify bool
	cmd := &cobra.Command{
		Use:   "triage <IS-N>",
		Short: "Set issue priority",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := cli.MustClient()
			slug := c.SlugOrDie()

			if clarify {
				if questions == "" {
					return fmt.Errorf("--clarify requires --questions")
				}
				// Parse questions: semicolon-separated, each optionally containing pipe-separated choices
				// Format: "question text|choice1,choice2;another question"
				parts := strings.Split(questions, ";")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if part == "" {
						continue
					}
					qCtx := part
					var choiceList []string
					if idx := strings.Index(part, "|"); idx >= 0 {
						qCtx = strings.TrimSpace(part[:idx])
						for _, ch := range strings.Split(part[idx+1:], ",") {
							ch = strings.TrimSpace(ch)
							if ch != "" {
								choiceList = append(choiceList, ch)
							}
						}
					}
					resp, err := c.AddBlockerQuestionWithResponse(ctx, dxclient.AddBlockerQuestionRequest{
						Slug:       slug,
						TargetType: "issue",
						TargetId:   id,
						Context:    qCtx,
						Choices:    &choiceList,
					})
					if err != nil {
						return err
					}
					if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
						return err
					}
					if resp.JSON200 == nil {
						return fmt.Errorf("empty add-blocker-question response")
					}
					fmt.Printf("BQ-%d  [issue:%s]  %s\n", resp.JSON200.Id, id, qCtx)
				}
				if title != "" || context != "" {
					body := dxclient.TriageIssueRequest{
						Slug:     slug,
						Id:       int32(n),
						Priority: 0,
					}
					if title != "" {
						body.Title = &title
					}
					if context != "" {
						body.Context = &context
					}
					_, _ = c.TriageIssueWithResponse(ctx, body)
				}
				fmt.Printf("%s needs clarification — solo will block until questions are answered\n", id)
				return nil
			}

			if priority == "" {
				return fmt.Errorf("--priority is required (use --clarify for vague issues)")
			}
			pri, _ := strconv.ParseInt(priority, 10, 32)
			body := dxclient.TriageIssueRequest{
				Slug:     slug,
				Id:       int32(n),
				Priority: int32(pri),
			}
			if title != "" {
				body.Title = &title
			}
			if issueType != "" {
				body.IssueType = &issueType
			}
			if context != "" {
				body.Context = &context
			}
			focuses, err := parseTodoIDs(focusArgs, "FO-")
			if err != nil {
				return fmt.Errorf("--focus: %w", err)
			}
			goals, err := parseTodoIDs(goalArgs, "G-")
			if err != nil {
				return fmt.Errorf("--goal: %w", err)
			}
			if len(focuses) > 0 {
				body.ThemeIds = &focuses // wire name still theme_ids for compat with generated client
			}
			if len(goals) > 0 {
				body.GoalIds = &goals
			}
			resp, err := c.TriageIssueWithResponse(ctx, body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("%s triaged (priority=%s)\n", id, priority)
			return nil
		},
	}
	cmd.Flags().StringVar(&priority, "priority", "", "priority 1-4 (1=highest)")
	cmd.Flags().StringVar(&title, "title", "", "set issue title")
	cmd.Flags().StringVar(&issueType, "type", "", "issue type: ops, impl, ask, or tracker")
	cmd.Flags().StringVar(&context, "context", "", "rewrite issue context (answer embedded question, clarify scope)")
	cmd.Flags().BoolVar(&clarify, "clarify", false, "create clarification questions instead of triaging (requires --questions)")
	cmd.Flags().StringVar(&questions, "questions", "", "semicolon-separated questions; use | for choices: \"question|a,b,c;question2\"")
	cmd.Flags().StringSliceVar(&focusArgs, "focus", nil, "focus ID(s) to link — FO-N or N (repeatable)")
	cmd.Flags().StringSliceVar(&goalArgs, "goal", nil, "goal ID(s) to link — G-N or N (repeatable)")
	return cmd
}

// parseTodoIDs converts loose ID forms (e.g. "TH-6" or "6") to int32 IDs.
// Prefix match is case-insensitive.
func parseTodoIDs(args []string, prefix string) ([]int32, error) {
	if len(args) == 0 {
		return nil, nil
	}
	out := make([]int32, 0, len(args))
	for _, a := range args {
		s := strings.TrimSpace(a)
		if s == "" {
			continue
		}
		if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			s = s[len(prefix):]
		}
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %q (expected %sN or N)", a, prefix)
		}
		out = append(out, int32(n))
	}
	return out, nil
}

// ── tech ──────────────────────────────────────────────────────────────────────

func todoTechCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tech", Short: "Tech lead actions"}
	cmd.AddCommand(todoTechAddCmd())
	return cmd
}

func todoTechAddCmd() *cobra.Command {
	var issue, feature, title, text, reason, testPlan, taskGroup string
	var autoReady, force bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a task",
		Long: `Add a task with structured fields.

--title       outcome-oriented headline (the WHAT); shown in list views.
--text        implementation plan (the HOW); concrete steps and file paths.
--reason      motivation (the WHY); optional but encouraged.
--test-plan   how the work will be verified; optional but required to close.`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected positional argument %q; use --issue to link to an issue", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			c := cli.MustClient()
			body := dxclient.AddTaskRequest{
				Slug: c.SlugOrDie(),
				Text: text,
			}
			if title != "" {
				body.Title = &title
			}
			if reason != "" {
				body.Reason = &reason
			}
			if testPlan != "" {
				body.TestPlan = &testPlan
			}
			if feature != "" {
				body.Feature = &feature
			}
			if issue != "" {
				body.Issue = &issue
			}
			if taskGroup != "" {
				body.TaskGroup = &taskGroup
			}
			if autoReady {
				body.AutoReady = &autoReady
			}
			if force {
				body.Force = &force
			}
			resp, err := c.AddTaskWithResponse(ctx, body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty add-task response")
			}
			r := resp.JSON200
			if r.DuplicateBlocked != nil && *r.DuplicateBlocked {
				fmt.Println("Blocked: near-duplicate tasks found on the same issue (>85% similarity):")
				if r.Similar != nil {
					for _, s := range *r.Similar {
						fmt.Printf("  %s  (%.0f%%)  %s  [%s]\n", s.Id, s.Score*100, s.Text, s.Status)
					}
				}
				fmt.Println("\nTo create anyway, re-run with --force")
				return fmt.Errorf("duplicate blocked")
			}
			headline := r.Title
			if headline == "" {
				headline = r.Text
			}
			fmt.Printf("%s  %s\n", clitypes.TaskIDStr(r.Id), headline)
			hasSimilar := r.Similar != nil && len(*r.Similar) > 0
			if !autoReady && hasSimilar {
				fmt.Println("\nSimilar tasks:")
				for _, s := range *r.Similar {
					fmt.Printf("  %s  (%.0f%%)  %s  [%s]\n", s.Id, s.Score*100, s.Text, s.Status)
				}
				fmt.Printf("\nTask created as draft (wip). To promote:\n")
				fmt.Printf("  dx task ready %s\n", clitypes.TaskIDStr(r.Id))
				fmt.Printf("To delete:\n")
				fmt.Printf("  dx task delete %s\n", clitypes.TaskIDStr(r.Id))
			} else if !autoReady {
				readyResp, err := c.ReadyTaskWithResponse(ctx, dxclient.ReadyTaskRequest{
					Id: r.Id,
				})
				if err != nil {
					return err
				}
				if err := c.CheckStatus(readyResp.StatusCode(), readyResp.Body); err != nil {
					return err
				}
				fmt.Println("(auto-promoted to ready — no similar tasks found)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "link to issue (IS-N)")
	cmd.Flags().StringVar(&feature, "feature", "", "link to feature name")
	cmd.Flags().StringVar(&title, "title", "", "outcome-oriented headline (what)")
	cmd.Flags().StringVar(&text, "text", "", "implementation plan (how)")
	cmd.Flags().StringVar(&reason, "reason", "", "motivation (why)")
	cmd.Flags().StringVar(&testPlan, "test-plan", "", "verification plan (required to close)")
	cmd.Flags().StringVar(&taskGroup, "task-group", "", "logical task group name")
	cmd.Flags().BoolVar(&autoReady, "auto-ready", false, "skip similarity check and create as ready")
	cmd.Flags().BoolVar(&force, "force", false, "bypass duplicate detection")
	cmd.MarkFlagRequired("text")
	return cmd
}

// ── printers ──────────────────────────────────────────────────────────────────

func printTaskItem(t clitypes.TaskItem) {
	fmt.Printf("ID:       %s\n", clitypes.TaskIDStr(t.ID))
	if t.Title != "" {
		fmt.Printf("Title:    %s\n", t.Title)
	}
	fmt.Printf("Status:   %s\n", t.Status)
	if t.IssueID != nil {
		fmt.Printf("Issue:    %s\n", clitypes.IssueIDStr(*t.IssueID))
	}
	if t.Feature != "" {
		fmt.Printf("Feature:  %s\n", t.Feature)
	}
	if t.TaskGroup != "" {
		fmt.Printf("Group:    %s\n", t.TaskGroup)
	}
	if t.Reason != "" {
		fmt.Printf("\nReason:\n%s\n", indent(t.Reason, "  "))
	}
	if t.Text != "" {
		label := "Implementation Plan"
		if t.Title == "" {
			label = "Text"
		}
		fmt.Printf("\n%s:\n%s\n", label, indent(t.Text, "  "))
	}
	if t.TestPlan != "" {
		fmt.Printf("\nTest Plan:\n%s\n", indent(t.TestPlan, "  "))
	}
	if t.TestRefs != "" {
		fmt.Printf("\nTest Refs:\n%s\n", indent(t.TestRefs, "  "))
	}
}

func indent(s, prefix string) string {
	if s == "" {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}

// journalOverdue returns true and the role name if either owner or tech journal
// is overdue. Cadence: max(7, 30 / max(1, closedTasks/10)) days between entries.
// More completed tasks → more frequent check-ins; minimum weekly, maximum monthly.
func journalOverdue(ownerDate, techDate string, closedTasks int64) (bool, string) {
	now := time.Now()
	cadenceDays := 30.0 / max(1.0, float64(closedTasks)/10.0)
	if cadenceDays < 7 {
		cadenceDays = 7
	}

	check := func(dateStr string) bool {
		if dateStr == "" {
			return true
		}
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return true
		}
		return now.Sub(t).Hours()/24 > cadenceDays
	}

	if check(ownerDate) {
		return true, "owner"
	}
	if check(techDate) {
		return true, "tech"
	}
	return false, ""
}

func printSimilarPatterns(cmd *cobra.Command, c *cli.Client, slug, text string) {
	n := int64(3)
	resp, err := c.SimilarPatternsWithResponse(cmd.Context(), dxclient.SimilarPatternsRequest{
		Slug: slug,
		Text: text,
		N:    &n,
	})
	if err != nil || resp.JSON200 == nil || resp.JSON200.Patterns == nil || len(*resp.JSON200.Patterns) == 0 {
		return
	}
	fmt.Println("\nRelevant patterns:")
	for _, r := range *resp.JSON200.Patterns {
		fmt.Printf("  %.3f  PT-%-4d  %s\n", r.Score, r.Pattern.Id, r.Pattern.Name)
		if r.Pattern.Description != "" {
			fmt.Printf("         %s\n", cli.Truncate(r.Pattern.Description, 80))
		}
	}
}

// RunTodo kept for compatibility.
func RunTodo(_ []string) {}

// ── reservations ─────────────────────────────────────────────────────────────

// ── release ───────────────────────────────────────────────────────────────────

func todoReleaseCmd() *cobra.Command {
	var forceFlag bool
	cmd := &cobra.Command{
		Use:   "release <TODO-N>",
		Short: "Release a claimed todo reservation (admin, no agent-id required)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return todoReleaseRun(cmd, args[0], forceFlag)
		},
	}
	cmd.Flags().BoolVar(&forceFlag, "force", false, "release even if the lease is still active")
	return cmd
}

func todoReleaseRun(cmd *cobra.Command, arg string, force bool) error {
	if len(arg) < 6 || arg[:5] != "TODO-" {
		return fmt.Errorf("expected TODO-N, got %q", arg)
	}
	id, err := strconv.ParseInt(arg[5:], 10, 32)
	if err != nil {
		return fmt.Errorf("invalid todo id %q: %w", arg, err)
	}
	c := cli.MustClient()
	slug := c.SlugOrDie()

	claimsResp, err := c.SoloListClaimsWithResponse(cmd.Context(), &dxclient.SoloListClaimsParams{Slug: slug})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(claimsResp.StatusCode(), claimsResp.Body); err != nil {
		return err
	}
	if claimsResp.JSON200 != nil && claimsResp.JSON200.Todos != nil {
		for _, t := range *claimsResp.JSON200.Todos {
			if int64(t.Id) == id {
				if !force {
					claimedBy := ""
					if t.ClaimedBy != nil {
						claimedBy = *t.ClaimedBy
					}
					claimedAt := ""
					if t.ClaimedAt != nil {
						claimedAt = *t.ClaimedAt
					}
					return fmt.Errorf("TODO-%d has an active lease (claimed-by=%s, claimed-at=%s); use --force to release anyway",
						t.Id, claimedBy, claimedAt)
				}
				break
			}
		}
	}

	releaseResp, err := c.SoloReleaseWithResponse(cmd.Context(), dxclient.SoloReleaseRequest{Id: int32(id), AgentId: ""})
	if err != nil {
		return err
	}
	if err := c.CheckStatus(releaseResp.StatusCode(), releaseResp.Body); err != nil {
		return err
	}
	fmt.Printf("%s released\n", arg)
	return nil
}

func todoReservationsCmd() *cobra.Command {
	var limitFlag int32
	cmd := &cobra.Command{
		Use:   "reservations",
		Short: "List reservation history (claimed_at, released_at) for todos and tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return todoReservationsRun(cmd, limitFlag)
		},
	}
	cmd.Flags().Int32Var(&limitFlag, "limit", 50, "max number of reservations to show")
	return cmd
}

func todoReservationsRun(cmd *cobra.Command, limit int32) error {
	c := cli.MustClient()
	slug := c.SlugOrDie()

	resp, err := c.SoloListReservationsWithResponse(cmd.Context(), &dxclient.SoloListReservationsParams{
		Slug:  slug,
		Limit: &limit,
	})
	if err != nil {
		return err
	}
	if resp.JSON200 == nil || resp.JSON200.Reservations == nil || len(*resp.JSON200.Reservations) == 0 {
		fmt.Println("no reservations")
		return nil
	}

	for _, r := range *resp.JSON200.Reservations {
		status := "active"
		if r.ReleasedAt != "" {
			status = "released"
		}
		fmt.Printf("  %-8s  %-5s  %-12s  claimed=%-26s  released=%-26s  %s\n",
			r.TargetType, r.TargetId, r.ClaimedBy, r.ClaimedAt, r.ReleasedAt, status)
	}
	return nil
}
