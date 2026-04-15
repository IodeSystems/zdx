package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/spf13/cobra"
)

func IssueCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "issue", Short: "Issue management"}
	cmd.AddCommand(issueListCmd(), issueAddCmd(), issueShowCmd(), issueCloseCmd(), issueEditCmd(), issueBlockCmd(), issueUnblockCmd())
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
				if len(iss.BlockedBy) > 0 {
					s += " [blocked:" + strings.Join(iss.BlockedBy, ",") + "]"
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
			body := map[string]any{
				"slug":       c.SlugOrDie(),
				"title":      title,
				"context":    ctx,
				"component":  component,
				"issue_type": issueType,
			}
			if blockedBy != "" {
				body["blocked_by"] = strings.Split(blockedBy, ",")
			}
			var iss issueItem
			if err := c.post("/api/dx/todo/issue/add", body, &iss); err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", issueIDStr(iss.ID), iss.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "context / description")
	cmd.Flags().StringVar(&component, "component", "", "component")
	cmd.Flags().StringVar(&blockedBy, "blocked-by", "", "blocking issues (IS-N, comma-separated)")
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
	var duplicateOf string
	cmd := &cobra.Command{
		Use:   "close <IS-N>",
		Short: "Close an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if reason == "duplicate" && duplicateOf == "" {
				return fmt.Errorf("--duplicate-of is required when --reason=duplicate")
			}
			if err := runCloseHooks(); err != nil {
				return err
			}
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			body := map[string]any{
				"slug":   c.SlugOrDie(),
				"id":     int32(n),
				"reason": reason,
			}
			if duplicateOf != "" {
				body["duplicate_of"] = duplicateOf
			}
			if err := c.post("/api/dx/todo/issue/close", body, &ok); err != nil {
				return err
			}
			fmt.Printf("%s closed\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "close reason")
	cmd.Flags().StringVar(&duplicateOf, "duplicate-of", "", "issue ID this duplicates (required when --reason=duplicate)")
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
		Short: "Add a blocker to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/todo/issue/add-block", map[string]any{
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
	var by string
	cmd := &cobra.Command{
		Use:   "unblock <IS-N>",
		Short: "Remove blocker(s) from an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			var ok struct {
				OK bool `json:"ok"`
			}
			body := map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(n),
			}
			if by != "" {
				body["blocked_by"] = by
			}
			if err := c.post("/api/dx/todo/issue/remove-block", body, &ok); err != nil {
				return err
			}
			if by != "" {
				fmt.Printf("%s unblocked from %s\n", id, by)
			} else {
				fmt.Printf("%s all blockers removed\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&by, "by", "", "specific blocker to remove (IS-N); omit to clear all")
	return cmd
}

func issueEditCmd() *cobra.Command {
	var title, ctx, component, issueType string
	var priority int
	cmd := &cobra.Command{
		Use:   "edit <IS-N>",
		Short: "Edit fields on an existing issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			n, _ := strconv.ParseInt(id[3:], 10, 32)
			c := mustClient()
			body := map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(n),
			}
			if cmd.Flags().Changed("title") {
				body["title"] = title
			}
			if cmd.Flags().Changed("context") {
				body["context"] = ctx
			}
			if cmd.Flags().Changed("priority") {
				body["priority"] = int32(priority)
			}
			if cmd.Flags().Changed("component") {
				body["component"] = component
			}
			if cmd.Flags().Changed("type") {
				body["issue_type"] = issueType
			}
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.post("/api/dx/todo/issue/edit", body, &ok); err != nil {
				return err
			}
			fmt.Printf("%s updated\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "context / description")
	cmd.Flags().IntVar(&priority, "priority", 0, "priority (1-4)")
	cmd.Flags().StringVar(&component, "component", "", "component")
	cmd.Flags().StringVar(&issueType, "type", "", "issue type: ops or impl")
	return cmd
}

// RunIssue kept for compatibility.
func RunIssue(_ []string) {}
