package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func GoalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "goal", Short: "Project goal management"}
	cmd.AddCommand(
		goalListCmd(),
		goalAddCmd(),
		goalSetCmd(),
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
		Long: `Add a project goal — a measurable outcome the project is working toward.

A good goal is outcome-oriented and, where possible, measurable:
  - "Reduce API p99 latency below 200ms" (metric: latency, unit: ms)
  - "Achieve 99.9% uptime over rolling 30 days" (metric: uptime, unit: percent)
  - "Onboard 50 paying customers by Q3" (metric: customers, unit: count)

Use --metric-name + --metric-unit to make progress trackable. Goals without
metrics are allowed but encourage adding them once the measure is known.

Example:
  dx goal add "Reduce cold-start time below 500ms" --metric-name=cold_start --metric-unit=ms`,
		Args: cobra.ExactArgs(1),
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

func goalSetCmd() *cobra.Command {
	var title, desc, status, metricName, metricUnit string
	var priority int32
	cmd := &cobra.Command{
		Use:   "set <id>",
		Short: "Update fields on a goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id64, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid goal id: %s", args[0])
			}
			id := int32(id64)
			c := cli.MustClient()
			slug := c.SlugOrDie()

			resp, err := c.ListGoalsWithResponse(cmd.Context(), &dxclient.ListGoalsParams{Slug: slug})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Goals == nil {
				return fmt.Errorf("no goals found")
			}
			var goal *dxclient.GoalItem
			for i := range *resp.JSON200.Goals {
				g := (*resp.JSON200.Goals)[i]
				if g.Id == id {
					goal = &g
					break
				}
			}
			if goal == nil {
				return fmt.Errorf("goal %d not found", id)
			}

			req := dxclient.UpdateGoalRequest{
				Id:          goal.Id,
				Title:       goal.Title,
				Description: goal.Description,
				Priority:    goal.Priority,
				Status:      goal.Status,
			}
			if mn := goal.MetricName; mn != "" {
				req.MetricName = &mn
			}
			if mu := goal.MetricUnit; mu != "" {
				req.MetricUnit = &mu
			}
			if cmd.Flags().Changed("title") {
				req.Title = title
			}
			if cmd.Flags().Changed("desc") {
				req.Description = desc
			}
			if cmd.Flags().Changed("status") {
				req.Status = status
			}
			if cmd.Flags().Changed("priority") {
				req.Priority = priority
			}
			if cmd.Flags().Changed("metric-name") {
				req.MetricName = &metricName
			}
			if cmd.Flags().Changed("metric-unit") {
				req.MetricUnit = &metricUnit
			}

			updResp, err := c.UpdateGoalWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(updResp.StatusCode(), updResp.Body); err != nil {
				return err
			}
			fmt.Printf("goal %d updated\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "goal title")
	cmd.Flags().StringVar(&desc, "desc", "", "goal description")
	cmd.Flags().StringVar(&status, "status", "", "status (active/paused/archived)")
	cmd.Flags().Int32Var(&priority, "priority", 0, "priority (1-4)")
	cmd.Flags().StringVar(&metricName, "metric-name", "", "measured metric name")
	cmd.Flags().StringVar(&metricUnit, "metric-unit", "", "metric unit (e.g., seconds, percent)")
	return cmd
}
