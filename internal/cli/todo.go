package cli

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type stringOrStrings []string

func (s *stringOrStrings) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '[' {
		var arr []string
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		*s = arr
		return nil
	}
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	if str == "" {
		*s = nil
	} else {
		*s = []string{str}
	}
	return nil
}

const triageGuidance = `  triage checklist:
    1. verify independently (reproduce or read the code)
    2. dup-check: dx issue list; close duplicates with --reason=duplicate
    3. rewrite prescriptively: title=intended outcome; context=should/did/direction
    4. apply: dx todo owner triage IS-N --title=... --context=... --type=<ops|impl|tracker> --priority=<1-4> --theme=<ID> --goal=<ID>
       use the active themes and goals listed above to classify the issue
    if the issue is too vague to triage, create clarification questions instead:
      dx question add --target-type=issue --target-id=IS-N --context="<question>" --choices="opt1,opt2,..."
    solo will block progress on the issue until all questions are answered.
`

func printTriageContext(c *Client, slug string) {
	var themeResp struct {
		Themes []themeItem `json:"themes"`
	}
	if err := c.get("/api/dx/themes", url.Values{"slug": {slug}}, &themeResp); err == nil && len(themeResp.Themes) > 0 {
		fmt.Println("  active themes:")
		for _, t := range themeResp.Themes {
			if t.Status == "active" {
				fmt.Printf("    TH-%-3d  %s\n", t.ID, t.Name)
			}
		}
	}
	var goalResp struct {
		Goals []goalItem `json:"goals"`
	}
	if err := c.get("/api/goals", url.Values{"slug": {slug}}, &goalResp); err == nil && len(goalResp.Goals) > 0 {
		fmt.Println("  active goals:")
		for _, g := range goalResp.Goals {
			if g.Status == "active" {
				fmt.Printf("    G-%-3d   %s\n", g.ID, g.Title)
			}
		}
	}
}

// ── wire types (match server JSON) ────────────────────────────────────────────

type issueItem struct {
	ID        int32           `json:"id"`
	Title     string          `json:"title"`
	Status    string          `json:"status"`
	Priority  string          `json:"priority"`
	Component string          `json:"component"`
	BlockedBy stringOrStrings `json:"blocked_by"`
	Context   string          `json:"context"`
	IssueType string          `json:"issue_type"`
	URL       string          `json:"url"`
}

type issueWorkItem struct {
	Agent     string `json:"agent"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type similarIssueItem struct {
	ID      string  `json:"id"`
	Title   string  `json:"title"`
	Context string  `json:"context"`
	Status  string  `json:"status"`
	Score   float32 `json:"score"`
}

type similarTaskItem struct {
	ID     string  `json:"id"`
	Text   string  `json:"text"`
	Status string  `json:"status"`
	Issue  string  `json:"issue"`
	Score  float32 `json:"score"`
}

type issueAddResponse struct {
	issueItem
	Similar []similarIssueItem `json:"similar,omitempty"`
}

type taskAddResponse struct {
	taskItem
	Similar          []similarTaskItem `json:"similar,omitempty"`
	DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
}

type taskItem struct {
	ID         int32  `json:"id"`
	Text       string `json:"text"`
	Feature    string `json:"feature"`
	Status     string `json:"status"`
	Reason     string `json:"reason"`
	IssueID    *int32 `json:"issue_id,omitempty"`
	TaskGroup  string `json:"task_group"`
	CreatedAt  string `json:"created_at"`
	StaleSince string `json:"stale_since,omitempty"`
}

type featureItem struct {
	ID          int32      `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	What        string     `json:"what"`
	Why         string     `json:"why"`
	DoneWhen    string     `json:"done_when"`
	Component   string     `json:"component"`
	Category    string     `json:"category"`
	PlanType    string     `json:"plan_type"`
	Specs       []specItem `json:"specs"`
}

type specItem struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Deferred    bool   `json:"deferred"`
}

func issueIDStr(n int32) string { return fmt.Sprintf("IS-%d", n) }
func taskIDStr(n int32) string  { return fmt.Sprintf("TK-%d", n) }

// ── TodoCmd ───────────────────────────────────────────────────────────────────

func TodoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Workflow queue",
		RunE:  func(cmd *cobra.Command, args []string) error { return soloRun(cmd, args) },
	}
	cmd.AddCommand(todoSoloCmd(), todoListCmd(), todoShowCmd(), todoDevCmd(), todoOwnerCmd(), todoTechCmd())
	return cmd
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
	issueFlag, _ := cmd.Flags().GetString("issue")
	agentID, _ := cmd.Flags().GetString("agent-id")
	c := mustClient()
	slug := c.SlugOrDie()

	// When agent-id is set, validate the agent and extract task_group.
	var agentTaskGroup string
	var heartbeatStop chan struct{}
	if agentID != "" {
		var agent agentItem
		if err := c.get("/api/agents/"+agentID, nil, &agent); err != nil {
			return fmt.Errorf("agent %s not found: %w", agentID, err)
		}
		agentTaskGroup = agent.TaskGroup
		heartbeatStop = make(chan struct{})
		go heartbeatLoop(c, agentID, 60*time.Second, heartbeatStop)
		defer close(heartbeatStop)
	}

	// Fetch issues
	var issueList struct {
		Issues []issueItem `json:"issues"`
	}
	if err := c.get("/api/dx/todo/issue/list", url.Values{"slug": {slug}}, &issueList); err != nil {
		return err
	}

	// If --issue given, restrict to that one
	var targetIssues []issueItem
	if issueFlag != "" {
		for _, iss := range issueList.Issues {
			if issueIDStr(iss.ID) == issueFlag {
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
		targetIssues = issueList.Issues
	}

	// Exclude tracker issues — they are closed by their children, never actionable directly.
	{
		var filtered []issueItem
		for _, iss := range targetIssues {
			if iss.IssueType != "tracker" {
				filtered = append(filtered, iss)
			}
		}
		targetIssues = filtered
	}

	// 0. Check for unread LLM comments on any open issue.
	for _, iss := range targetIssues {
		if iss.Status != "open" {
			continue
		}
		var unreadResp struct {
			HasUnread bool `json:"has_unread"`
		}
		if err := c.get("/api/dx/comment/unread-check", url.Values{
			"slug":        {slug},
			"target_type": {"issue"},
			"target_id":   {issueIDStr(iss.ID)},
			"role":        {"llm"},
		}, &unreadResp); err != nil {
			return err
		}
		if unreadResp.HasUnread {
			fmt.Printf("[read:comments] %s  %s\n", issueIDStr(iss.ID), iss.Title)
			// Show comments inline with unread indicators.
			var commResp struct {
				Comments []commentItem `json:"comments"`
			}
			if err := c.get("/api/dx/comment/list", url.Values{
				"slug":        {slug},
				"target_type": {"issue"},
				"target_id":   {issueIDStr(iss.ID)},
				"role":        {"llm"},
			}, &commResp); err != nil {
				return err
			}
			fmt.Println()
			printComments(commResp.Comments)
			// Mark read so next solo run advances.
			var ok struct {
				OK bool `json:"ok"`
			}
			_ = c.post("/api/dx/comment/mark-read", map[string]any{
				"slug":        slug,
				"target_type": "issue",
				"target_id":   issueIDStr(iss.ID),
				"role":        "llm",
			}, &ok)
			return nil
		}
	}

	// 0b. Check for unread LLM comments on any feature.
	var featList struct {
		Features []featureItem `json:"features"`
	}
	if err := c.get("/api/features", querySlug(c), &featList); err != nil {
		return err
	}
	for _, f := range featList.Features {
		var unreadResp struct {
			HasUnread bool `json:"has_unread"`
		}
		if err := c.get("/api/dx/comment/unread-check", url.Values{
			"slug":        {slug},
			"target_type": {"feature"},
			"target_id":   {f.Name},
			"role":        {"llm"},
		}, &unreadResp); err != nil {
			return err
		}
		if unreadResp.HasUnread {
			fmt.Printf("[read:comments] feature %q\n", f.Name)
			var commResp struct {
				Comments []commentItem `json:"comments"`
			}
			if err := c.get("/api/dx/comment/list", url.Values{
				"slug":        {slug},
				"target_type": {"feature"},
				"target_id":   {f.Name},
				"role":        {"llm"},
			}, &commResp); err != nil {
				return err
			}
			fmt.Println()
			printComments(commResp.Comments)
			var ok struct {
				OK bool `json:"ok"`
			}
			_ = c.post("/api/dx/comment/mark-read", map[string]any{
				"slug":        slug,
				"target_type": "feature",
				"target_id":   f.Name,
				"role":        "llm",
			}, &ok)
			return nil
		}
	}

	// 0b2. Check for unanswered QA questions (oldest first).
	{
		var unansweredResp struct {
			Questions []questionItem `json:"questions"`
		}
		if err := c.get("/api/dx/qa/unanswered", url.Values{"slug": {slug}}, &unansweredResp); err != nil {
			return err
		}
		if len(unansweredResp.Questions) > 0 {
			q := unansweredResp.Questions[0]
			fmt.Printf("[answer]  QA-%d  %s\n", q.ID, q.Question)
			if q.Category != "" {
				fmt.Printf("  category: %s\n", q.Category)
			}
			fmt.Printf("  asked: %s\n", q.CreatedAt)
			fmt.Printf("  answer: dx qa answer %d --answer=\"...\"\n", q.ID)
			return nil
		}
	}

	// 0b3. Check for stale unread comments (>24h old, unread for LLM role).
	{
		var staleResp struct {
			Comments []commentItem `json:"comments"`
		}
		if err := c.get("/api/dx/comment/stale-unread", url.Values{
			"slug":      {slug},
			"role":      {"llm"},
			"age_hours": {"24"},
		}, &staleResp); err != nil {
			return err
		}
		if len(staleResp.Comments) > 0 {
			sc := staleResp.Comments[0]
			fmt.Printf("[respond:stale] %s %s\n", sc.TargetType, sc.TargetID)
			fmt.Printf("  from: %s  at: %s\n", sc.Author, sc.CreatedAt)
			fmt.Printf("  %s\n", sc.Body)
			return nil
		}
	}

	// 0c. Check for pending blocker-questions on open issues.
	bqBlockedIssues := map[string]bool{}
	{
		var bqResp struct {
			Questions []blockerQuestionItem `json:"questions"`
			Total     int                   `json:"total"`
		}
		bqParams := url.Values{"slug": {slug}, "status": {"pending"}}
		if err := c.get("/api/dx/blocker-questions/list", bqParams, &bqResp); err != nil {
			return err
		}
		for _, q := range bqResp.Questions {
			if q.TargetType != "issue" {
				continue
			}
			matched := false
			for _, iss := range targetIssues {
				if issueIDStr(iss.ID) == q.TargetID && iss.Status == "open" {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
			if issueFlag != "" {
				// Scoped to a single issue — block until answered.
				fmt.Printf("[clarify] %s  BQ-%d\n", q.TargetID, q.ID)
				fmt.Printf("  %s\n", q.Context)
				if len(q.Choices) > 0 {
					for i, ch := range q.Choices {
						fmt.Printf("    %d. %s\n", i+1, ch)
					}
				}
				fmt.Printf("  answer: dx question answer %d --answer=\"...\"\n", q.ID)
				return nil
			}
			// Global mode — skip this issue, pick other work.
			bqBlockedIssues[q.TargetID] = true
		}
	}
	// Filter out BQ-blocked issues in global mode.
	if issueFlag == "" && len(bqBlockedIssues) > 0 {
		filtered := targetIssues[:0]
		for _, iss := range targetIssues {
			if !bqBlockedIssues[issueIDStr(iss.ID)] {
				filtered = append(filtered, iss)
			}
		}
		targetIssues = filtered
	}

	// 0c1. Check for stale tasks (flagged by recovery sweep, never claimed).
	{
		staleParams := url.Values{"slug": {slug}}
		if issueFlag != "" {
			staleParams.Set("issue", issueFlag)
		}
		var staleResp struct {
			Tasks []taskItem `json:"tasks"`
		}
		if err := c.get("/api/dx/tasks/stale", staleParams, &staleResp); err == nil && len(staleResp.Tasks) > 0 {
			t := staleResp.Tasks[0]
			fmt.Printf("[review:stale] %s  %s\n", taskIDStr(t.ID), t.Text)
			if t.IssueID != nil {
				fmt.Printf("  issue: %s\n", issueIDStr(*t.IssueID))
			}
			fmt.Println("  ⚠ This task was flagged stale — it was created but never claimed.")
			fmt.Println("  Before editing, verify the work is still needed by reading the current code.")
			fmt.Println("  If already implemented: dx todo dev done " + taskIDStr(t.ID))
			fmt.Println("  If superseded: dx todo dev done " + taskIDStr(t.ID))
			return nil
		}
	}

	// 0d. Bootstrap: if no issues exist at all, guide agent to analyze the project.
	if issueFlag == "" && len(issueList.Issues) == 0 && len(featList.Features) == 0 {
		fmt.Printf("[bootstrap] %s\n", slug)
		fmt.Println(`
No issues or features exist yet — this looks like a new project.

Analyze the project to bootstrap its feature catalog and first issue:

1. Scan the codebase for conceptual features:
   - Entry points (main packages, server routes, CLI commands)
   - Data model (migrations, schema files, ORM models)
   - External integrations (APIs, queues, caches, auth providers)
   - UI structure (pages, components, layouts)
   - Build / deploy (Dockerfile, CI config, Makefile)

2. For each discovered feature, create it:
   dx feature add <name> --desc="<what it does>"

3. Create an initial setup issue for zdx integration:
   dx issue add --title="Integrate project with zdx tooling" \
     --context="Set up .zdx/config.yaml close-hooks (lint, gen), configure components, and verify dx todo solo cycle works end-to-end."

4. Run dx todo solo again to start the normal triage flow.`)
		return nil
	}

	// 0e. Check project health: goals, constraints, journal cadence.
	// Only in global mode — these are cross-cutting owner/tech concerns.
	if issueFlag == "" {
		var health struct {
			GoalCount        int64  `json:"goal_count"`
			ConstraintCount  int64  `json:"constraint_count"`
			OwnerJournalDate string `json:"owner_journal_date"`
			TechJournalDate  string `json:"tech_journal_date"`
			ClosedTaskCount  int64  `json:"closed_task_count"`
		}
		if err := c.get("/api/dx/solo/health", url.Values{"slug": {slug}}, &health); err == nil {
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
			fmt.Printf("[triage] %s  %s\n", issueIDStr(iss.ID), iss.Title)
			if iss.IssueType != "" {
				fmt.Printf("  type:      %s\n", iss.IssueType)
			}
			if iss.Component != "" {
				fmt.Printf("  component: %s\n", iss.Component)
			}
			if iss.Context != "" {
				fmt.Printf("\n%s\n", iss.Context)
			}
			printTriageContext(c, slug)
			fmt.Print(triageGuidance)
			return nil
		}
	}

	// Cross-cutting owner/tech checks (1b, 1c, 2a) scan all features/specs globally;
	// skip them when --issue is set so vertical scope is preserved.
	if issueFlag == "" {
		// 1b. Check for ANY feature with no specs (reuse featList from 0b).
		for _, f := range featList.Features {
			if len(f.Specs) == 0 {
				fmt.Printf("[owner:spec]  feature %q has no specs — dx feature spec add %q\n", f.Name, f.Name)
				return nil
			}
		}

		// 1c. Check for features due for periodic owner re-review (>30 days stale).
		var staleResp struct {
			Features []featureItem `json:"features"`
		}
		if err := c.get("/api/dx/features/stale", querySlug(c), &staleResp); err != nil {
			return err
		}
		if len(staleResp.Features) > 0 {
			f := staleResp.Features[0]
			fmt.Printf("[owner:review]  feature %q not reviewed in >30 days — dx feature review %q\n", f.Name, f.Name)
			return nil
		}

		// 2a. Check for specs with no test_refs — tech lead owns this.
		var uncoveredResp struct {
			Specs []struct {
				ID          int32  `json:"id"`
				FeatureName string `json:"feature_name"`
				Description string `json:"description"`
				Kind        string `json:"kind"`
			} `json:"specs"`
		}
		if err := c.get("/api/dx/specs/uncovered", querySlug(c), &uncoveredResp); err != nil {
			return err
		}
		if len(uncoveredResp.Specs) > 0 {
			s := uncoveredResp.Specs[0]
			fmt.Printf("[tech:test-ref]  feature %q spec %d (%s) has no test refs — add task or link via dx spec link\n", s.FeatureName, s.ID, s.Description)
			return nil
		}

		// 2b. Check for specs linked to tests but missing demo artifacts.
		var demoGapResp struct {
			Specs []struct {
				ID          int32  `json:"id"`
				FeatureName string `json:"feature_name"`
				Description string `json:"description"`
				Kind        string `json:"kind"`
			} `json:"specs"`
		}
		if err := c.get("/api/dx/specs/demo-gap", querySlug(c), &demoGapResp); err != nil {
			return err
		}
		if len(demoGapResp.Specs) > 0 {
			s := demoGapResp.Specs[0]
			fmt.Printf("[owner:demo-gap]  feature %q spec %d (%s) has no demo recording — run dx test --layer demo\n", s.FeatureName, s.ID, s.Description)
			return nil
		}
	}

	// 2. Find open issue with no pending tasks
	for _, iss := range targetIssues {
		if iss.Status != "open" {
			continue
		}
		var taskList struct {
			Tasks []taskItem `json:"tasks"`
		}
		if err := c.get("/api/dx/todo/issue/tasks", url.Values{
			"slug":     {slug},
			"issue_id": {issueIDStr(iss.ID)},
		}, &taskList); err != nil {
			return err
		}
		hasPending := false
		for _, t := range taskList.Tasks {
			if t.Status == "pending" || t.Status == "in_progress" {
				hasPending = true
				break
			}
		}
		if !hasPending {
			if len(taskList.Tasks) > 0 {
				var unreviewedResp struct {
					Tasks []taskItem `json:"tasks"`
				}
				issueID := issueIDStr(iss.ID)
				if err := c.get("/api/dx/todo/dev/unreviewed", url.Values{
					"slug":  {slug},
					"issue": {issueID},
				}, &unreviewedResp); err == nil && len(unreviewedResp.Tasks) > 0 {
					t := unreviewedResp.Tasks[0]
					fmt.Printf("[review]  %s  %s\n", taskIDStr(t.ID), t.Text)
					fmt.Printf("  issue: %s\n", issueID)
					return nil
				}
				fmt.Printf("[closable] %s  %s\n", issueIDStr(iss.ID), iss.Title)
			} else {
				fmt.Printf("[add]     %s  %s\n", issueIDStr(iss.ID), iss.Title)
			}
			return nil
		}
	}

	// 3. Find a pending task
	if agentID != "" {
		// Agent mode: atomically claim a task via the ClaimTask API.
		claimBody := map[string]any{
			"slug":       slug,
			"agent_id":   agentID,
			"task_group": agentTaskGroup,
		}
		if issueFlag != "" {
			claimBody["issue"] = issueFlag
		}
		var claimed agentTaskItem
		if err := c.post("/api/tasks/claim", claimBody, &claimed); err == nil {
			printStaleWarning(claimed.CreatedAt, claimed.ID)
			fmt.Printf("[dev]     %s  %s\n", claimed.ID, claimed.Text)
			if claimed.Issue != "" {
				fmt.Printf("  issue: %s\n", claimed.Issue)
			}
			fmt.Printf("  claimed_by: %s\n", agentID)
			return nil
		}
	} else {
		for _, iss := range targetIssues {
			if iss.Status != "open" {
				continue
			}
			var taskList struct {
				Tasks []taskItem `json:"tasks"`
			}
			if err := c.get("/api/dx/todo/issue/tasks", url.Values{
				"slug":     {slug},
				"issue_id": {issueIDStr(iss.ID)},
			}, &taskList); err != nil {
				return err
			}
			for _, t := range taskList.Tasks {
				if t.Status == "pending" {
					printStaleWarning(t.CreatedAt, taskIDStr(t.ID))
					fmt.Printf("[dev]     %s  %s\n", taskIDStr(t.ID), t.Text)
					fmt.Printf("  issue: %s\n", issueIDStr(iss.ID))
					fmt.Fprintln(os.Stderr, "  note: task not claimed. Use --agent-id to claim before starting work.")
					return nil
				}
			}
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

type soloQueueItem struct {
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

type evaluateDiffResp struct {
	Added   []soloQueueItem `json:"added"`
	Removed []struct {
		Key  string `json:"key"`
		Text string `json:"text"`
		Kind string `json:"kind"`
	} `json:"removed"`
	Changed []struct {
		Before struct {
			Key      string `json:"key"`
			Text     string `json:"text"`
			Priority int32  `json:"priority"`
		} `json:"before"`
		After soloQueueItem `json:"after"`
	} `json:"changed"`
	Unchanged []soloQueueItem `json:"unchanged"`
}

func soloEvaluateRun(cmd *cobra.Command, showDiff bool, apply bool) error {
	issueFlag, _ := cmd.Flags().GetString("issue")
	c := mustClient()
	slug := c.SlugOrDie()

	var diff evaluateDiffResp
	if err := c.post("/api/dx/solo/evaluate", map[string]any{
		"slug":  slug,
		"issue": issueFlag,
	}, &diff); err != nil {
		return err
	}

	if len(diff.Added) == 0 && len(diff.Removed) == 0 && len(diff.Changed) == 0 {
		fmt.Printf("queue up to date (%d items)\n", len(diff.Unchanged))
		if !apply {
			return nil
		}
	}

	if len(diff.Added) > 0 {
		fmt.Printf("+ %d new:\n", len(diff.Added))
		for _, a := range diff.Added {
			fmt.Printf("  + [%s] %s  %s\n", a.Kind, a.TargetID, a.Text)
		}
	}
	if len(diff.Removed) > 0 {
		fmt.Printf("- %d removed:\n", len(diff.Removed))
		for _, r := range diff.Removed {
			fmt.Printf("  - [%s] %s\n", r.Kind, r.Text)
		}
	}
	if len(diff.Changed) > 0 {
		fmt.Printf("~ %d changed:\n", len(diff.Changed))
		for _, ch := range diff.Changed {
			fmt.Printf("  ~ [%s] %s\n", ch.After.Kind, ch.After.Text)
		}
	}

	if !apply {
		fmt.Println("\nrun with --apply to commit changes")
		return nil
	}

	// Collect all items (added + changed + unchanged) and apply
	var items []soloQueueItem
	items = append(items, diff.Added...)
	for _, ch := range diff.Changed {
		items = append(items, ch.After)
	}
	items = append(items, diff.Unchanged...)

	var ok struct {
		OK bool `json:"ok"`
	}
	if err := c.post("/api/dx/solo/apply", map[string]any{
		"slug":  slug,
		"items": items,
	}, &ok); err != nil {
		return err
	}
	fmt.Printf("applied: %d items in queue\n", len(items))
	return nil
}

// ── list ──────────────────────────────────────────────────────────────────────

func todoListCmd() *cobra.Command {
	var issue, feature string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			slug := c.SlugOrDie()

			if issue != "" {
				var taskList struct {
					Tasks []taskItem `json:"tasks"`
				}
				if err := c.get("/api/dx/todo/issue/tasks", url.Values{
					"slug":     {slug},
					"issue_id": {issue},
				}, &taskList); err != nil {
					return err
				}
				printTasks(taskList.Tasks)
				return nil
			}
			if feature != "" {
				var taskList struct {
					Tasks []taskItem `json:"tasks"`
				}
				if err := c.get("/api/tasks-by-feature", url.Values{
					"slug":    {slug},
					"feature": {feature},
				}, &taskList); err != nil {
					return err
				}
				printTasks(taskList.Tasks)
				return nil
			}
			var taskList struct {
				Tasks []taskItem `json:"tasks"`
			}
			if err := c.get("/api/tasks", url.Values{"slug": {slug}}, &taskList); err != nil {
				return err
			}
			printTasks(taskList.Tasks)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "filter by issue (IS-N)")
	cmd.Flags().StringVar(&feature, "feature", "", "filter by feature name")
	return cmd
}

func printTasks(tasks []taskItem) {
	if len(tasks) == 0 {
		fmt.Println("no tasks")
		return
	}
	for _, t := range tasks {
		fmt.Printf("%-8s %-12s %s\n", taskIDStr(t.ID), t.Status, t.Text)
	}
}

// ── show ──────────────────────────────────────────────────────────────────────

func todoShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <IS-N|TK-N|feature-name>",
		Short: "Show issue, task, or feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			c := mustClient()
			slug := c.SlugOrDie()

			switch {
			case len(id) > 3 && id[:3] == "IS-":
				var resp struct {
					Issue issueItem       `json:"issue"`
					Work  []issueWorkItem `json:"work"`
				}
				if err := c.get("/api/dx/todo/issue/show", url.Values{"slug": {slug}, "id": {id}}, &resp); err != nil {
					return err
				}
				printIssueItem(resp.Issue)
				if len(resp.Work) > 0 {
					fmt.Println("\nWork log:")
					for _, w := range resp.Work {
						date := w.CreatedAt
						if len(date) >= 10 {
							date = date[:10]
						}
						fmt.Printf("  [%s] %s: %s\n", date, w.Agent, w.Note)
					}
				}
				var commResp struct {
					Comments []commentItem `json:"comments"`
				}
				if err := c.get("/api/dx/comment/list", url.Values{
					"slug":        {slug},
					"target_type": {"issue"},
					"target_id":   {id},
					"role":        {"llm"},
				}, &commResp); err == nil && len(commResp.Comments) > 0 {
					fmt.Println("\nComments:")
					printComments(commResp.Comments)
				}
				printSimilarPatterns(c, slug, resp.Issue.Title+" "+resp.Issue.Context)
			case len(id) > 3 && id[:3] == "TK-":
				n, _ := strconv.ParseInt(id[3:], 10, 32)
				taskID := int32(n)
				var taskList struct {
					Tasks []taskItem `json:"tasks"`
				}
				if err := c.get("/api/tasks", url.Values{"slug": {slug}}, &taskList); err != nil {
					return err
				}
				for _, t := range taskList.Tasks {
					if t.ID == taskID {
						printTaskItem(t)
						var commResp struct {
							Comments []commentItem `json:"comments"`
						}
						if err := c.get("/api/dx/comment/list", url.Values{
							"slug":        {slug},
							"target_type": {"task"},
							"target_id":   {id},
							"role":        {"llm"},
						}, &commResp); err == nil && len(commResp.Comments) > 0 {
							fmt.Println("\nComments:")
							printComments(commResp.Comments)
						}
						var revResp struct {
							Revisions []revisionItem `json:"revisions"`
						}
						if err := c.get("/api/dx/revisions", url.Values{
							"slug":        {slug},
							"target_type": {"task"},
							"target_id":   {id},
						}, &revResp); err == nil && len(revResp.Revisions) > 0 {
							fmt.Println("\nRevisions:")
							for _, r := range revResp.Revisions {
								date := r.CreatedAt
								if len(date) >= 10 {
									date = date[:10]
								}
								fmt.Printf("  [%s] %s: %s → %s (%s)\n", date, r.Field, r.OldVal, r.NewVal, r.Agent)
							}
						}
						printSimilarPatterns(c, slug, t.Text)
						return nil
					}
				}
				return fmt.Errorf("task %s not found", id)
			default:
				var featList struct {
					Features []featureItem `json:"features"`
				}
				if err := c.get("/api/features", url.Values{"slug": {slug}}, &featList); err != nil {
					return err
				}
				for _, f := range featList.Features {
					if f.Name == id {
						printFeatureItem(f)
						var commResp struct {
							Comments []commentItem `json:"comments"`
						}
						if err := c.get("/api/dx/comment/list", url.Values{
							"slug":        {slug},
							"target_type": {"feature"},
							"target_id":   {f.Name},
							"role":        {"llm"},
						}, &commResp); err == nil && len(commResp.Comments) > 0 {
							fmt.Println("\nComments:")
							printComments(commResp.Comments)
						}
						var revResp struct {
							Revisions []revisionItem `json:"revisions"`
						}
						if err := c.get("/api/dx/revisions", url.Values{
							"slug":        {slug},
							"target_type": {"feature"},
							"target_id":   {f.Name},
						}, &revResp); err == nil && len(revResp.Revisions) > 0 {
							fmt.Println("\nRevisions:")
							for _, r := range revResp.Revisions {
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
	cmd := &cobra.Command{
		Use:   "done <TK-N>",
		Short: "Mark task done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCloseHooks(); err != nil {
				return err
			}
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/todo/dev/done", map[string]any{
				"id":        int32(n),
				"test_plan": testPlan,
				"test_refs": testRefs,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s done\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&testPlan, "test-plan", "", "test plan description")
	cmd.Flags().StringVar(&testRefs, "test-refs", "", "test references")
	return cmd
}

func todoDevUndoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "undone <TK-N>",
		Short: "Reopen task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/todo/dev/undone", map[string]any{"id": int32(n)}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s undone\n", id)
			return nil
		},
	}
}

func todoDevStartCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "start <TK-N>",
		Short: "Mark task in-progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.put("/api/task-status", map[string]any{
				"id":     int32(n),
				"status": "in_progress",
				"reason": reason,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s started\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "notes")
	return cmd
}

func todoDevReviewCmd() *cobra.Command {
	var verdict, body string
	cmd := &cobra.Command{
		Use:   "review <TK-N>",
		Short: "Review a completed task or submit review verdict",
		Long:  "Without --verdict: prints task details and review material.\nWith --verdict: marks the task as reviewed and posts a review comment.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			slug := c.SlugOrDie()

			if verdict != "" {
				var ok struct {
					OK bool `json:"ok"`
				}
				if err := c.post("/api/dx/todo/dev/review", map[string]any{
					"slug":    slug,
					"id":      int32(n),
					"verdict": verdict,
					"comment": body,
				}, &ok); err != nil {
					return err
				}
				fmt.Printf("%s reviewed [%s]\n", id, verdict)
				return nil
			}

			var resp struct {
				Task      taskItem `json:"task"`
				IssueType string   `json:"issue_type"`
				TestPlan  string   `json:"test_plan"`
				TestRefs  string   `json:"test_refs"`
			}
			if err := c.get("/api/dx/todo/dev/review-data", url.Values{
				"slug": {slug},
				"id":   {strconv.Itoa(int(n))},
			}, &resp); err != nil {
				return err
			}

			fmt.Printf("Task:       %s\n", id)
			fmt.Printf("Text:       %s\n", resp.Task.Text)
			fmt.Printf("Status:     %s\n", resp.Task.Status)
			fmt.Printf("Issue type: %s\n", resp.IssueType)
			if resp.TestPlan != "" {
				fmt.Printf("Test plan:  %s\n", resp.TestPlan)
			}
			if resp.TestRefs != "" {
				fmt.Printf("Test refs:  %s\n", resp.TestRefs)
			}
			if resp.IssueType == "ops" {
				fmt.Println("\nReview checklist (ops):")
				fmt.Println("  - Demo recording artifacts reviewed (playwright code, stdout/stderr, browser logs)")
				fmt.Println("  - Test plan coverage verified")
			} else {
				fmt.Println("\nReview checklist (impl):")
				fmt.Println("  - Fix SHAs and diffs reviewed")
				fmt.Println("  - Test plan and code blocks verified")
			}
			fmt.Printf("\nSubmit review: dx todo dev review %s --verdict=<approve|reject> --body=\"...\"\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&verdict, "verdict", "", "review verdict: approve or reject")
	cmd.Flags().StringVar(&body, "body", "", "review comment body")
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
	var themes, goals []int
	var clarify bool
	cmd := &cobra.Command{
		Use:   "triage <IS-N>",
		Short: "Set issue priority",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
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
					var q blockerQuestionItem
					if err := c.post("/api/dx/blocker-questions/add", map[string]any{
						"slug":        slug,
						"target_type": "issue",
						"target_id":   id,
						"context":     qCtx,
						"choices":     choiceList,
					}, &q); err != nil {
						return err
					}
					fmt.Printf("BQ-%d  [issue:%s]  %s\n", q.ID, id, qCtx)
				}
				if title != "" || context != "" {
					body := map[string]any{
						"slug":     slug,
						"id":       int32(n),
						"priority": int32(0),
					}
					if title != "" {
						body["title"] = title
					}
					if context != "" {
						body["context"] = context
					}
					var ok struct {
						OK bool `json:"ok"`
					}
					_ = c.post("/api/dx/todo/owner/triage", body, &ok)
				}
				fmt.Printf("%s needs clarification — solo will block until questions are answered\n", id)
				return nil
			}

			if priority == "" {
				return fmt.Errorf("--priority is required (use --clarify for vague issues)")
			}
			pri, _ := strconv.ParseInt(priority, 10, 32)
			body := map[string]any{
				"slug":     slug,
				"id":       int32(n),
				"priority": int32(pri),
			}
			if title != "" {
				body["title"] = title
			}
			if issueType != "" {
				body["issue_type"] = issueType
			}
			if context != "" {
				body["context"] = context
			}
			if len(themes) > 0 {
				body["theme_ids"] = themes
			}
			if len(goals) > 0 {
				body["goal_ids"] = goals
			}
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/todo/owner/triage", body, &ok); err != nil {
				return err
			}
			fmt.Printf("%s triaged (priority=%s)\n", id, priority)
			return nil
		},
	}
	cmd.Flags().StringVar(&priority, "priority", "", "priority 1-4 (1=highest)")
	cmd.Flags().StringVar(&title, "title", "", "set issue title")
	cmd.Flags().StringVar(&issueType, "type", "", "issue type: ops, impl, or tracker")
	cmd.Flags().StringVar(&context, "context", "", "rewrite issue context (answer embedded question, clarify scope)")
	cmd.Flags().BoolVar(&clarify, "clarify", false, "create clarification questions instead of triaging (requires --questions)")
	cmd.Flags().StringVar(&questions, "questions", "", "semicolon-separated questions; use | for choices: \"question|a,b,c;question2\"")
	cmd.Flags().IntSliceVar(&themes, "theme", nil, "theme ID(s) to link (repeatable)")
	cmd.Flags().IntSliceVar(&goals, "goal", nil, "goal ID(s) to link (repeatable)")
	return cmd
}

// ── tech ──────────────────────────────────────────────────────────────────────

func todoTechCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tech", Short: "Tech lead actions"}
	cmd.AddCommand(todoTechAddCmd())
	return cmd
}

func todoTechAddCmd() *cobra.Command {
	var issue, feature, text, taskGroup string
	var autoReady, force bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a task",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unexpected positional argument %q; use --issue to link to an issue", args[0])
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp taskAddResponse
			body := map[string]any{
				"slug": c.SlugOrDie(),
				"text": text,
			}
			if feature != "" {
				body["feature"] = feature
			}
			if issue != "" {
				body["issue"] = issue
			}
			if taskGroup != "" {
				body["task_group"] = taskGroup
			}
			if autoReady {
				body["auto_ready"] = true
			}
			if force {
				body["force"] = true
			}
			if err := c.post("/api/dx/todo/tech/add", body, &resp); err != nil {
				return err
			}
			if resp.DuplicateBlocked {
				fmt.Println("Blocked: near-duplicate tasks found on the same issue (>85% similarity):")
				for _, s := range resp.Similar {
					fmt.Printf("  %s  (%.0f%%)  %s  [%s]\n", s.ID, s.Score*100, s.Text, s.Status)
				}
				fmt.Println("\nTo create anyway, re-run with --force")
				return fmt.Errorf("duplicate blocked")
			}
			fmt.Printf("%s  %s\n", taskIDStr(resp.ID), resp.Text)
			if !autoReady && len(resp.Similar) > 0 {
				fmt.Println("\nSimilar tasks:")
				for _, s := range resp.Similar {
					fmt.Printf("  %s  (%.0f%%)  %s  [%s]\n", s.ID, s.Score*100, s.Text, s.Status)
				}
				fmt.Printf("\nTask created as draft (wip). To promote:\n")
				fmt.Printf("  dx task ready %s\n", taskIDStr(resp.ID))
				fmt.Printf("To delete:\n")
				fmt.Printf("  dx task delete %s\n", taskIDStr(resp.ID))
			} else if !autoReady {
				if err := c.post("/api/dx/todo/task/ready", map[string]any{
					"id": resp.ID,
				}, nil); err != nil {
					return err
				}
				fmt.Println("(auto-promoted to pending — no similar tasks found)")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "link to issue (IS-N)")
	cmd.Flags().StringVar(&feature, "feature", "", "link to feature name")
	cmd.Flags().StringVar(&text, "text", "", "task description")
	cmd.Flags().StringVar(&taskGroup, "task-group", "", "logical task group name")
	cmd.Flags().BoolVar(&autoReady, "auto-ready", false, "skip similarity check and create as pending")
	cmd.Flags().BoolVar(&force, "force", false, "bypass duplicate detection")
	cmd.MarkFlagRequired("text")
	return cmd
}

// ── printers ──────────────────────────────────────────────────────────────────

func printIssueItem(iss issueItem) {
	fmt.Printf("ID:        %s\n", issueIDStr(iss.ID))
	fmt.Printf("Title:     %s\n", iss.Title)
	fmt.Printf("Status:    %s\n", iss.Status)
	fmt.Printf("Priority:  %s\n", iss.Priority)
	issType := iss.IssueType
	if issType == "" {
		issType = "ops"
	}
	fmt.Printf("Type:      %s\n", issType)
	if iss.Component != "" {
		fmt.Printf("Component: %s\n", iss.Component)
	}
	if iss.URL != "" {
		fmt.Printf("URL:       %s\n", iss.URL)
	}
	if len(iss.BlockedBy) > 0 {
		fmt.Printf("Blocked:   %s\n", strings.Join(iss.BlockedBy, ", "))
	}
	if iss.Context != "" {
		fmt.Printf("\n%s\n", iss.Context)
	}
}

func printTaskItem(t taskItem) {
	fmt.Printf("ID:      %s\n", taskIDStr(t.ID))
	fmt.Printf("Text:    %s\n", t.Text)
	fmt.Printf("Status:  %s\n", t.Status)
	if t.IssueID != nil {
		fmt.Printf("Issue:   %s\n", issueIDStr(*t.IssueID))
	}
	if t.Feature != "" {
		fmt.Printf("Feature: %s\n", t.Feature)
	}
	if t.Reason != "" {
		fmt.Printf("Reason:  %s\n", t.Reason)
	}
	if t.TaskGroup != "" {
		fmt.Printf("Group:   %s\n", t.TaskGroup)
	}
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

func printSimilarPatterns(c *Client, slug, text string) {
	var resp struct {
		Patterns []struct {
			Pattern patternItem `json:"pattern"`
			Score   float64     `json:"score"`
		} `json:"patterns"`
	}
	if err := c.post("/api/dx/patterns/similar", map[string]any{
		"slug": slug,
		"text": text,
		"n":    3,
	}, &resp); err != nil || len(resp.Patterns) == 0 {
		return
	}
	fmt.Println("\nRelevant patterns:")
	for _, r := range resp.Patterns {
		fmt.Printf("  %.3f  PT-%-4d  %s\n", r.Score, r.Pattern.ID, r.Pattern.Name)
		if r.Pattern.Description != "" {
			fmt.Printf("         %s\n", truncate(r.Pattern.Description, 80))
		}
	}
}

// RunTodo kept for compatibility.
func RunTodo(_ []string) {}
