package project

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func PlanCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "plan", Short: "Plan management"}
	cmd.AddCommand(
		planListCmd(),
		planShowCmd(),
		planAddCmd(),
		planStepCmd(),
	)
	return cmd
}

func planListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List plans",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListPlansWithResponse(cmd.Context(), &dxclient.ListPlansParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Plans == nil || len(*resp.JSON200.Plans) == 0 {
				fmt.Println("no plans")
				return nil
			}
			for _, p := range *resp.JSON200.Plans {
				anchor := ""
				if p.FocusId > 0 {
					anchor = fmt.Sprintf("  focus:FO-%d", p.FocusId)
				} else if p.FeatureId > 0 {
					anchor = fmt.Sprintf("  feature:%d", p.FeatureId)
				} else if p.IssueId != "" {
					anchor = fmt.Sprintf("  issue:%s", p.IssueId)
				}
				fmt.Printf("PL-%-4d %-12s %-10s %s%s\n", p.Id, p.Status, p.PlanType, p.Title, anchor)
			}
			return nil
		},
	}
}

func planShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <PL-N>",
		Short: "Show plan detail with steps",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id, err := parsePlanID(args[0])
			if err != nil {
				return err
			}
			resp, err := c.GetPlanWithResponse(cmd.Context(), &dxclient.GetPlanParams{
				Slug: c.SlugOrDie(),
				Id:   id,
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
			p := resp.JSON200
			fmt.Printf("PL-%d  %s\n", p.Id, p.Title)
			fmt.Printf("Status:  %s   Type: %s\n", p.Status, p.PlanType)
			if p.Body != "" {
				fmt.Printf("\n%s\n", p.Body)
			}
			if p.Steps != nil && len(*p.Steps) > 0 {
				fmt.Printf("\nSteps (%d):\n", len(*p.Steps))
				for _, s := range *p.Steps {
					dep := ""
					if s.DependsOn > 0 {
						dep = fmt.Sprintf("  (after step %d)", s.DependsOn)
					}
					refs := ""
					if s.Refs != nil {
						for _, r := range *s.Refs {
							refs += fmt.Sprintf(" → %s:%s", r.TargetType, r.TargetId)
						}
					}
					fmt.Printf("  %d. [%-10s] %s%s%s\n", s.Seq, s.Status, s.Text, dep, refs)
				}
			}
			return nil
		},
	}
}

func planAddCmd() *cobra.Command {
	var body, planType, featureName, issueID string
	var focusID int32
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Create a plan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			req := dxclient.AddPlanRequest{
				Slug:  c.SlugOrDie(),
				Title: args[0],
			}
			if body != "" {
				req.Body = &body
			}
			if planType != "" {
				req.PlanType = &planType
			}
			if featureName != "" {
				req.FeatureName = &featureName
			}
			if focusID > 0 {
				req.FocusId = &focusID
			}
			if issueID != "" {
				req.IssueId = &issueID
			}
			resp, err := c.AddPlanWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("PL-%d  %s\n", resp.JSON200.Id, resp.JSON200.Title)
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "plan body (markdown)")
	cmd.Flags().StringVar(&planType, "type", "implement", "plan type")
	cmd.Flags().StringVar(&featureName, "feature", "", "anchor to feature by name")
	cmd.Flags().Int32Var(&focusID, "focus", 0, "anchor to focus by ID (FO-N)")
	cmd.Flags().StringVar(&issueID, "issue", "", "anchor to issue (IS-N)")
	return cmd
}

func planStepCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "step", Short: "Plan step operations"}
	cmd.AddCommand(planStepAddCmd(), planStepUpdateCmd(), planStepRefAddCmd())
	return cmd
}

func planStepAddCmd() *cobra.Command {
	var seq int32
	var dependsOn int32
	cmd := &cobra.Command{
		Use:   "add <PL-N> <text>",
		Short: "Add a step to a plan",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			planID, err := parsePlanID(args[0])
			if err != nil {
				return err
			}
			req := dxclient.AddPlanStepRequest{
				PlanId: planID,
				Text:   args[1],
				Seq:    &seq,
			}
			if dependsOn > 0 {
				req.DependsOn = &dependsOn
			}
			resp, err := c.AddPlanStepWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Printf("step %d added (seq %d)\n", resp.JSON200.Id, resp.JSON200.Seq)
			return nil
		},
	}
	cmd.Flags().Int32Var(&seq, "seq", 0, "step sequence number")
	cmd.Flags().Int32Var(&dependsOn, "depends-on", 0, "step ID this depends on")
	return cmd
}

func planStepUpdateCmd() *cobra.Command {
	var text, status string
	var dependsOn int32
	cmd := &cobra.Command{
		Use:   "update <step-id>",
		Short: "Update a plan step (status, text)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			stepID, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid step id: %s", args[0])
			}
			req := dxclient.UpdatePlanStepRequest{Id: int32(stepID)}
			if cmd.Flags().Changed("text") {
				req.Text = text
			}
			if cmd.Flags().Changed("status") {
				req.Status = status
			}
			if cmd.Flags().Changed("depends-on") {
				req.DependsOn = dependsOn
			}
			resp, err := c.UpdatePlanStepWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("step %d updated\n", stepID)
			return nil
		},
	}
	cmd.Flags().StringVar(&text, "text", "", "step text")
	cmd.Flags().StringVar(&status, "status", "", "step status (pending/in-progress/done/blocked/abandoned)")
	cmd.Flags().Int32Var(&dependsOn, "depends-on", 0, "step ID this depends on")
	return cmd
}

func planStepRefAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ref <step-id> <target-type> <target-id>",
		Short: "Link a step to a spawned issue/feature/task",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			stepID, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("invalid step id: %s", args[0])
			}
			resp, err := c.AddPlanStepRefWithResponse(cmd.Context(), dxclient.AddPlanStepRefRequest{
				StepId:     int32(stepID),
				TargetType: args[1],
				TargetId:   args[2],
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			fmt.Printf("ref added: step %d → %s:%s\n", stepID, args[1], args[2])
			return nil
		},
	}
}

func parsePlanID(ref string) (int32, error) {
	s := ref
	if len(s) > 3 && (s[:3] == "PL-" || s[:3] == "pl-") {
		s = s[3:]
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid plan ID: %s", ref)
	}
	return int32(n), nil
}
