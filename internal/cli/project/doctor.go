package project

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
	"github.com/iodesystems/zdx-go/internal/doctor"
)

func DoctorCmd() *cobra.Command {
	var autoFix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose project health and propose fixes",
		Long:  "Doctor checks the project against its maturity vine, auto-fixes what it can, and proposes actions for the rest. Deferred proposals don't nag until the next rung.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(autoFix)
		},
	}
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-apply fixable issues without prompting")
	return cmd
}

func runDoctor(autoFix bool) error {
	state := &doctor.ProjectState{
		Deferred: map[string]bool{},
	}

	// 1. Local detection
	doctor.DetectLocal(state)

	// 2. Remote state (if credentials exist)
	if state.CredentialsExist {
		populateRemoteState(state)
	}

	// 3. If no classification, ask
	if state.Classification == "" {
		class, err := promptClassification()
		if err != nil {
			return err
		}
		state.Classification = class
		// Persist to server if connected
		if state.RemoteReachable {
			c := cli.MustClient()
			_ = c.Post("/api/dx/doctor/classify", map[string]any{
				"slug":           c.SlugOrDie(),
				"classification": string(class),
			}, nil)
		}
	}

	// 4. Run checks
	findings := doctor.Evaluate(state)
	sum := doctor.Summarize(findings)

	// 5. Print findings by rung
	currentRung := ""
	for _, f := range findings {
		if f.Rung != currentRung {
			currentRung = f.Rung
			fmt.Printf("\n── %s ──\n", currentRung)
		}
		icon := "  ✓"
		switch f.Status {
		case doctor.StatusFail:
			icon = "  ✗"
		case doctor.StatusDeferred:
			icon = "  ○"
		}
		fmt.Printf("%s  %s", icon, f.Check.Description)
		if f.Message != "" && f.Status != doctor.StatusPass {
			fmt.Printf("  — %s", f.Message)
		}
		fmt.Println()

		// Auto-fix
		if f.Status == doctor.StatusFail && f.FixFunc != nil {
			if autoFix {
				if err := f.FixFunc(); err != nil {
					fmt.Printf("      fix failed: %v\n", err)
				} else {
					fmt.Println("      → fixed")
				}
			} else {
				fmt.Println("      → auto-fixable (run with --fix)")
			}
		}

		// Proposal
		if f.Status == doctor.StatusFail && f.Proposal != "" {
			fmt.Printf("      → %s\n", f.Proposal)
		}
	}

	// 6. Summary
	fmt.Printf("\n%d checks: %d passed, %d failed (%d auto-fixable, %d proposals), %d deferred\n",
		sum.Total, sum.Passed, sum.Failed, sum.AutoFix, sum.Propose, sum.Deferred)

	// 7. Interactive defer prompt for remaining failures
	if sum.Failed > 0 && !autoFix {
		reader := bufio.NewReader(os.Stdin)
		for i := range findings {
			f := &findings[i]
			if f.Status != doctor.StatusFail || f.FixFunc != nil {
				continue
			}
			if f.Proposal == "" {
				continue
			}
			fmt.Printf("\nDefer '%s'? [y/N] ", f.Check.Name)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line == "y" || line == "yes" {
				if state.RemoteReachable {
					c := cli.MustClient()
					_ = c.Post("/api/dx/doctor/defer", map[string]any{
						"slug":       c.SlugOrDie(),
						"check_name": f.Check.Name,
						"rung":       f.Rung,
					}, nil)
				}
				fmt.Printf("  deferred: %s\n", f.Check.Name)
			}
		}
	}

	return nil
}

func populateRemoteState(state *doctor.ProjectState) {
	c, err := cli.DefaultClient()
	if err != nil {
		return
	}
	slug := c.SlugOrDie()

	// Health check
	var health struct {
		Status string `json:"status"`
	}
	if err := c.Get("/api/health", nil, &health); err == nil && health.Status == "ok" {
		state.RemoteReachable = true
	} else {
		return
	}

	// Classification
	var proj struct {
		Classification string `json:"classification"`
	}
	if err := c.Get("/api/dx/project/info", url.Values{"slug": {slug}}, &proj); err == nil {
		state.Classification = doctor.Classification(proj.Classification)
	}

	// Goals
	var goalResp struct {
		Goals []clitypes.GoalItem `json:"goals"`
	}
	if err := c.Get("/api/goals", url.Values{"slug": {slug}}, &goalResp); err == nil {
		for _, g := range goalResp.Goals {
			if g.Status == "archived" {
				continue
			}
			state.GoalsTotal++
			state.GoalCount++
			if g.MetricName != "" {
				state.GoalsQuantified++
			}
		}
	}

	// Constraints
	var conResp struct {
		Constraints []struct{ ID int32 } `json:"constraints"`
	}
	if err := c.Get("/api/constraints", url.Values{"slug": {slug}}, &conResp); err == nil {
		state.ConstraintCount = len(conResp.Constraints)
	}

	// Features
	var featResp struct {
		Features []clitypes.FeatureItem `json:"features"`
	}
	if err := c.Get("/api/features", url.Values{"slug": {slug}}, &featResp); err == nil {
		state.FeatureCount = len(featResp.Features)
		for _, f := range featResp.Features {
			if len(f.Specs) > 0 {
				state.FeaturesWithSpecs++
			}
			if f.GoalID > 0 || f.ParentFeatureID > 0 {
				state.FeaturesAttributed++
			}
			state.SpecsTotal += len(f.Specs)
			if len(f.Specs) > 8 {
				state.OverspeccedCount++
			}
		}
	}

	// Untriaged issues
	var issueResp struct {
		Issues []clitypes.IssueItem `json:"issues"`
	}
	if err := c.Get("/api/dx/todo/issue/list", url.Values{"slug": {slug}}, &issueResp); err == nil {
		for _, iss := range issueResp.Issues {
			if iss.Priority == "" {
				state.UntriagedIssues++
			}
		}
	}

	// Deferrals
	var defResp struct {
		Deferrals []struct {
			CheckName string `json:"check_name"`
		} `json:"deferrals"`
	}
	if err := c.Get("/api/dx/doctor/deferrals", url.Values{"slug": {slug}}, &defResp); err == nil {
		for _, d := range defResp.Deferrals {
			state.Deferred[d.CheckName] = true
		}
	}
}

func promptClassification() (doctor.Classification, error) {
	fmt.Println("What kind of project is this?")
	for i, c := range doctor.AllClassifications {
		fmt.Printf("  %d. %s\n", i+1, doctor.ClassificationLabel(c))
	}
	fmt.Print("\nChoice [1-5]: ")
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSpace(line)
	switch line {
	case "1":
		return doctor.ClassLibrary, nil
	case "2":
		return doctor.ClassTool, nil
	case "3":
		return doctor.ClassService, nil
	case "4":
		return doctor.ClassSaaS, nil
	case "5":
		return doctor.ClassSite, nil
	default:
		// Try matching by name
		for _, c := range doctor.AllClassifications {
			if strings.EqualFold(line, string(c)) {
				return c, nil
			}
		}
		return doctor.ClassTool, nil // default
	}
}
