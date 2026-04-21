package project

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func DiscussionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "discussion", Short: "Discussion management"}
	cmd.AddCommand(
		discussionStartCmd(),
		discussionListCmd(),
		discussionShowCmd(),
		discussionSendCmd(),
	)
	return cmd
}

func parseDiscussionID(s string) (int32, error) {
	s = strings.TrimPrefix(s, "DS-")
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid discussion ID: %s", s)
	}
	return int32(id), nil
}

func discussionIDStr(id int32) string {
	return fmt.Sprintf("DS-%d", id)
}

func discussionStartCmd() *cobra.Command {
	var title string
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new discussion",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			if title == "" {
				title = "New discussion"
			}
			resp, err := c.CreateDiscussionWithResponse(cmd.Context(), dxclient.CreateDiscussionRequest{
				Slug:  c.SlugOrDie(),
				Title: title,
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
			d := resp.JSON200
			fmt.Printf("%s  [%s]  %s\n", discussionIDStr(d.Id), d.Status, d.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "discussion title")
	return cmd
}

func discussionListCmd() *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discussions",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			params := &dxclient.ListDiscussionsParams{Slug: c.SlugOrDie()}
			if status != "" {
				params.Status = &status
			}
			resp, err := c.ListDiscussionsWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Discussions == nil {
				return nil
			}
			for _, d := range *resp.JSON200.Discussions {
				fmt.Printf("%-8s  %-8s  %-6d msg  %s\n",
					discussionIDStr(d.Id), d.Status, d.MessageCount, d.Title)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status (default: active)")
	return cmd
}

func discussionShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <DS-N>",
		Short: "Show discussion metadata and messages",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseDiscussionID(args[0])
			if err != nil {
				return err
			}
			slug := c.SlugOrDie()

			dresp, err := c.GetDiscussionWithResponse(cmd.Context(), id, &dxclient.GetDiscussionParams{Slug: slug})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(dresp.StatusCode(), dresp.Body); err != nil {
				return err
			}
			if dresp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			d := dresp.JSON200
			fmt.Printf("%s  [%s]  %s\n", discussionIDStr(d.Id), d.Status, d.Title)
			fmt.Printf("provider: %s  created: %s\n\n", d.Provider, d.CreatedAt)

			mresp, err := c.ListDiscussionMessagesWithResponse(cmd.Context(), id, &dxclient.ListDiscussionMessagesParams{Slug: slug})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(mresp.StatusCode(), mresp.Body); err != nil {
				return err
			}
			if mresp.JSON200 == nil || mresp.JSON200.Messages == nil {
				return nil
			}
			for _, m := range *mresp.JSON200.Messages {
				fmt.Printf("[%s]\n%s\n\n", m.Role, m.Content)
			}
			return nil
		},
	}
	return cmd
}

func discussionSendCmd() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "send <DS-N>",
		Short: "Send a message and stream the LLM response",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parseDiscussionID(args[0])
			if err != nil {
				return err
			}
			slug := c.SlugOrDie()

			path := fmt.Sprintf("/api/dx/discussions/%d/messages/send", id)
			body := map[string]string{"slug": slug, "message": message}

			var streamErr error
			err = c.PostStreamSSE(cmd.Context(), path, body, func(raw []byte) error {
				var ev struct {
					Delta     string `json:"delta,omitempty"`
					Error     string `json:"error,omitempty"`
					Done      bool   `json:"done,omitempty"`
					MessageID int32  `json:"message_id,omitempty"`
				}
				if jerr := json.Unmarshal(raw, &ev); jerr != nil {
					return nil
				}
				if ev.Error != "" {
					streamErr = fmt.Errorf("server error: %s", ev.Error)
					return streamErr
				}
				if ev.Delta != "" {
					fmt.Print(ev.Delta)
					_ = os.Stdout.Sync()
				}
				return nil
			})
			// Emit a trailing newline so the next shell prompt starts cleanly.
			fmt.Println()
			if streamErr != nil {
				return streamErr
			}
			return err
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "message to send")
	_ = cmd.MarkFlagRequired("message")
	return cmd
}
