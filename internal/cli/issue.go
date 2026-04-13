package cli

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/apitypes"
)

func IssueCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "issue", Short: "Issue management"}
	cmd.AddCommand(issueListCmd(), issueAddCmd(), issueShowCmd(), issueCloseCmd())
	return cmd
}

func issueListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List issues",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var issues []apitypes.IssueResp
			if err := c.get("/api/dx/issues", url.Values{"slug": {c.SlugOrDie()}}, &issues); err != nil {
				return err
			}
			if len(issues) == 0 {
				fmt.Println("no issues")
				return nil
			}
			for _, iss := range issues {
				status := iss.Status
				if iss.BlockedBy != "" {
					status += " [blocked:" + iss.BlockedBy + "]"
				}
				fmt.Printf("%-8s %-20s %s\n", iss.ID, status, iss.Title)
			}
			return nil
		},
	}
}

func issueAddCmd() *cobra.Command {
	var title, ctx, priority, component string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create an issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			c := mustClient()
			var iss apitypes.IssueResp
			if err := c.post("/api/dx/issue", apitypes.CreateIssueRequest{
				Slug:      c.SlugOrDie(),
				Title:     title,
				Context:   ctx,
				Priority:  priority,
				Component: component,
			}, &iss); err != nil {
				return err
			}
			fmt.Printf("%s  %s\n", iss.ID, iss.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "context / description")
	cmd.Flags().StringVar(&priority, "priority", "", "priority 1-4")
	cmd.Flags().StringVar(&component, "component", "", "component")
	cmd.MarkFlagRequired("title")
	return cmd
}

func issueShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <IS-N>",
		Short: "Show issue detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var iss apitypes.IssueResp
			if err := c.get("/api/dx/issue", url.Values{"slug": {c.SlugOrDie()}, "id": {args[0]}}, &iss); err != nil {
				return err
			}
			printIssue(iss)
			if len(iss.Work) > 0 {
				fmt.Println("\nWork log:")
				for _, w := range iss.Work {
					fmt.Printf("  [%s] %s: %s\n", w.CreatedAt[:10], w.Agent, w.Note)
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
			c := mustClient()
			var ok apitypes.OKResponse
			if err := c.post("/api/dx/issue/close", apitypes.CloseIssueRequest{
				Slug:   c.SlugOrDie(),
				ID:     args[0],
				Reason: reason,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("%s closed\n", args[0])
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "close reason")
	return cmd
}

// RunIssue kept for compatibility.
func RunIssue(_ []string) {}
