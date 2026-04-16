package project

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
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
	cmd.AddCommand(commentListCmd(), commentAddCmd(), commentMarkReadCmd(), commentReplyCmd(), commentReactCmd())
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
			q := url.Values{
				"slug":        {c.SlugOrDie()},
				"target_type": {args[0]},
				"target_id":   {args[1]},
			}
			if role != "" {
				q.Set("role", role)
			}
			var resp struct {
				Comments []clitypes.CommentItem `json:"comments"`
			}
			if err := c.Get("/api/dx/comment/list", q, &resp); err != nil {
				return err
			}
			if len(resp.Comments) == 0 {
				fmt.Println("no comments")
				return nil
			}
			cli.PrintComments(resp.Comments)
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
			var cm clitypes.CommentItem
			if err := c.Post("/api/dx/comment/add", payload, &cm); err != nil {
				return err
			}
			fmt.Printf("C-%d added\n", cm.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment body")
	cmd.Flags().StringVar(&authorAlias, "as", "", "author alias (e.g. claude); falls back to $DX_AUTHOR_ALIAS")
	cmd.MarkFlagRequired("body")
	return cmd
}

func commentMarkReadCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "mark-read <target-type> <target-id> | mark-read C-1,C-2,...",
		Short: "Mark comments as read — by target or by comment IDs",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()

			// Check if first arg looks like C-N (batch by comment IDs)
			if strings.HasPrefix(args[0], "C-") {
				ids := strings.Split(args[0], ",")
				for _, raw := range ids {
					for _, id := range strings.Split(raw, ",") {
						id = strings.TrimSpace(id)
						cid, err := parseCommentID(id)
						if err != nil {
							return err
						}
						// Resolve comment to its target
						var cm clitypes.CommentItem
						if err := c.Get("/api/dx/comment/get", url.Values{"id": {strconv.Itoa(int(cid))}}, &cm); err != nil {
							return fmt.Errorf("C-%d: %w", cid, err)
						}
						var ok struct {
							OK bool `json:"ok"`
						}
						if err := c.Post("/api/dx/comment/mark-read", dxclient.MarkCommentsReadRequest{
							Slug:       c.SlugOrDie(),
							TargetType: cm.TargetType,
							TargetId:   cm.TargetID,
							Role:       role,
						}, &ok); err != nil {
							return fmt.Errorf("C-%d: %w", cid, err)
						}
						fmt.Printf("C-%d marked read\n", cid)
					}
				}
				return nil
			}

			// Legacy: mark-read <target-type> <target-id>
			if len(args) < 2 {
				return fmt.Errorf("usage: mark-read <target-type> <target-id> or mark-read C-1,C-2,...")
			}
			var ok struct {
				OK bool `json:"ok"`
			}
			if err := c.Post("/api/dx/comment/mark-read", dxclient.MarkCommentsReadRequest{
				Slug:       c.SlugOrDie(),
				TargetType: args[0],
				TargetId:   args[1],
				Role:       role,
			}, &ok); err != nil {
				return err
			}
			fmt.Printf("marked read\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "reader role (e.g. dev, owner)")
	cmd.MarkFlagRequired("role")
	return cmd
}

func commentReplyCmd() *cobra.Command {
	var body, react, authorAlias string
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

			var orig clitypes.CommentItem
			if err := c.Get("/api/dx/comment/get", url.Values{"id": {strconv.Itoa(int(cid))}}, &orig); err != nil {
				return fmt.Errorf("could not find C-%d: %w", cid, err)
			}

			if body != "" {
				payload := dxclient.AddCommentRequest{
					Slug:       c.SlugOrDie(),
					TargetType: orig.TargetType,
					TargetId:   orig.TargetID,
					Body:       body,
					ParentId:   &cid,
				}
				if alias := resolveAuthorAlias(authorAlias); alias != "" {
					payload.AuthorAlias = &alias
				}
				var cm clitypes.CommentItem
				if err := c.Post("/api/dx/comment/add", payload, &cm); err != nil {
					return err
				}
				fmt.Printf("C-%d added (reply to C-%d)\n", cm.ID, cid)
			}

			if react != "" {
				var resp struct {
					ID int32 `json:"id"`
				}
				if err := c.Post("/api/dx/comment/react", dxclient.ReactToCommentRequest{
					Slug:      c.SlugOrDie(),
					CommentId: cid,
					Emoji:     react,
				}, &resp); err != nil {
					return err
				}
				fmt.Printf("reacted %s to C-%d\n", react, cid)
			}

			if body == "" && react == "" {
				return fmt.Errorf("provide --body and/or --react")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "reply body")
	cmd.Flags().StringVar(&react, "react", "", "reaction emoji (e.g. thumbs-up, +1)")
	cmd.Flags().StringVar(&authorAlias, "as", "", "author alias (e.g. claude); falls back to $DX_AUTHOR_ALIAS")
	return cmd
}

func commentReactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "react <C-N> <emoji>",
		Short: "React to a comment (e.g. comment react C-123 thumbs-up)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			cid, err := parseCommentID(args[0])
			if err != nil {
				return err
			}
			var resp struct {
				ID int32 `json:"id"`
			}
			if err := c.Post("/api/dx/comment/react", dxclient.ReactToCommentRequest{
				Slug:      c.SlugOrDie(),
				CommentId: cid,
				Emoji:     args[1],
			}, &resp); err != nil {
				return err
			}
			fmt.Printf("reacted %s to C-%d\n", args[1], cid)
			return nil
		},
	}
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
			var resp struct {
				Revisions []clitypes.RevisionItem `json:"revisions"`
			}
			if err := c.Get("/api/dx/revisions", url.Values{
				"slug":        {c.SlugOrDie()},
				"target_type": {args[0]},
				"target_id":   {args[1]},
			}, &resp); err != nil {
				return err
			}
			if len(resp.Revisions) == 0 {
				fmt.Println("no revisions")
				return nil
			}
			for _, r := range resp.Revisions {
				date := r.CreatedAt
				if len(date) >= 10 {
					date = date[:10]
				}
				fmt.Printf("[%s] %s %s: %s → %s (%s)\n", date, r.TargetID, r.Field, r.OldVal, r.NewVal, r.Agent)
			}
			return nil
		},
	}
}
