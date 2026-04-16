package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type constraintItem struct {
	ID          int32  `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func ConstraintCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "constraint", Short: "Project constraint management"}
	cmd.AddCommand(
		constraintListCmd(),
		constraintAddCmd(),
	)
	return cmd
}

func constraintListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List project constraints",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			var resp struct {
				Constraints []constraintItem `json:"constraints"`
			}
			if err := c.Get("/api/constraints", QuerySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Constraints) == 0 {
				fmt.Println("no constraints")
				return nil
			}
			for _, g := range resp.Constraints {
				fmt.Printf("%-4d [P%d] %-8s  %s\n", g.ID, g.Priority, g.Status, g.Title)
			}
			return nil
		},
	}
}

func constraintAddCmd() *cobra.Command {
	var desc string
	var priority int32
	var status string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a project constraint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := MustClient()
			var g constraintItem
			if err := c.Post("/api/constraint", map[string]any{
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
	cmd.Flags().StringVar(&desc, "desc", "", "constraint description")
	cmd.Flags().Int32Var(&priority, "priority", 1, "priority (1-4)")
	cmd.Flags().StringVar(&status, "status", "active", "status (active/paused/archived)")
	return cmd
}
