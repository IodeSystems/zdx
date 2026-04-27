package project

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ProposalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "proposal", Short: "Proposal management"}
	cmd.AddCommand(
		proposalAddCmd(),
		proposalListCmd(),
		proposalShowCmd(),
		proposalEditCmd(),
		proposalApproveCmd(),
		proposalRejectCmd(),
		proposalSnoozeCmd(),
	)
	return cmd
}

func parseProposalID(s string) (int32, error) {
	s = strings.TrimPrefix(s, "PR-")
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid proposal ID: %s", s)
	}
	return int32(id), nil
}

func proposalIDStr(id int32) string {
	return fmt.Sprintf("PR-%d", id)
}

func proposalAddCmd() *cobra.Command {
	var title, value, body, sourceType, sourceRef, duplicatesReviewed string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a proposal",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			if sourceType == "" {
				sourceType = "session-review"
			}
			req := dxclient.CreateProposalRequest{
				Slug:       c.SlugOrDie(),
				Title:      title,
				Value:      value,
				Body:       body,
				SourceType: sourceType,
			}
			if sourceRef != "" {
				req.SourceRef = &sourceRef
			}
			if duplicatesReviewed != "" {
				req.DuplicatesReviewed = &duplicatesReviewed
			}
			resp, err := c.CreateProposalWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			r := resp.JSON200
			if r.DuplicatesReviewToken != nil {
				fmt.Println("Similar proposals found — review before submitting:")
				if r.Similar != nil {
					for _, s := range *r.Similar {
						fmt.Printf("  PR-%-4d [%-10s]  %s\n", s.Id, s.Status, s.Title)
					}
				}
				fmt.Printf("\nTo submit anyway:\n  dx proposal add --title %q --body %q --duplicates-reviewed=%s\n",
					title, body, *r.DuplicatesReviewToken)
				return nil
			}
			if r.Proposal == nil {
				return fmt.Errorf("empty response")
			}
			p := r.Proposal
			fmt.Printf("%s  [%s]  %s\n", proposalIDStr(p.Id), p.SourceType, p.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "proposal title")
	cmd.Flags().StringVar(&value, "value", "", "value / goals justification (why this matters)")
	cmd.Flags().StringVar(&body, "body", "", "proposal body (implementation details)")
	cmd.Flags().StringVar(&sourceType, "source", "session-review", "source type (session-review, conversation, etc.)")
	cmd.Flags().StringVar(&sourceRef, "source-ref", "", "source reference (optional)")
	cmd.Flags().StringVar(&duplicatesReviewed, "duplicates-reviewed", "", "server-provided token confirming duplicate review")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func proposalListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := &dxclient.ListProposalsParams{Slug: c.SlugOrDie()}
			if status != "" && status != "all" {
				params.Status = &status
			}
			resp, err := c.ListProposalsWithResponse(cmd.Context(), params)
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
				line := fmt.Sprintf("%-8s %-10s  %s", proposalIDStr(p.Id), p.Status, p.Title)
				if p.ApprovedIssueId != nil && *p.ApprovedIssueId != "" {
					line += fmt.Sprintf("  → %s", *p.ApprovedIssueId)
				}
				fmt.Println(line)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "proposed", "filter by status (proposed|snoozed|all)")
	return cmd
}

func proposalShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <PR-N>",
		Short: "Show a proposal with version history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseProposalID(args[0])
			if err != nil {
				return err
			}
			resp, err := c.ShowProposalWithResponse(cmd.Context(), id, &dxclient.ShowProposalParams{
				Slug: c.SlugOrDie(),
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
			p := resp.JSON200.Proposal
			fmt.Printf("%s  [%s]  %s\n", proposalIDStr(p.Id), p.Status, p.Title)
			fmt.Printf("source: %s", p.SourceType)
			if p.SourceRef != nil {
				fmt.Printf(" (%s)", *p.SourceRef)
			}
			fmt.Println()
			if p.ApprovedIssueId != nil && *p.ApprovedIssueId != "" {
				fmt.Printf("approved → %s\n", *p.ApprovedIssueId)
			}
			if p.SnoozedUntil != nil {
				fmt.Printf("snoozed until: %s\n", *p.SnoozedUntil)
			}
			fmt.Printf("created: %s  by %s\n\n", p.CreatedAt, p.CreatedBy)
			if p.Value != "" {
				fmt.Printf("Value:\n%s\n\n", p.Value)
			}
			fmt.Println(p.Body)
			if resp.JSON200.Versions != nil && len(*resp.JSON200.Versions) > 0 {
				fmt.Printf("\n--- version history (%d) ---\n", len(*resp.JSON200.Versions))
				for _, v := range *resp.JSON200.Versions {
					fmt.Printf("\n[v%d]  %s  by %s\n%s\n", v.Id, v.EditedAt, v.EditedBy, v.Body)
				}
			}
			return nil
		},
	}
	return cmd
}

func proposalEditCmd() *cobra.Command {
	var title, value, body string
	cmd := &cobra.Command{
		Use:   "edit <PR-N>",
		Short: "Update a proposal body (creates a version)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseProposalID(args[0])
			if err != nil {
				return err
			}
			resp, err := c.UpdateProposalWithResponse(cmd.Context(), id, dxclient.UpdateProposalRequest{
				Slug:  c.SlugOrDie(),
				Title: title,
				Value: value,
				Body:  body,
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
			fmt.Printf("%s  updated  %s\n", proposalIDStr(p.Id), p.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&value, "value", "", "value / goals justification")
	cmd.Flags().StringVar(&body, "body", "", "new body")
	_ = cmd.MarkFlagRequired("body")
	return cmd
}

func proposalApproveCmd() *cobra.Command {
	var issueType string
	var priority int32
	cmd := &cobra.Command{
		Use:   "approve <PR-N>",
		Short: "Approve a proposal (creates an issue)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseProposalID(args[0])
			if err != nil {
				return err
			}
			req := dxclient.ApproveProposalRequest{Slug: c.SlugOrDie()}
			if issueType != "" {
				req.IssueType = &issueType
			}
			if priority != 0 {
				req.Priority = &priority
			}
			resp, err := c.ApproveProposalWithResponse(cmd.Context(), id, req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			p := resp.JSON200.Proposal
			issueID := ""
			if p.ApprovedIssueId != nil {
				issueID = *p.ApprovedIssueId
			}
			fmt.Printf("%s  approved  → %s\n", proposalIDStr(p.Id), issueID)
			return nil
		},
	}
	cmd.Flags().StringVar(&issueType, "type", "impl", "issue type for the created issue (impl|ops|ask|tracker)")
	cmd.Flags().Int32Var(&priority, "priority", 0, "priority for the created issue (1-4)")
	return cmd
}

func proposalRejectCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "reject <PR-N>",
		Short: "Reject a proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseProposalID(args[0])
			if err != nil {
				return err
			}
			req := dxclient.RejectProposalRequest{Slug: c.SlugOrDie()}
			if reason != "" {
				req.Reason = &reason
			}
			resp, err := c.RejectProposalWithResponse(cmd.Context(), id, req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("%s  rejected\n", proposalIDStr(resp.JSON200.Id))
			return nil
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "reason for rejection (optional)")
	return cmd
}

func proposalSnoozeCmd() *cobra.Command {
	var until string
	cmd := &cobra.Command{
		Use:   "snooze <PR-N>",
		Short: "Snooze a proposal until a date",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseProposalID(args[0])
			if err != nil {
				return err
			}
			resp, err := c.SnoozeProposalWithResponse(cmd.Context(), id, dxclient.SnoozeProposalRequest{
				Slug:         c.SlugOrDie(),
				SnoozedUntil: until,
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
			snoozedUntil := ""
			if p.SnoozedUntil != nil {
				snoozedUntil = *p.SnoozedUntil
			}
			fmt.Printf("%s  snoozed until %s\n", proposalIDStr(p.Id), snoozedUntil)
			return nil
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "snooze until date (YYYY-MM-DD)")
	_ = cmd.MarkFlagRequired("until")
	return cmd
}
