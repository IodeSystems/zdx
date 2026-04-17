package project

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

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
			c := cli.MustClient()
			resp, err := c.ListGoalsWithResponse(cmd.Context(), &dxclient.ListGoalsParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Goals == nil || len(*resp.JSON200.Goals) == 0 {
				fmt.Println("no goals")
				return nil
			}
			for _, g := range *resp.JSON200.Goals {
				metric := ""
				if g.MetricName != "" {
					metric = fmt.Sprintf("  metric:%s(%s)", g.MetricName, g.MetricUnit)
				}
				fmt.Printf("%-4d [P%d] %-8s  %s%s\n", g.Id, g.Priority, g.Status, g.Title, metric)
			}
			return nil
		},
	}
}

func goalAddCmd() *cobra.Command {
	var desc string
	var priority int32
	var status, metricName, metricUnit string
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Add a project goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			req := dxclient.CreateGoalRequest{
				Slug:        c.SlugOrDie(),
				Title:       args[0],
				Description: desc,
				Priority:    priority,
				Status:      status,
			}
			if metricName != "" {
				req.MetricName = &metricName
			}
			if metricUnit != "" {
				req.MetricUnit = &metricUnit
			}
			resp, err := c.CreateGoalWithResponse(cmd.Context(), req)
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
	cmd.Flags().StringVar(&desc, "desc", "", "goal description")
	cmd.Flags().Int32Var(&priority, "priority", 1, "priority (1-4)")
	cmd.Flags().StringVar(&status, "status", "active", "status (active/paused/archived)")
	cmd.Flags().StringVar(&metricName, "metric-name", "", "measured metric name")
	cmd.Flags().StringVar(&metricUnit, "metric-unit", "", "metric unit (e.g., seconds, percent)")
	return cmd
}
