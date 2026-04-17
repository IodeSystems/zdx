package project

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

type planItem struct {
	ID         int32          `json:"id"`
	Title      string         `json:"title"`
	Body       string         `json:"body"`
	PlanType   string         `json:"plan_type"`
	Status     string         `json:"status"`
	Complexity string         `json:"complexity"`
	Approach   string         `json:"approach"`
	FeatureID  int32          `json:"feature_id"`
	FocusID    int32          `json:"focus_id"`
	IssueID    string         `json:"issue_id"`
	CreatedAt  string         `json:"created_at"`
	UpdatedAt  string         `json:"updated_at"`
	Steps      []planStepItem `json:"steps"`
}

type planStepItem struct {
	ID        int32             `json:"id"`
	PlanID    int32             `json:"plan_id"`
	Seq       int32             `json:"seq"`
	Text      string            `json:"text"`
	Status    string            `json:"status"`
	DependsOn int32             `json:"depends_on"`
	Refs      []planStepRefItem `json:"refs"`
}

type planStepRefItem struct {
	StepID     int32  `json:"step_id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
}

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
			var resp struct {
				Plans []planItem `json:"plans"`
			}
			if err := c.Get("/api/dx/plans", cli.QuerySlug(c), &resp); err != nil {
				return err
			}
			if len(resp.Plans) == 0 {
				fmt.Println("no plans")
				return nil
			}
			for _, p := range resp.Plans {
				anchor := ""
				if p.FocusID > 0 {
					anchor = fmt.Sprintf("  focus:FO-%d", p.FocusID)
				} else if p.FeatureID > 0 {
					anchor = fmt.Sprintf("  feature:%d", p.FeatureID)
				} else if p.IssueID != "" {
					anchor = fmt.Sprintf("  issue:%s", p.IssueID)
				}
				fmt.Printf("PL-%-4d %-12s %-10s %s%s\n", p.ID, p.Status, p.PlanType, p.Title, anchor)
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
			var p planItem
			q := url.Values{"slug": {c.SlugOrDie()}, "id": {strconv.Itoa(int(id))}}
			if err := c.Get("/api/dx/plan", q, &p); err != nil {
				return err
			}
			fmt.Printf("PL-%d  %s\n", p.ID, p.Title)
			fmt.Printf("Status:  %s   Type: %s\n", p.Status, p.PlanType)
			if p.Body != "" {
				fmt.Printf("\n%s\n", p.Body)
			}
			if len(p.Steps) > 0 {
				fmt.Printf("\nSteps (%d):\n", len(p.Steps))
				for _, s := range p.Steps {
					dep := ""
					if s.DependsOn > 0 {
						dep = fmt.Sprintf("  (after step %d)", s.DependsOn)
					}
					refs := ""
					for _, r := range s.Refs {
						refs += fmt.Sprintf(" → %s:%s", r.TargetType, r.TargetID)
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
			req := map[string]any{
				"slug":      c.SlugOrDie(),
				"title":     args[0],
				"body":      body,
				"plan_type": planType,
			}
			if featureName != "" {
				req["feature_name"] = featureName
			}
			if focusID > 0 {
				req["focus_id"] = focusID
			}
			if issueID != "" {
				req["issue_id"] = issueID
			}
			var p planItem
			if err := c.Post("/api/dx/plan/add", req, &p); err != nil {
				return err
			}
			fmt.Printf("PL-%d  %s\n", p.ID, p.Title)
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
			req := map[string]any{
				"plan_id": planID,
				"text":    args[1],
				"seq":     seq,
			}
			if dependsOn > 0 {
				req["depends_on"] = dependsOn
			}
			var s planStepItem
			if err := c.Post("/api/dx/plan/step/add", req, &s); err != nil {
				return err
			}
			fmt.Printf("step %d added (seq %d)\n", s.ID, s.Seq)
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
			req := map[string]any{"id": int32(stepID)}
			if cmd.Flags().Changed("text") {
				req["text"] = text
			}
			if cmd.Flags().Changed("status") {
				req["status"] = status
			}
			if cmd.Flags().Changed("depends-on") {
				req["depends_on"] = dependsOn
			}
			if err := c.Post("/api/dx/plan/step/update", req, nil); err != nil {
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
			if err := c.Post("/api/dx/plan/step/ref/add", map[string]any{
				"step_id":     int32(stepID),
				"target_type": args[1],
				"target_id":   args[2],
			}, nil); err != nil {
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
