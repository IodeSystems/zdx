package cli

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

func TaskCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "task", Short: "Task management"}
	cmd.AddCommand(taskReadyCmd())
	return cmd
}

func taskReadyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ready <TK-N>",
		Short: "Promote a draft task from wip to pending",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			n, _ := strconv.ParseInt(args[0][3:], 10, 32)
			if err := c.post("/api/dx/todo/task/ready", map[string]any{
				"id": int32(n),
			}, nil); err != nil {
				return err
			}
			fmt.Printf("%s promoted to pending\n", args[0])
			return nil
		},
	}
}
