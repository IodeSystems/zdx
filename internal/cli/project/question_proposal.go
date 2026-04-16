package project

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

type questionProposalItem struct {
	ID             int32  `json:"id"`
	QuestionID     int32  `json:"question_id"`
	QuestionType   string `json:"question_type"`
	Title          string `json:"title"`
	Context        string `json:"context"`
	Status         string `json:"status"`
	DeniedReason   string `json:"denied_reason"`
	CreatedIssueID string `json:"created_issue_id"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func QuestionProposalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "question-proposal", Short: "Issue proposals attached to questions"}
	cmd.AddCommand(questionProposalAddCmd(), questionProposalListCmd(), questionProposalAcceptCmd(), questionProposalDenyCmd())
	return cmd
}

func questionProposalAddCmd() *cobra.Command {
	var questionID int32
	var questionType, title, ctx string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create an issue proposal on a question",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var p questionProposalItem
			if err := c.Post("/api/dx/question-proposals/add", map[string]any{
				"slug":          c.SlugOrDie(),
				"question_id":   questionID,
				"question_type": questionType,
				"title":         title,
				"context":       ctx,
			}, &p); err != nil {
				return err
			}
			fmt.Printf("QP-%d  [%s:%d]  %s\n", p.ID, p.QuestionType, p.QuestionID, p.Title)
			return nil
		},
	}
	cmd.Flags().Int32Var(&questionID, "question-id", 0, "question ID")
	cmd.Flags().StringVar(&questionType, "question-type", "qa", "question type (qa or blocker)")
	cmd.Flags().StringVar(&title, "title", "", "proposed issue title")
	cmd.Flags().StringVar(&ctx, "context", "", "proposed issue context")
	cmd.MarkFlagRequired("question-id")
	cmd.MarkFlagRequired("title")
	return cmd
}

func questionProposalListCmd() *cobra.Command {
	var questionID int32
	var questionType string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proposals for a question",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var resp struct {
				Proposals []questionProposalItem `json:"proposals"`
			}
			params := url.Values{
				"slug":          {c.SlugOrDie()},
				"question_id":   {strconv.Itoa(int(questionID))},
				"question_type": {questionType},
			}
			if err := c.Get("/api/dx/question-proposals/by-question", params, &resp); err != nil {
				return err
			}
			if len(resp.Proposals) == 0 {
				fmt.Println("no proposals")
				return nil
			}
			for _, p := range resp.Proposals {
				line := fmt.Sprintf("QP-%-4d %-10s  %s", p.ID, p.Status, p.Title)
				if p.CreatedIssueID != "" {
					line += fmt.Sprintf("  → %s", p.CreatedIssueID)
				}
				fmt.Println(line)
			}
			return nil
		},
	}
	cmd.Flags().Int32Var(&questionID, "question-id", 0, "question ID")
	cmd.Flags().StringVar(&questionType, "question-type", "qa", "question type (qa or blocker)")
	cmd.MarkFlagRequired("question-id")
	return cmd
}

func questionProposalAcceptCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "accept <ID>",
		Short: "Accept a proposal (creates an issue)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}
			var resp struct {
				Proposal questionProposalItem `json:"proposal"`
				Issue    struct {
					ID string `json:"id"`
				} `json:"issue"`
			}
			if err := c.Post("/api/dx/question-proposals/accept", map[string]any{
				"slug": c.SlugOrDie(),
				"id":   int32(id),
			}, &resp); err != nil {
				return err
			}
			fmt.Printf("QP-%d  accepted → %s\n", resp.Proposal.ID, resp.Issue.ID)
			return nil
		},
	}
	return cmd
}

func questionProposalDenyCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "deny <ID>",
		Short: "Deny a proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}
			var p questionProposalItem
			if err := c.Post("/api/dx/question-proposals/deny", map[string]any{
				"slug":   c.SlugOrDie(),
				"id":     int32(id),
				"reason": reason,
			}, &p); err != nil {
				return err
			}
			fmt.Printf("QP-%d  denied\n", p.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for denying (optional)")
	return cmd
}
