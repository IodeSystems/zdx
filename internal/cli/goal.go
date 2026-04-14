package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type goalItem struct {
	ID          int32  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func GoalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "goal", Short: "Project goal management"}
	cmd.AddCommand(
		goalListCmd(),
		goalAddCmd(),
	)
	return cmd
}

func goalListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List project goals",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var resp struct {
				Goals []goalItem `json:"goals"`
			}
			if err := c.get("/api/goals", querySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Goals) == 0 {
				fmt.Println("no goals")
				return nil
			}
			for _, g := range resp.Goals {
				fmt.Printf("%-4d [P%d] %-8s  %s\n", g.ID, g.Priority, g.Status, g.Title)
			}
			return nil
		},
	}
}

func goalAddCmd() *cobra.Command {
	var desc string
	var priority int32
	var status string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a project goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			var g goalItem
			if err := c.post("/api/goal", map[string]any{
				"slug":        c.SlugOrDie(),
				"title":       args[0],
				"description": desc,
				"priority":    priority,
				"status":      status,
			}, &g); err != nil {
				return err
			}
			fmt.Printf("%d  %s\n", g.ID, g.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "goal description")
	cmd.Flags().Int32Var(&priority, "priority", 1, "priority (1-4)")
	cmd.Flags().StringVar(&status, "status", "active", "status (active/paused/archived)")
	return cmd
}
