package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

func TaskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Task management"}
	cmd.AddCommand(taskReadyCmd())
	cmd.AddCommand(taskDeleteCmd())
	return cmd
}

func taskReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready <TK-N>",
		Short: "Promote a draft task from wip to ready",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			n, _ := strconv.ParseInt(args[0][3:], 10, 32)
			if err := c.Post("/api/dx/todo/task/ready", map[string]any{
				"id": int32(n),
			}, nil); err != nil {
				return err
			}
			fmt.Printf("%s promoted to ready\n", args[0])
			return nil
		},
	}
}

func taskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <TK-N>",
		Short: "Delete a draft task (only permitted while in wip state)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			if len(args[0]) < 4 || args[0][:3] != "TK-" {
				return fmt.Errorf("expected TK-N, got %q", args[0])
			}
			n, err := strconv.ParseInt(args[0][3:], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid task id %q: %w", args[0], err)
			}
			if err := c.Post("/api/dx/todo/task/delete", map[string]any{
				"id": int32(n),
			}, nil); err != nil {
				return err
			}
			fmt.Printf("%s deleted\n", args[0])
			return nil
		},
	}
}
