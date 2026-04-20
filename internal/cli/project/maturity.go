package project

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func MaturityCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "maturity", Short: "Maturity item / questionnaire management"}
	cmd.AddCommand(
		maturityListCmd(),
		maturityShowCmd(),
		maturityDoneCmd(),
		maturitySnoozeCmd(),
		maturityDismissCmd(),
		maturityQuestionsCmd(),
		maturityAnswerCmd(),
	)
	return cmd
}

func maturityListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List maturity items",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := &dxclient.ListMaturityItemsParams{Slug: c.SlugOrDie()}
			if status != "all" {
				params.Status = &status
			}
			resp, err := c.ListMaturityItemsWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Items == nil || len(*resp.JSON200.Items) == 0 {
				fmt.Println("no maturity items")
				return nil
			}
			for _, it := range *resp.JSON200.Items {
				fmt.Printf("%-5d  %-9s  p%-3d  %-28s  %s\n",
					it.Id, it.Status, it.PriorityHint, it.Kind, it.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "open", "filter: open|done|snoozed|dismissed|all")
	return cmd
}

func maturityShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <item-id>",
		Short: "Show maturity item detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.GetMaturityItemWithResponse(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			it := resp.JSON200
			fmt.Printf("ID:              %d\n", it.Id)
			fmt.Printf("Title:           %s\n", it.Title)
			fmt.Printf("Status:          %s\n", it.Status)
			fmt.Printf("Kind:            %s\n", it.Kind)
			fmt.Printf("Priority:        %d\n", it.PriorityHint)
			if it.TargetType != "" {
				target := it.TargetType
				if it.TargetId > 0 {
					target = fmt.Sprintf("%s:%d", it.TargetType, it.TargetId)
				}
				fmt.Printf("Target:          %s\n", target)
			}
			if it.SourceQuestion != "" {
				fmt.Printf("Source question: %s\n", it.SourceQuestion)
			}
			if it.Description != "" {
				fmt.Printf("\nDescription:\n%s\n", it.Description)
			}
			if it.Justification != "" {
				fmt.Printf("\nJustification:\n%s\n", it.Justification)
			}
			if it.SnoozeUntil != "" {
				fmt.Printf("\nSnooze until:    %s\n", it.SnoozeUntil)
			}
			fmt.Printf("\nCreated:         %s\n", it.CreatedAt)
			fmt.Printf("Updated:         %s\n", it.UpdatedAt)
			return nil
		},
	}
}

func maturityDoneCmd() *cobra.Command {
	var justification string
	cmd := &cobra.Command{
		Use:   "done <item-id>",
		Short: "Mark a maturity item done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateMaturityStatus(cmd, args[0], "done", justification, "")
		},
	}
	cmd.Flags().StringVar(&justification, "justification", "", "what was done / why it is complete")
	cmd.MarkFlagRequired("justification")
	return cmd
}

func maturitySnoozeCmd() *cobra.Command {
	var until string
	cmd := &cobra.Command{
		Use:   "snooze <item-id>",
		Short: "Snooze a maturity item until a given date",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ts, err := parseSnoozeUntil(until)
			if err != nil {
				return err
			}
			return updateMaturityStatus(cmd, args[0], "snoozed", "", ts)
		},
	}
	cmd.Flags().StringVar(&until, "until", "", "snooze until date (YYYY-MM-DD or RFC3339)")
	cmd.MarkFlagRequired("until")
	return cmd
}

func maturityDismissCmd() *cobra.Command {
	var reason string
	cmd := &cobra.Command{
		Use:   "dismiss <item-id>",
		Short: "Dismiss a maturity item as not applicable",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return updateMaturityStatus(cmd, args[0], "dismissed", reason, "")
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "why this item is not applicable")
	cmd.MarkFlagRequired("reason")
	return cmd
}

func updateMaturityStatus(cmd *cobra.Command, id, status, justification, snoozeUntil string) error {
	c := cli.MustClient()
	body := dxclient.UpdateMaturityItemRequest{Status: status}
	if justification != "" {
		body.Justification = &justification
	}
	if snoozeUntil != "" {
		body.SnoozeUntil = &snoozeUntil
	}
	resp, err := c.UpdateMaturityItemWithResponse(cmd.Context(), id, body)
	if err != nil {
		return err
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("empty response")
	}
	fmt.Printf("%d → %s\n", resp.JSON200.Id, resp.JSON200.Status)
	return nil
}

func parseSnoozeUntil(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("--until is required")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format(time.RFC3339), nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	return "", fmt.Errorf("--until must be YYYY-MM-DD or RFC3339, got %q", s)
}

func maturityQuestionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "questions",
		Short: "List maturity questions with current answers",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListMaturityQuestionsWithResponse(cmd.Context(), &dxclient.ListMaturityQuestionsParams{
				Slug: c.SlugOrDie(),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Questions == nil || len(*resp.JSON200.Questions) == 0 {
				fmt.Println("no maturity questions")
				return nil
			}
			for _, q := range *resp.JSON200.Questions {
				ans := q.Answer
				if ans == "" {
					ans = "-"
				}
				fmt.Printf("%-28s  %-8s  %s\n", q.Key, ans, q.Prompt)
			}
			return nil
		},
	}
}

func maturityAnswerCmd() *cobra.Command {
	var answeredBy string
	cmd := &cobra.Command{
		Use:   "answer <question-key> <yes|no>",
		Short: "Record an answer and stamp resulting maturity items",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := args[0]
			answer := strings.ToLower(args[1])
			c := cli.MustClient()
			resp, err := c.SubmitMaturityAnswerWithResponse(cmd.Context(),
				&dxclient.SubmitMaturityAnswerParams{Slug: c.SlugOrDie()},
				dxclient.SubmitMaturityAnswerRequest{
					QuestionKey: key,
					Answer:      answer,
					AnsweredBy:  answeredBy,
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
			fmt.Printf("%s = %s\n", key, answer)
			items := resp.JSON200.Items
			if items == nil || len(*items) == 0 {
				fmt.Println("no new items stamped")
				return nil
			}
			fmt.Printf("stamped %d item(s):\n", len(*items))
			for _, it := range *items {
				fmt.Printf("  %-5d  %-28s  %s\n", it.Id, it.Kind, it.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&answeredBy, "as", "", "record answer as this author (defaults to server-detected identity)")
	return cmd
}
