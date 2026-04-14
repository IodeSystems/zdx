package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

const triageGuidance = `  triage checklist:
    1. verify independently (reproduce or read the code)
    2. dup-check: dx issue list; close duplicates with --reason=duplicate
    3. rewrite prescriptively: title=intended outcome; context=should/did/direction
    4. apply: dx todo owner triage IS-N --title=... --context=... --type=<ops|impl> --priority=<1-4>
`

// ── wire types (match server JSON) ────────────────────────────────────────────

type issueItem struct {
	ID        int32  `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Component string `json:"component"`
	BlockedBy string `json:"blocked_by"`
	Context   string `json:"context"`
	IssueType string `json:"issue_type"`
}

type issueWorkItem struct {
	Agent     string `json:"agent"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type taskItem struct {
	ID      int32  `json:"id"`
	Text    string `json:"text"`
	Feature string `json:"feature"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	IssueID *int32 `json:"issue_id,omitempty"`
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
	var issueFlag string
	cmd := &cobra.Command{
		Use:   "solo",
		Short: "Next actionable item",
		RunE:  func(cmd *cobra.Command, args []string) error { return soloRun(cmd, args) },
	}
	cmd.Flags().StringVar(&issueFlag, "issue", "", "filter to specific issue (IS-N)")
	return cmd
}

func soloRun(cmd *cobra.Command, _ []string) error {
	issueFlag, _ := cmd.Flags().GetString("issue")
	c := mustClient()
	slug := c.SlugOrDie()

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
			var ok struct{ OK bool `json:"ok"` }
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
			var ok struct{ OK bool `json:"ok"` }
			_ = c.post("/api/dx/comment/mark-read", map[string]any{
				"slug":        slug,
				"target_type": "feature",
				"target_id":   f.Name,
				"role":        "llm",
			}, &ok)
			return nil
		}
	}

	// 1. Find untriaged open issue (no priority)
	for _, iss := range targetIssues {
		if iss.Status == "open" && iss.Priority == "" {
			fmt.Printf("[triage] %s  %s\n", issueIDStr(iss.ID), iss.Title)
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
			fmt.Printf("[add]     %s  %s\n", issueIDStr(iss.ID), iss.Title)
			return nil
		}
	}

	// 3. Find a pending task
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
				fmt.Printf("[dev]     %s  %s\n", taskIDStr(t.ID), t.Text)
				fmt.Printf("  issue: %s\n", issueIDStr(iss.ID))
				return nil
			}
		}
	}

	fmt.Println("nothing to do")
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
	cmd.AddCommand(todoDevDoneCmd(), todoDevUndoneCmd(), todoDevStartCmd())
	return cmd
}

func todoDevDoneCmd() *cobra.Command {
	var testPlan, testRefs string
	cmd := &cobra.Command{
		Use:   "done <TK-N>",
		Short: "Mark task done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct{ OK bool `json:"ok"` }
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
			var ok struct{ OK bool `json:"ok"` }
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
			var ok struct{ OK bool `json:"ok"` }
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

// ── owner ─────────────────────────────────────────────────────────────────────

func todoOwnerCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "owner", Short: "Owner actions"}
	cmd.AddCommand(todoOwnerTriageCmd())
	return cmd
}

func todoOwnerTriageCmd() *cobra.Command {
	var priority, title, issueType, context string
	cmd := &cobra.Command{
		Use:   "triage <IS-N>",
		Short: "Set issue priority",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			pri, _ := strconv.ParseInt(priority, 10, 32)
			c := mustClient()
			body := map[string]any{
				"slug":     c.SlugOrDie(),
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
			var ok struct{ OK bool `json:"ok"` }
			if err := c.post("/api/dx/todo/owner/triage", body, &ok); err != nil {
				return err
			}
			fmt.Printf("%s triaged (priority=%s)\n", id, priority)
			return nil
		},
	}
	cmd.Flags().StringVar(&priority, "priority", "", "priority 1-4 (1=highest)")
	cmd.Flags().StringVar(&title, "title", "", "set issue title")
	cmd.Flags().StringVar(&issueType, "type", "", "issue type: ops or impl")
	cmd.Flags().StringVar(&context, "context", "", "rewrite issue context (answer embedded question, clarify scope)")
	cmd.MarkFlagRequired("priority")
	return cmd
}

// ── tech ──────────────────────────────────────────────────────────────────────

func todoTechCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tech", Short: "Tech lead actions"}
	cmd.AddCommand(todoTechAddCmd())
	return cmd
}

func todoTechAddCmd() *cobra.Command {
	var issue, feature, text string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a task",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var t taskItem
			if err := c.post("/api/dx/todo/tech/add", map[string]any{
				"slug":    c.SlugOrDie(),
				"text":    text,
				"feature": feature,
				"issue":   issue,
			}, &t); err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", taskIDStr(t.ID), t.Text)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "link to issue (IS-N)")
	cmd.Flags().StringVar(&feature, "feature", "", "link to feature name")
	cmd.Flags().StringVar(&text, "text", "", "task description")
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
	if iss.BlockedBy != "" {
		fmt.Printf("Blocked:   %s\n", iss.BlockedBy)
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
}


// RunTodo kept for compatibility.
func RunTodo(_ []string) {}
