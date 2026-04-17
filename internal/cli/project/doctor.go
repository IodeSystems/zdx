package project

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/doctor"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func DoctorCmd() *cobra.Command {
	var autoFix bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose project health and propose fixes",
		Long:  "Doctor checks the project against its maturity vine, auto-fixes what it can, and proposes actions for the rest. Deferred proposals don't nag until the next rung.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), autoFix)
		},
	}
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-apply fixable issues without prompting")
	return cmd
}

func runDoctor(ctx context.Context, autoFix bool) error {
	state := &doctor.ProjectState{
		Deferred: map[string]bool{},
	}

	// 1. Local detection
	doctor.DetectLocal(state)

	// 2. Remote state (if credentials exist)
	if state.CredentialsExist {
		populateRemoteState(ctx, state)
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
			_, _ = c.SetClassificationWithResponse(ctx, dxclient.SetClassificationRequest{
				Slug:           c.SlugOrDie(),
				Classification: string(class),
			})
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
					rung := f.Rung
					_, _ = c.DeferDoctorCheckWithResponse(ctx, dxclient.DeferDoctorCheckRequest{
						Slug:      c.SlugOrDie(),
						CheckName: f.Check.Name,
						Rung:      &rung,
					})
				}
				fmt.Printf("  deferred: %s\n", f.Check.Name)
			}
		}
	}

	return nil
}

func populateRemoteState(ctx context.Context, state *doctor.ProjectState) {
	c, err := cli.DefaultClient()
	if err != nil {
		return
	}
	slug := c.SlugOrDie()

	// Health check
	if hResp, err := c.HealthWithResponse(ctx); err == nil && hResp.JSON200 != nil {
		if status, ok := (*hResp.JSON200)["status"]; ok && status == "ok" {
			state.RemoteReachable = true
		}
	}
	if !state.RemoteReachable {
		return
	}

	// Classification
	if pResp, err := c.GetProjectInfoWithResponse(ctx, &dxclient.GetProjectInfoParams{Slug: slug}); err == nil && pResp.JSON200 != nil {
		state.Classification = doctor.Classification(pResp.JSON200.Classification)
	}

	// Goals
	if gResp, err := c.ListGoalsWithResponse(ctx, &dxclient.ListGoalsParams{Slug: slug}); err == nil && gResp.JSON200 != nil && gResp.JSON200.Goals != nil {
		for _, g := range *gResp.JSON200.Goals {
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
	if cResp, err := c.ListConstraintsWithResponse(ctx, &dxclient.ListConstraintsParams{Slug: slug}); err == nil && cResp.JSON200 != nil && cResp.JSON200.Constraints != nil {
		state.ConstraintCount = len(*cResp.JSON200.Constraints)
	}

	// Features
	if fResp, err := c.ListFeaturesWithResponse(ctx, &dxclient.ListFeaturesParams{Slug: slug}); err == nil && fResp.JSON200 != nil && fResp.JSON200.Features != nil {
		feats := *fResp.JSON200.Features
		state.FeatureCount = len(feats)
		for _, f := range feats {
			specCount := 0
			if f.Specs != nil {
				specCount = len(*f.Specs)
			}
			if specCount > 0 {
				state.FeaturesWithSpecs++
			}
			if f.GoalId > 0 || f.ParentFeatureId > 0 {
				state.FeaturesAttributed++
			}
			state.SpecsTotal += specCount
			if specCount > 8 {
				state.OverspeccedCount++
			}
		}
	}

	// Untriaged issues
	if iResp, err := c.ListIssuesWithResponse(ctx, &dxclient.ListIssuesParams{Slug: slug}); err == nil && iResp.JSON200 != nil && iResp.JSON200.Issues != nil {
		for _, iss := range *iResp.JSON200.Issues {
			if iss.Priority == "" && iss.Status != "closed" {
				state.UntriagedIssues++
			}
		}
	}

	// Stale agent sessions (still open past the sweeper's idle threshold)
	if sResp, err := c.ListStaleOpenClaudeSessionsWithResponse(ctx, &dxclient.ListStaleOpenClaudeSessionsParams{Slug: slug}); err == nil && sResp.JSON200 != nil {
		state.StaleAgentSessions = int(sResp.JSON200.Total)
		state.StaleAgentSessionsMinutes = sResp.JSON200.Minutes
	}

	// Deferrals
	if dResp, err := c.ListDoctorDeferralsWithResponse(ctx, &dxclient.ListDoctorDeferralsParams{Slug: slug}); err == nil && dResp.JSON200 != nil && dResp.JSON200.Deferrals != nil {
		for _, d := range *dResp.JSON200.Deferrals {
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
