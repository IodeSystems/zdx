package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/spf13/cobra"
)

func IssueCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "issue", Short: "Issue management"}
	cmd.AddCommand(issueListCmd(), issueAddCmd(), issueShowCmd(), issueCloseCmd(), issueBlockCmd(), issueUnblockCmd())
	return cmd
}

func issueListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp struct {
				Issues []issueItem `json:"issues"`
			}
			if err := c.get("/api/dx/todo/issue/list", url.Values{"slug": {c.SlugOrDie()}}, &resp); err != nil {
				return err
			}
			if len(resp.Issues) == 0 {
				fmt.Println("no issues")
				return nil
			}
			for _, iss := range resp.Issues {
				if status != "" && iss.Status != status {
					continue
				}
				s := iss.Status
				if iss.BlockedBy != "" {
					s += " [blocked:" + iss.BlockedBy + "]"
				}
				fmt.Printf("%-8s %-30s %s\n", issueIDStr(iss.ID), s, iss.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (open, closed)")
	return cmd
}

func issueAddCmd() *cobra.Command {
	var title, ctx, component, blockedBy, issueType string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create an issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var iss issueItem
			if err := c.post("/api/dx/todo/issue/add", map[string]any{
				"slug":       c.SlugOrDie(),
				"title":      title,
				"context":    ctx,
				"component":  component,
				"blocked_by": blockedBy,
				"issue_type": issueType,
			}, &iss); err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", issueIDStr(iss.ID), iss.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "context / description")
	cmd.Flags().StringVar(&component, "component", "", "component")
	cmd.Flags().StringVar(&blockedBy, "blocked-by", "", "blocking issue (IS-N)")
	cmd.Flags().StringVar(&issueType, "type", "ops", "issue type: ops or impl")
	cmd.MarkFlagRequired("title")
	return cmd
}

func issueShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <IS-N>",
		Short: "Show issue detail with work log",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp struct {
				Issue issueItem       `json:"issue"`
				Work  []issueWorkItem `json:"work"`
			}
			if err := c.get("/api/dx/todo/issue/show", url.Values{
				"slug": {c.SlugOrDie()},
				"id":   {args[0]},
			}, &resp); err != nil {
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
			return nil
		},
	}
}

func issueCloseCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "close <IS-N>",
		Short: "Close an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := runCloseHooks(); err != nil {
				return err
			}
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct{ OK bool `json:"ok"` }
			if err := c.post("/api/dx/todo/issue/close", map[string]any{
				"slug":   c.SlugOrDie(),
				"id":     int32(n),
				"reason": reason,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s closed\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "close reason")
	return cmd
}

func runCloseHooks() error {
	cfg := config.Load()
	if cfg == nil {
		return nil
	}
	steps := cfg.AllCloseSteps("")
	if len(steps) == 0 {
		return nil
	}
	for _, ns := range steps {
		label := ns.Name
		if label == "" {
			label = ns.Run
		}
		fmt.Printf("[close] %s: %s\n", ns.Component, label)
		if err := runShell(ns.Run, ns.CWD); err != nil {
			return fmt.Errorf("close hook %q failed: %w", label, err)
		}
	}
	return nil
}

func issueBlockCmd() *cobra.Command {
	var by string
	cmd := &cobra.Command{
		Use:   "block <IS-N>",
		Short: "Set a blocker on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct{ OK bool `json:"ok"` }
			if err := c.post("/api/dx/todo/issue/set-blocked-by", map[string]any{
				"slug":       c.SlugOrDie(),
				"id":         int32(n),
				"blocked_by": by,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s blocked by %s\n", id, by)
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "blocking issue (IS-N)")
	cmd.MarkFlagRequired("by")
	return cmd
}

func issueUnblockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unblock <IS-N>",
		Short: "Clear blockers on an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct{ OK bool `json:"ok"` }
			if err := c.post("/api/dx/todo/issue/set-blocked-by", map[string]any{
				"slug":       c.SlugOrDie(),
				"id":         int32(n),
				"blocked_by": "",
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s unblocked\n", id)
			return nil
		},
	}
}

// RunIssue kept for compatibility.
func RunIssue(_ []string) {}
