package cli

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

type commentItem struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Author     string `json:"author"`
	Body       string `json:"body"`
	CreatedAt  string `json:"created_at"`
	Unread     *bool  `json:"unread,omitempty"`
}

func CommentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "comment", Short: "Comment on issues, tasks, and features"}
	cmd.AddCommand(commentListCmd(), commentAddCmd(), commentMarkReadCmd())
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
		fmt.Printf("%s[%s] %s: %s\n", dot, date, cm.Author, cm.Body)
	}
}

func commentAddCmd() *cobra.Command {
	var body string
	cmd := &cobra.Command{
		Use:   "add <target-type> <target-id>",
		Short: "Add a comment (e.g. comment add issue IS-5 --body '...')",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var cm commentItem
			if err := c.post("/api/dx/comment/add", map[string]any{
				"slug":        c.SlugOrDie(),
				"target_type": args[0],
				"target_id":   args[1],
				"body":        body,
			}, &cm); err != nil {
				return err
			}
			fmt.Printf("comment %d added\n", cm.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "comment body")
	cmd.MarkFlagRequired("body")
	return cmd
}

func commentMarkReadCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "mark-read <target-type> <target-id>",
		Short: "Mark all comments on a target as read for a role",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var ok struct{ OK bool `json:"ok"` }
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

