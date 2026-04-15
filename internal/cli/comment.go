package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type commentItem struct {
	ID          int32  `json:"id"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	Author      string `json:"author"`
	AuthorAlias string `json:"author_alias,omitempty"`
	Body        string `json:"body"`
	CreatedAt   string `json:"created_at"`
	ParentID    *int32 `json:"parent_id,omitempty"`
	Unread      *bool  `json:"unread,omitempty"`
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
			c := mustClient()
			q := url.Values{
				"slug":        {c.SlugOrDie()},
				"target_type": {args[0]},
				"target_id":   {args[1]},
			}
			if role != "" {
				q.Set("role", role)
			}
			var resp struct {
				Comments []commentItem `json:"comments"`
			}
			if err := c.get("/api/dx/comment/list", q, &resp); err != nil {
				return err
			}
			if len(resp.Comments) == 0 {
				fmt.Println("no comments")
				return nil
			}
			printComments(resp.Comments)
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "", "show read/unread indicator for this role (e.g. llm, dev)")
	return cmd
}

func printComments(comments []commentItem) {
	for _, cm := range comments {
		date := cm.CreatedAt
		if len(date) >= 10 {
			date = date[:10]
		}
		dot := ""
		if cm.Unread != nil {
			if *cm.Unread {
				dot = "○ "
			} else {
				dot = "● "
			}
		}
		indent := ""
		if cm.ParentID != nil {
			indent = "  ↳ "
		}
		author := cm.Author
		if cm.AuthorAlias != "" {
			author = cm.AuthorAlias + " (" + cm.Author + ")"
		}
		fmt.Printf("%s%sC-%d [%s] %s: %s\n", indent, dot, cm.ID, date, author, cm.Body)
	}
}

func commentAddCmd() *cobra.Command {
	var body, authorAlias string
	cmd := &cobra.Command{
		Use:   "add <target-type> <target-id>",
		Short: "Add a comment (e.g. comment add issue IS-5 --body '...')",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			payload := map[string]any{
				"slug":        c.SlugOrDie(),
				"target_type": args[0],
				"target_id":   args[1],
				"body":        body,
			}
			if authorAlias != "" {
				payload["author_alias"] = authorAlias
			}
			var cm commentItem
			if err := c.post("/api/dx/comment/add", payload, &cm); err != nil {
				return err
			}
			fmt.Printf("C-%d added\n", cm.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment body")
	cmd.Flags().StringVar(&authorAlias, "as", "", "author alias (e.g. claude)")
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
			c := mustClient()

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
						var cm commentItem
						if err := c.get("/api/dx/comment/get", url.Values{"id": {strconv.Itoa(int(cid))}}, &cm); err != nil {
							return fmt.Errorf("C-%d: %w", cid, err)
						}
						var ok struct {
							OK bool `json:"ok"`
						}
						if err := c.post("/api/dx/comment/mark-read", map[string]any{
							"slug":        c.SlugOrDie(),
							"target_type": cm.TargetType,
							"target_id":   cm.TargetID,
							"role":        role,
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
			if err := c.post("/api/dx/comment/mark-read", map[string]any{
				"slug":        c.SlugOrDie(),
				"target_type": args[0],
				"target_id":   args[1],
				"role":        role,
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
			c := mustClient()
			cid, err := parseCommentID(args[0])
			if err != nil {
				return err
			}

			var orig commentItem
			if err := c.get("/api/dx/comment/get", url.Values{"id": {strconv.Itoa(int(cid))}}, &orig); err != nil {
				return fmt.Errorf("could not find C-%d: %w", cid, err)
			}

			if body != "" {
				payload := map[string]any{
					"slug":        c.SlugOrDie(),
					"target_type": orig.TargetType,
					"target_id":   orig.TargetID,
					"body":        body,
					"parent_id":   cid,
				}
				if authorAlias != "" {
					payload["author_alias"] = authorAlias
				}
				var cm commentItem
				if err := c.post("/api/dx/comment/add", payload, &cm); err != nil {
					return err
				}
				fmt.Printf("C-%d added (reply to C-%d)\n", cm.ID, cid)
			}

			if react != "" {
				var resp struct {
					ID int32 `json:"id"`
				}
				if err := c.post("/api/dx/comment/react", map[string]any{
					"slug":       c.SlugOrDie(),
					"comment_id": cid,
					"emoji":      react,
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
	cmd.Flags().StringVar(&authorAlias, "as", "", "author alias (e.g. claude)")
	return cmd
}

func commentReactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "react <C-N> <emoji>",
		Short: "React to a comment (e.g. comment react C-123 thumbs-up)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			cid, err := parseCommentID(args[0])
			if err != nil {
				return err
			}
			var resp struct {
				ID int32 `json:"id"`
			}
			if err := c.post("/api/dx/comment/react", map[string]any{
				"slug":       c.SlugOrDie(),
				"comment_id": cid,
				"emoji":      args[1],
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

// ── revision type for CLI use ─────────────────────────────────────────────

type revisionItem struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Field      string `json:"field"`
	OldVal     string `json:"old_val"`
	NewVal     string `json:"new_val"`
	Agent      string `json:"agent"`
	CreatedAt  string `json:"created_at"`
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
			c := mustClient()
			var resp struct {
				Revisions []revisionItem `json:"revisions"`
			}
			if err := c.get("/api/dx/revisions", url.Values{
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
