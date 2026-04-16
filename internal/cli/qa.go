package cli

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"
)

type QuestionItem struct {
	ID        int32  `json:"id"`
	Category  string `json:"category"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func QaCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "qa", Short: "Q&A support portal"}
	cmd.AddCommand(qaAddCmd(), qaAnswerCmd(), qaListCmd())
	return cmd
}

func qaAddCmd() *cobra.Command {
	var category, question string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a question",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			var q QuestionItem
			if err := c.Post("/api/dx/qa/add", map[string]any{
				"slug":     c.SlugOrDie(),
				"category": category,
				"question": question,
			}, &q); err != nil {
				return err
			}
			fmt.Printf("QA-%d  [%s]  %s\n", q.ID, q.Category, q.Question)
			return nil
		},
	}
	cmd.Flags().StringVar(&category, "category", "", "category label")
	cmd.Flags().StringVar(&question, "question", "", "question text")
	cmd.MarkFlagRequired("question")
	return cmd
}

func qaAnswerCmd() *cobra.Command {
	var answer string
	cmd := &cobra.Command{
		Use:   "answer <ID>",
		Short: "Answer a question",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}
			var q QuestionItem
			if err := c.Post("/api/dx/qa/answer", map[string]any{
				"slug":   c.SlugOrDie(),
				"id":     int32(id),
				"answer": answer,
			}, &q); err != nil {
				return err
			}
			fmt.Printf("QA-%d  answered\n", q.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&answer, "answer", "", "answer text")
	cmd.MarkFlagRequired("answer")
	return cmd
}

func qaListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List questions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			var resp struct {
				Questions []QuestionItem `json:"questions"`
			}
			if err := c.Get("/api/dx/qa/list", url.Values{"slug": {c.SlugOrDie()}}, &resp); err != nil {
				return err
			}
			if len(resp.Questions) == 0 {
				fmt.Println("no questions")
				return nil
			}
			for _, q := range resp.Questions {
				answered := "unanswered"
				if q.Answer != "" {
					answered = "answered"
				}
				fmt.Printf("QA-%-4d [%s] %-10s  %s\n", q.ID, q.Category, answered, q.Question)
			}
			return nil
		},
	}
}
