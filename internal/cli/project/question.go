package project

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func QuestionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "question", Short: "Blocker questions requiring human decisions"}
	cmd.AddCommand(questionAddCmd(), questionAnswerCmd(), questionListCmd())
	return cmd
}

func questionAddCmd() *cobra.Command {
	var targetType, targetID, ctx, choices string
	var choiceArgs []string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Create a blocker question",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var choiceList []string
			for _, ch := range choiceArgs {
				ch = strings.TrimSpace(ch)
				if ch != "" {
					choiceList = append(choiceList, ch)
				}
			}
			if choices != "" {
				for _, ch := range strings.Split(choices, ",") {
					ch = strings.TrimSpace(ch)
					if ch != "" {
						choiceList = append(choiceList, ch)
					}
				}
			}
			body := dxclient.AddBlockerQuestionRequest{
				Slug:       c.SlugOrDie(),
				TargetType: targetType,
				TargetId:   targetID,
				Context:    ctx,
			}
			if len(choiceList) > 0 {
				body.Choices = &choiceList
			}
			resp, err := c.AddBlockerQuestionWithResponse(cmd.Context(), body)
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
			fmt.Printf("BQ-%d  [%s:%s]  %s\n", q.Id, q.TargetType, q.TargetId, q.Context)
			return nil
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "", "target entity type (issue, task, feature)")
	cmd.Flags().StringVar(&targetID, "target-id", "", "target entity ID (e.g. IS-1, TK-5)")
	cmd.Flags().StringVar(&ctx, "context", "", "question context describing what needs a decision")
	cmd.Flags().StringVar(&choices, "choices", "", "comma-separated choices (optional; commas inside options are not supported — use --choice instead)")
	cmd.Flags().StringArrayVarP(&choiceArgs, "choice", "C", nil, "single choice; repeat for multiple (safe for options containing commas)")
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
			c := cli.MustClient()
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid ID: %s", args[0])
			}
			body := dxclient.AnswerBlockerQuestionRequest{
				Slug:   c.SlugOrDie(),
				Id:     int32(id),
				Answer: answer,
			}
			if answeredBy != "" {
				body.AnsweredBy = &answeredBy
			}
			resp, err := c.AnswerBlockerQuestionWithResponse(cmd.Context(), body)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("BQ-%d  answered\n", resp.JSON200.Id)
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
			c := cli.MustClient()
			params := &dxclient.ListBlockerQuestionsParams{Slug: c.SlugOrDie()}
			if status != "" {
				params.Status = &status
			}
			resp, err := c.ListBlockerQuestionsWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Questions == nil || len(*resp.JSON200.Questions) == 0 {
				fmt.Println("no blocker questions")
				return nil
			}
			for _, q := range *resp.JSON200.Questions {
				fmt.Printf("BQ-%-4d [%s:%s] %-10s  %s\n", q.Id, q.TargetType, q.TargetId, q.Status, q.Context)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (pending, answered)")
	return cmd
}
