package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

type proposalItem struct {
	ID             int32   `json:"id"`
	JournalEntryID *int32  `json:"journal_entry_id,omitempty"`
	Title          string  `json:"title"`
	Context        string  `json:"context"`
	Status         string  `json:"status"`
	Priority       int32   `json:"priority"`
	FiledIssueID   *string `json:"filed_issue_id,omitempty"`
	CreatedAt      string  `json:"created_at"`
}

func ProposalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "proposal", Short: "Proposal management"}
	cmd.AddCommand(
		proposalListCmd(),
		proposalAddCmd(),
		proposalReviewCmd(),
	)
	return cmd
}

func proposalListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			q := querySlug(c)
			if status != "" {
				q.Set("status", status)
			}
			var resp struct {
				Proposals []proposalItem `json:"proposals"`
			}
			if err := c.get("/api/proposals", q, &resp); err != nil {
				return err
			}
			if len(resp.Proposals) == 0 {
				fmt.Println("no proposals")
				return nil
			}
			for _, p := range resp.Proposals {
				filed := ""
				if p.FiledIssueID != nil {
					filed = " → " + *p.FiledIssueID
				}
				fmt.Printf("%-4d [P%d] %-10s  %s%s\n", p.ID, p.Priority, p.Status, p.Title, filed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (proposed/filed/backlogged/dismissed)")
	return cmd
}

func proposalAddCmd() *cobra.Command {
	var ctx string
	var priority int32
	var journalEntry int32
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			body := map[string]any{
				"slug":     c.SlugOrDie(),
				"title":    args[0],
				"context":  ctx,
				"priority": priority,
			}
			if journalEntry > 0 {
				body["journal_entry_id"] = journalEntry
			}
			var p proposalItem
			if err := c.post("/api/proposal", body, &p); err != nil {
				return err
			}
			fmt.Printf("%d  %s\n", p.ID, p.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&ctx, "context", "", "proposal context/description")
	cmd.Flags().Int32Var(&priority, "priority", 0, "priority (higher = more important)")
	cmd.Flags().Int32Var(&journalEntry, "journal-entry", 0, "link to journal entry ID")
	return cmd
}

func proposalReviewCmd() *cobra.Command {
	var action string
	cmd := &cobra.Command{
		Use:   "review <id>",
		Short: "Review a proposal (file/backlog/dismiss)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid proposal ID: %s", args[0])
			}
			c := mustClient()
			switch action {
			case "file":
				// Create an issue from the proposal, then mark it filed
				var resp struct {
					Proposals []proposalItem `json:"proposals"`
				}
				if err := c.get("/api/proposals", querySlug(c), &resp); err != nil {
					return err
				}
				var prop *proposalItem
				for i := range resp.Proposals {
					if resp.Proposals[i].ID == int32(id) {
						prop = &resp.Proposals[i]
						break
					}
				}
				if prop == nil {
					return fmt.Errorf("proposal %d not found", id)
				}
				var issue struct {
					ID string `json:"id"`
				}
				if err := c.post("/api/dx/todo/issue/add", map[string]any{
					"slug":    c.SlugOrDie(),
					"title":   prop.Title,
					"context": prop.Context,
				}, &issue); err != nil {
					return fmt.Errorf("failed to create issue: %w", err)
				}
				var ok struct{ OK bool }
				if err := c.post("/api/proposal/file", map[string]any{
					"id":             id,
					"filed_issue_id": issue.ID,
				}, &ok); err != nil {
					return err
				}
				fmt.Printf("proposal %d filed as %s\n", id, issue.ID)
			case "backlog":
				var ok struct{ OK bool }
				if err := c.put("/api/proposal", map[string]any{"id": id, "status": "backlogged"}, &ok); err != nil {
					return err
				}
				fmt.Printf("proposal %d backlogged\n", id)
			case "dismiss":
				var ok struct{ OK bool }
				if err := c.put("/api/proposal", map[string]any{"id": id, "status": "dismissed"}, &ok); err != nil {
					return err
				}
				fmt.Printf("proposal %d dismissed\n", id)
			default:
				return fmt.Errorf("--action must be one of: file, backlog, dismiss")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&action, "action", "", "review action (file/backlog/dismiss)")
	_ = cmd.MarkFlagRequired("action")
	return cmd
}
