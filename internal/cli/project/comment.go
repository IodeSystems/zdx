package project

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// resolveAuthorAlias returns the explicit --as flag value if set, otherwise
// falls back to DX_AUTHOR_ALIAS so agents running under /work auto-tag their
// comments without needing to remember --as on every call.
func resolveAuthorAlias(flag string) string {
	if flag != "" {
		return flag
	}
	return os.Getenv("DX_AUTHOR_ALIAS")
}

func CommentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "comment", Short: "Comment on issues, tasks, and features"}
	cmd.AddCommand(commentListCmd(), commentAddCmd(), commentReplyCmd())
	return cmd
}

func commentListCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "list <target-type> <target-id>",
		Short: "List comments on a target (e.g. comment list issue IS-5)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			params := &dxclient.ListCommentsParams{
				Slug:       &slug,
				TargetType: &args[0],
				TargetId:   &args[1],
			}
			if role != "" {
				params.Role = &role
			}
			resp, err := c.ListCommentsWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Comments == nil || len(*resp.JSON200.Comments) == 0 {
				fmt.Println("no comments")
				return nil
			}
			cli.PrintComments(cli.CommentsToCli(resp.JSON200.Comments))
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "show read/unread indicator for this role (e.g. llm, dev)")
	return cmd
}

func commentAddCmd() *cobra.Command {
	var body, authorAlias string
	cmd := &cobra.Command{
		Use:   "add <target-type> <target-id>",
		Short: "Add a comment (e.g. comment add issue IS-5 --body '...')",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			payload := dxclient.AddCommentRequest{
				Slug:       c.SlugOrDie(),
				TargetType: args[0],
				TargetId:   args[1],
				Body:       body,
			}
			if alias := resolveAuthorAlias(authorAlias); alias != "" {
				payload.AuthorAlias = &alias
			}
			resp, err := c.AddCommentWithResponse(cmd.Context(), payload)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("C-%d added\n", resp.JSON200.Id)
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment body")
	cmd.Flags().StringVar(&authorAlias, "as", "", "author alias (e.g. claude); falls back to $DX_AUTHOR_ALIAS")
	cmd.MarkFlagRequired("body")
	return cmd
}

func commentReplyCmd() *cobra.Command {
	var body, authorAlias string
	cmd := &cobra.Command{
		Use:   "reply <C-N>",
		Short: "Reply to a comment by ID (e.g. comment reply C-123 --body '...')",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			cid, err := parseCommentID(args[0])
			if err != nil {
				return err
			}

			getResp, err := c.GetCommentWithResponse(cmd.Context(), &dxclient.GetCommentParams{Id: cid})
			if err != nil {
				return fmt.Errorf("could not find C-%d: %w", cid, err)
			}
			if err := c.CheckStatus(getResp.StatusCode(), getResp.Body); err != nil {
				return fmt.Errorf("could not find C-%d: %w", cid, err)
			}
			if getResp.JSON200 == nil {
				return fmt.Errorf("could not find C-%d", cid)
			}
			orig := getResp.JSON200

			if body != "" {
				payload := dxclient.AddCommentRequest{
					Slug:       c.SlugOrDie(),
					TargetType: orig.TargetType,
					TargetId:   orig.TargetId,
					Body:       body,
					ParentId:   &cid,
				}
				if alias := resolveAuthorAlias(authorAlias); alias != "" {
					payload.AuthorAlias = &alias
				}
				addResp, err := c.AddCommentWithResponse(cmd.Context(), payload)
				if err != nil {
					return err
				}
				if err := c.CheckStatus(addResp.StatusCode(), addResp.Body); err != nil {
					return err
				}
				if addResp.JSON200 == nil {
					return fmt.Errorf("empty response")
				}
				fmt.Printf("C-%d added (reply to C-%d)\n", addResp.JSON200.Id, cid)
			}

			if body == "" {
				return fmt.Errorf("provide --body")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "reply body")
	cmd.Flags().StringVar(&authorAlias, "as", "", "author alias (e.g. claude); falls back to $DX_AUTHOR_ALIAS")
	return cmd
}

func parseCommentID(s string) (int32, error) {
	s = strings.TrimPrefix(s, "C-")
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid comment ID %q (expected C-N)", s)
	}
	return int32(n), nil
}

func RevisionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "revision", Short: "View revision history"}
	cmd.AddCommand(revisionListCmd())
	return cmd
}

func revisionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <target-type> <target-id>",
		Short: "List revisions for a target",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			resp, err := c.ListRevisionsWithResponse(cmd.Context(), &dxclient.ListRevisionsParams{
				Slug:       &slug,
				TargetType: &args[0],
				TargetId:   &args[1],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Revisions == nil || len(*resp.JSON200.Revisions) == 0 {
				fmt.Println("no revisions")
				return nil
			}
			for _, r := range *resp.JSON200.Revisions {
				date := r.CreatedAt
				if len(date) >= 10 {
					date = date[:10]
				}
				fmt.Printf("[%s] %s %s: %s → %s (%s)\n", date, r.TargetId, r.Field, r.OldVal, r.NewVal, r.Agent)
			}
			return nil
		},
	}
}
