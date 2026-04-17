package work

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

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
			c := cli.MustClient()
			resp, err := c.ListConstraintsWithResponse(cmd.Context(), &dxclient.ListConstraintsParams{
				Slug: c.SlugOrDie(),
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Constraints == nil || len(*resp.JSON200.Constraints) == 0 {
				fmt.Println("no constraints")
				return nil
			}
			for _, g := range *resp.JSON200.Constraints {
				fmt.Printf("%-4d [P%d] %-8s  %s\n", g.Id, g.Priority, g.Status, g.Title)
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
			c := cli.MustClient()
			resp, err := c.CreateConstraintWithResponse(cmd.Context(), dxclient.CreateConstraintRequest{
				Slug:        c.SlugOrDie(),
				Title:       args[0],
				Description: desc,
				Priority:    priority,
				Status:      status,
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
			fmt.Printf("%d  %s\n", resp.JSON200.Id, resp.JSON200.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&desc, "desc", "", "constraint description")
	cmd.Flags().Int32Var(&priority, "priority", 1, "priority (1-4)")
	cmd.Flags().StringVar(&status, "status", "active", "status (active/paused/archived)")
	return cmd
}
