package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ReviewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Task reviews"}
	cmd.AddCommand(reviewShowCmd())
	return cmd
}

func reviewShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <TK-N>",
		Short: "Show review record(s) for a task with their comment threads",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			id := args[0]
			if len(id) < 4 || id[:3] != "TK-" {
				return fmt.Errorf("expected TK-N, got %q", id)
			}
			c := cli.MustClient()
			slug := c.SlugOrDie()

			resp, err := c.ListTaskReviewsWithResponse(ctx, &dxclient.ListTaskReviewsParams{
				Slug:   slug,
				TaskId: id,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Reviews == nil || len(*resp.JSON200.Reviews) == 0 {
				fmt.Printf("No reviews on %s\n", id)
				fmt.Printf("Submit one: dx todo dev review %s --verdict=<approve|reject> [--body=\"...\"]\n", id)
				return nil
			}

			for _, r := range *resp.JSON200.Reviews {
				reviewer := "-"
				if r.ReviewerEmail != nil && *r.ReviewerEmail != "" {
					reviewer = *r.ReviewerEmail
				}
				fmt.Printf("Review #%d  task=%s  role=%s  verdict=%s  reviewer=%s  created=%s\n",
					r.Id, r.TaskId, r.ReviewerRole, valOrDash(r.Verdict),
					reviewer, r.CreatedAt)
				if r.Body != "" {
					fmt.Printf("  body: %s\n", r.Body)
				}

				targetType := "review"
				targetID := strconv.FormatInt(int64(r.Id), 10)
				cresp, cerr := c.ListCommentsWithResponse(ctx, &dxclient.ListCommentsParams{
					Slug:       &slug,
					TargetType: &targetType,
					TargetId:   &targetID,
				})
				if cerr != nil {
					return cerr
				}
				if cresp.JSON200 != nil && cresp.JSON200.Comments != nil && len(*cresp.JSON200.Comments) > 0 {
					fmt.Println("  comments:")
					for _, cm := range *cresp.JSON200.Comments {
						alias := ""
						if cm.AuthorAlias != nil && *cm.AuthorAlias != "" {
							alias = " (" + *cm.AuthorAlias + ")"
						}
						fmt.Printf("    C-%d [%s] %s%s: %s\n", cm.Id, cm.CreatedAt, cm.Author, alias, cm.Body)
					}
				}
				fmt.Printf("  discuss: dx comment add review %d --body=\"...\"\n", r.Id)
			}
			return nil
		},
	}
}

func valOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
