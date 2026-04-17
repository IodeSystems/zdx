package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

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
			resp, err := c.AddQuestionProposalWithResponse(cmd.Context(), dxclient.AddQuestionProposalRequest{
				Slug:         c.SlugOrDie(),
				QuestionId:   questionID,
				QuestionType: questionType,
				Title:        title,
				Context:      ctx,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			p := resp.JSON200
			fmt.Printf("QP-%d  [%s:%d]  %s\n", p.Id, p.QuestionType, p.QuestionId, p.Title)
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
			resp, err := c.ListQuestionProposalsWithResponse(cmd.Context(), &dxclient.ListQuestionProposalsParams{
				Slug:         c.SlugOrDie(),
				QuestionId:   questionID,
				QuestionType: questionType,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Proposals == nil || len(*resp.JSON200.Proposals) == 0 {
				fmt.Println("no proposals")
				return nil
			}
			for _, p := range *resp.JSON200.Proposals {
				line := fmt.Sprintf("QP-%-4d %-10s  %s", p.Id, p.Status, p.Title)
				if p.CreatedIssueId != "" {
					line += fmt.Sprintf("  → %s", p.CreatedIssueId)
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
			resp, err := c.AcceptQuestionProposalWithResponse(cmd.Context(), dxclient.AcceptQuestionProposalRequest{
				Slug: c.SlugOrDie(),
				Id:   int32(id),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("QP-%d  accepted → %s\n", resp.JSON200.Proposal.Id, clitypes.IssueIDStr(resp.JSON200.Issue.Id))
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
			resp, err := c.DenyQuestionProposalWithResponse(cmd.Context(), dxclient.DenyQuestionProposalRequest{
				Slug:   c.SlugOrDie(),
				Id:     int32(id),
				Reason: reason,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("QP-%d  denied\n", resp.JSON200.Id)
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for denying (optional)")
	return cmd
}
