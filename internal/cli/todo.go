package cli

import (
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/apitypes"
)

func TodoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "todo",
		Short: "Workflow queue",
		RunE: func(cmd *cobra.Command, args []string) error {
			return soloRun(cmd, args)
		},
	}
	cmd.AddCommand(todoSoloCmd(), todoListCmd(), todoShowCmd(), todoDevCmd(), todoOwnerCmd(), todoTechCmd())
	return cmd
}

// ── solo ──────────────────────────────────────────────────────────────────────

func todoSoloCmd() *cobra.Command {
	var issue string
	cmd := &cobra.Command{
		Use:   "solo",
		Short: "Next actionable item",
		RunE:  func(cmd *cobra.Command, args []string) error { return soloRun(cmd, args) },
	}
	cmd.Flags().StringVar(&issue, "issue", "", "filter to specific issue")
	return cmd
}

func soloRun(cmd *cobra.Command, _ []string) error {
	issue, _ := cmd.Flags().GetString("issue")
	c := mustClient()
	params := url.Values{"slug": {c.SlugOrDie()}}
	if issue != "" {
		params.Set("issue", issue)
	}
	var item *apitypes.SoloItem
	if err := c.get("/api/dx/solo", params, &item); err != nil {
		return err
	}
	if item == nil {
		fmt.Println("nothing to do")
		return nil
	}
	fmt.Printf("[%s] %s  %s\n", item.Kind, item.ID, item.Summary)
	if item.Issue != "" && item.Issue != item.ID {
		fmt.Printf("  issue: %s\n", item.Issue)
	}
	if item.Feature != "" {
		fmt.Printf("  feature: %s\n", item.Feature)
	}
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
			params := url.Values{"slug": {c.SlugOrDie()}}
			if issue != "" {
				params.Set("issue", issue)
			}
			if feature != "" {
				params.Set("feature", feature)
			}
			var tasks []apitypes.TaskResp
			if err := c.get("/api/dx/tasks", params, &tasks); err != nil {
				return err
			}
			if len(tasks) == 0 {
				fmt.Println("no tasks")
				return nil
			}
			for _, t := range tasks {
				fmt.Printf("%-8s %-12s %s\n", t.ID, t.Status, t.Text)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "filter by issue")
	cmd.Flags().StringVar(&feature, "feature", "", "filter by feature")
	return cmd
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
				var iss apitypes.IssueResp
				if err := c.get("/api/dx/issue", url.Values{"slug": {slug}, "id": {id}}, &iss); err != nil {
					return err
				}
				printIssue(iss)
			case len(id) > 3 && id[:3] == "TK-":
				var tasks []apitypes.TaskResp
				if err := c.get("/api/dx/tasks", url.Values{"slug": {slug}}, &tasks); err != nil {
					return err
				}
				for _, t := range tasks {
					if t.ID == id {
						printTask(t)
						return nil
					}
				}
				return fmt.Errorf("task %s not found", id)
			default:
				var f apitypes.FeatureResp
				if err := c.get("/api/dx/feature", url.Values{"slug": {slug}, "name": {id}}, &f); err != nil {
					return err
				}
				printFeature(f)
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
			c := mustClient()
			var ok apitypes.OKResponse
			if err := c.post("/api/dx/task/done", apitypes.MarkTaskDoneRequest{
				Slug:     c.SlugOrDie(),
				ID:       args[0],
				TestPlan: testPlan,
				TestRefs: testRefs,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s done\n", args[0])
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
			c := mustClient()
			var ok apitypes.OKResponse
			if err := c.post("/api/dx/task/undone", apitypes.MarkTaskUndoneRequest{
				Slug: c.SlugOrDie(),
				ID:   args[0],
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s undone\n", args[0])
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
			c := mustClient()
			var ok apitypes.OKResponse
			if err := c.put("/api/dx/task/status", apitypes.UpdateTaskStatusRequest{
				Slug:   c.SlugOrDie(),
				ID:     args[0],
				Status: "in_progress",
				Reason: reason,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s started\n", args[0])
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
	var priority string
	cmd := &cobra.Command{
		Use:   "triage <IS-N>",
		Short: "Set issue priority",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if priority == "" {
				return fmt.Errorf("--priority is required")
			}
			c := mustClient()
			var ok apitypes.OKResponse
			if err := c.put("/api/dx/issue", apitypes.UpdateIssueRequest{
				Slug:     c.SlugOrDie(),
				ID:       args[0],
				Priority: priority,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s triaged (priority=%s)\n", args[0], priority)
			return nil
		},
	}
	cmd.Flags().StringVar(&priority, "priority", "", "priority 1-4 (1=highest)")
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
			if text == "" {
				return fmt.Errorf("--text is required")
			}
			c := mustClient()
			var t apitypes.TaskResp
			if err := c.post("/api/dx/task", apitypes.CreateTaskRequest{
				Slug:    c.SlugOrDie(),
				Text:    text,
				Feature: feature,
				Issue:   issue,
			}, &t); err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", t.ID, t.Text)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "link to issue")
	cmd.Flags().StringVar(&feature, "feature", "", "link to feature")
	cmd.Flags().StringVar(&text, "text", "", "task description")
	cmd.MarkFlagRequired("text")
	return cmd
}

// ── printers ──────────────────────────────────────────────────────────────────

func printIssue(iss apitypes.IssueResp) {
	fmt.Printf("ID:        %s\n", iss.ID)
	fmt.Printf("Title:     %s\n", iss.Title)
	fmt.Printf("Status:    %s\n", iss.Status)
	fmt.Printf("Priority:  %s\n", iss.Priority)
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

func printTask(t apitypes.TaskResp) {
	fmt.Printf("ID:      %s\n", t.ID)
	fmt.Printf("Text:    %s\n", t.Text)
	fmt.Printf("Status:  %s\n", t.Status)
	if t.Issue != "" {
		fmt.Printf("Issue:   %s\n", t.Issue)
	}
	if t.Feature != "" {
		fmt.Printf("Feature: %s\n", t.Feature)
	}
	if t.Reason != "" {
		fmt.Printf("Reason:  %s\n", t.Reason)
	}
}

func printFeature(f apitypes.FeatureResp) {
	fmt.Printf("Feature:     %s\n", f.Name)
	fmt.Printf("Description: %s\n", f.Description)
	if f.What != "" {
		fmt.Printf("What:        %s\n", f.What)
	}
	if f.Why != "" {
		fmt.Printf("Why:         %s\n", f.Why)
	}
	if f.DoneWhen != "" {
		fmt.Printf("Done when:   %s\n", f.DoneWhen)
	}
	if len(f.Specs) > 0 {
		fmt.Println("\nSpecs:")
		for _, s := range f.Specs {
			fmt.Printf("  [%s] %s\n", s.Kind, s.Description)
		}
	}
}

// RunTodo kept for compatibility — unused now but keeps old callers building.
func RunTodo(_ []string) { fmt.Fprintln(os.Stderr, "use TodoCmd()") }
