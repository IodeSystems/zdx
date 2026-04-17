package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

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
			c := cli.MustClient()
			resp, err := c.AddQuestionWithResponse(cmd.Context(), dxclient.AddQuestionRequest{
				Slug:     c.SlugOrDie(),
				Category: category,
				Question: question,
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
			q := resp.JSON200
			fmt.Printf("QA-%d  [%s]  %s\n", q.Id, q.Category, q.Question)
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
			c := cli.MustClient()
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}
			resp, err := c.AnswerQuestionWithResponse(cmd.Context(), dxclient.AnswerQuestionRequest{
				Slug:   c.SlugOrDie(),
				Id:     int32(id),
				Answer: answer,
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
			fmt.Printf("QA-%d  answered\n", resp.JSON200.Id)
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
			c := cli.MustClient()
			resp, err := c.ListQuestionsWithResponse(cmd.Context(), &dxclient.ListQuestionsParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Questions == nil || len(*resp.JSON200.Questions) == 0 {
				fmt.Println("no questions")
				return nil
			}
			for _, q := range *resp.JSON200.Questions {
				answered := "unanswered"
				if q.Answer != "" {
					answered = "answered"
				}
				fmt.Printf("QA-%-4d [%s] %-10s  %s\n", q.Id, q.Category, answered, q.Question)
			}
			return nil
		},
	}
}
