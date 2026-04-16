package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type blockerQuestionItem struct {
	ID         int32    `json:"id"`
	TargetType string   `json:"target_type"`
	TargetID   string   `json:"target_id"`
	Context    string   `json:"context"`
	Choices    []string `json:"choices"`
	Answer     string   `json:"answer"`
	AnsweredBy string   `json:"answered_by"`
	Status     string   `json:"status"`
	CreatedAt  string   `json:"created_at"`
	AnsweredAt string   `json:"answered_at"`
}

func QuestionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "question", Short: "Blocker questions requiring human decisions"}
	cmd.AddCommand(questionAddCmd(), questionAnswerCmd(), questionListCmd())
	return cmd
}

func questionAddCmd() *cobra.Command {
	var targetType, targetID, ctx, choices string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a blocker question",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			var choiceList []string
			if choices != "" {
				for _, ch := range strings.Split(choices, ",") {
					ch = strings.TrimSpace(ch)
					if ch != "" {
						choiceList = append(choiceList, ch)
					}
				}
			}
			var q blockerQuestionItem
			if err := c.Post("/api/dx/blocker-questions/add", map[string]any{
				"slug":        c.SlugOrDie(),
				"target_type": targetType,
				"target_id":   targetID,
				"context":     ctx,
				"choices":     choiceList,
			}, &q); err != nil {
				return err
			}
			fmt.Printf("BQ-%d  [%s:%s]  %s\n", q.ID, q.TargetType, q.TargetID, q.Context)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "target entity type (issue, task, feature)")
	cmd.Flags().StringVar(&targetID, "target-id", "", "target entity ID (e.g. IS-1, TK-5)")
	cmd.Flags().StringVar(&ctx, "context", "", "question context describing what needs a decision")
	cmd.Flags().StringVar(&choices, "choices", "", "comma-separated choices (optional)")
	cmd.MarkFlagRequired("target-type")
	cmd.MarkFlagRequired("target-id")
	cmd.MarkFlagRequired("context")
	return cmd
}

func questionAnswerCmd() *cobra.Command {
	var answer, answeredBy string
	cmd := &cobra.Command{
		Use:   "answer <ID>",
		Short: "Answer a blocker question",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}
			var q blockerQuestionItem
			if err := c.Post("/api/dx/blocker-questions/answer", map[string]any{
				"slug":        c.SlugOrDie(),
				"id":          int32(id),
				"answer":      answer,
				"answered_by": answeredBy,
			}, &q); err != nil {
				return err
			}
			fmt.Printf("BQ-%d  answered\n", q.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&answer, "answer", "", "answer text")
	cmd.Flags().StringVar(&answeredBy, "answered-by", "", "who answered (optional)")
	cmd.MarkFlagRequired("answer")
	return cmd
}

func questionListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List blocker questions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			var resp struct {
				Questions []blockerQuestionItem `json:"questions"`
			}
			params := url.Values{"slug": {c.SlugOrDie()}}
			if status != "" {
				params.Set("status", status)
			}
			if err := c.Get("/api/dx/blocker-questions/list", params, &resp); err != nil {
				return err
			}
			if len(resp.Questions) == 0 {
				fmt.Println("no blocker questions")
				return nil
			}
			for _, q := range resp.Questions {
				fmt.Printf("BQ-%-4d [%s:%s] %-10s  %s\n", q.ID, q.TargetType, q.TargetID, q.Status, q.Context)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (pending, answered)")
	return cmd
}
