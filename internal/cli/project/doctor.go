package project

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/doctor"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func DoctorCmd() *cobra.Command {
	var autoFix bool
	var reQuestionnaire bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose project health and propose fixes",
		Long:  "Doctor checks the project against its maturity vine, auto-fixes what it can, and proposes actions for the rest. Deferred proposals don't nag until the next rung.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), autoFix, reQuestionnaire)
		},
	}
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-apply fixable issues without prompting")
	cmd.Flags().BoolVar(&reQuestionnaire, "re-questionnaire", false, "re-prompt already-answered maturity questions")
	return cmd
}

func runDoctor(ctx context.Context, autoFix bool, reQuestionnaire bool) error {
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
		class, err := promptClassification(os.Stdin)
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

	// 4. Walk unanswered maturity questions
	if state.RemoteReachable && len(state.MaturityQuestions) > 0 {
		c := cli.MustClient()
		slug := c.SlugOrDie()
		reader := bufio.NewReader(os.Stdin)
		classification := string(state.Classification)
		for _, q := range state.MaturityQuestions {
			if !reQuestionnaire && q.Answer != "" {
				continue
			}
			// Check applicable classifications
			applicable := false
			if q.ApplicableClassifications != nil {
				for _, ac := range *q.ApplicableClassifications {
					if ac == classification {
						applicable = true
						break
					}
				}
			} else {
				applicable = true
			}
			if !applicable {
				continue
			}
			fmt.Printf("\nMaturity question: %s [yes/no/skip]: ", q.Prompt)
			line, _ := reader.ReadString('\n')
			line = strings.TrimSpace(strings.ToLower(line))
			if line == "skip" || line == "s" {
				continue
			}
			answer := ""
			switch line {
			case "yes", "y":
				answer = "yes"
			case "no", "n":
				answer = "no"
			default:
				continue
			}
			if _, err := c.SubmitMaturityAnswerWithResponse(ctx, &dxclient.SubmitMaturityAnswerParams{Slug: slug}, dxclient.SubmitMaturityAnswerRequest{
				QuestionKey: q.Key,
				Answer:      answer,
				AnsweredBy:  "owner",
			}); err != nil {
				fmt.Printf("  warning: failed to submit answer: %v\n", err)
			}
		}
		// Reload items after answering
		if iResp, err := c.ListMaturityItemsWithResponse(ctx, &dxclient.ListMaturityItemsParams{Slug: slug}); err == nil && iResp.JSON200 != nil && iResp.JSON200.Items != nil {
			state.MaturityItems = *iResp.JSON200.Items
		}
	}

	// 5. Run checks
	findings := doctor.Evaluate(state)
	sum := doctor.Summarize(findings)

	// 6. Print findings by rung
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

	// 7. Summary
	fmt.Printf("\n%d checks: %d passed, %d failed (%d auto-fixable, %d proposals), %d deferred\n",
		sum.Total, sum.Passed, sum.Failed, sum.AutoFix, sum.Propose, sum.Deferred)

	// 8. Stamped maturity items report
	if len(state.MaturityItems) > 0 {
		counts := map[string]int{}
		for _, item := range state.MaturityItems {
			counts[item.Status]++
		}
		fmt.Printf("\n── Stamped maturity items ──\n")
		for _, item := range state.MaturityItems {
			if item.Status == "open" {
				priority := ""
				switch item.PriorityHint {
				case 1:
					priority = " [critical]"
				case 2:
					priority = " [high]"
				case 3:
					priority = " [medium]"
				case 4:
					priority = " [low]"
				}
				fmt.Printf("  ○  %s%s\n", item.Title, priority)
			}
		}
		if n := counts["done"]; n > 0 {
			fmt.Printf("  ✓  %d done\n", n)
		}
		if n := counts["dismissed"]; n > 0 {
			fmt.Printf("  —  %d dismissed\n", n)
		}
		if n := counts["snoozed"]; n > 0 {
			fmt.Printf("  ⊘  %d snoozed\n", n)
		}
		open := counts["open"]
		fmt.Printf("  %d open, %d done, %d dismissed, %d snoozed\n", open, counts["done"], counts["dismissed"], counts["snoozed"])
	}

	// 9. Interactive defer prompt for remaining failures
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

	// Classification + Vision
	if pResp, err := c.GetProjectInfoWithResponse(ctx, &dxclient.GetProjectInfoParams{Slug: slug}); err == nil && pResp.JSON200 != nil {
		state.Classification = doctor.Classification(pResp.JSON200.Classification)
	}
	// Vision: check via list-projects which includes title/description
	if lpResp, err := c.ListProjectsWithResponse(ctx); err == nil && lpResp.JSON200 != nil && lpResp.JSON200.Projects != nil {
		for _, p := range *lpResp.JSON200.Projects {
			if p.Slug == slug && p.Title != nil && *p.Title != "" {
				state.HasVision = true
				break
			}
		}
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
				state.SpecCount += specCount
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
			if doctor.IsImplDetailFeature(f.Name, f.Description) {
				state.FeaturesImplDetail++
			}
		}
	}

	// Demo test results
	if tResp, err := c.ListTestsWithResponse(ctx, &dxclient.ListTestsParams{Slug: slug}); err == nil && tResp.JSON200 != nil && tResp.JSON200.Tests != nil {
		for _, t := range *tResp.JSON200.Tests {
			if t.Layer == "demo" {
				state.DemoTestResultsExist = true
				break
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

	// Concern coverage
	if csResp, err := c.GetConcernDoctorStateWithResponse(ctx, &dxclient.GetConcernDoctorStateParams{Slug: slug}); err == nil && csResp.JSON200 != nil {
		state.ConcernCount = int(csResp.JSON200.ConcernCount)
		state.FeaturesWithConcerns = int(csResp.JSON200.FeaturesWithConcerns)
		state.ConcernsWithSpecsNoPattern = int(csResp.JSON200.ConcernsWithSpecsNoPattern)
		state.SecurityConcernSpecCount = int(csResp.JSON200.SecurityConcernSpecCount)
	}

	// Force-closed issues with no work-log substance
	if fcResp, err := c.ListForceClosedNoSubstanceWithResponse(ctx, &dxclient.ListForceClosedNoSubstanceParams{Slug: slug}); err == nil && fcResp.JSON200 != nil && fcResp.JSON200.Issues != nil {
		issues := *fcResp.JSON200.Issues
		state.ForceClosedNoSubstanceTotal = len(issues)
		for _, iss := range issues {
			state.ForceClosedNoSubstance = append(state.ForceClosedNoSubstance, doctor.ForceClosedIssue{
				ID:     iss.Id,
				Title:  iss.Title,
				Reason: extractCloseReason(iss.CloseNote),
			})
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

	// Maturity questions (with current answer state)
	if qResp, err := c.ListMaturityQuestionsWithResponse(ctx, &dxclient.ListMaturityQuestionsParams{Slug: slug}); err == nil && qResp.JSON200 != nil && qResp.JSON200.Questions != nil {
		state.MaturityQuestions = *qResp.JSON200.Questions
	}

	// Stamped maturity items
	if iResp, err := c.ListMaturityItemsWithResponse(ctx, &dxclient.ListMaturityItemsParams{Slug: slug}); err == nil && iResp.JSON200 != nil && iResp.JSON200.Items != nil {
		state.MaturityItems = *iResp.JSON200.Items
	}
}

// extractCloseReason parses the reason out of a close note like
// "[closed:wontfix] notes..." → "wontfix". Returns "" when the note doesn't
// match the expected shape.
func extractCloseReason(note string) string {
	const prefix = "[closed:"
	if !strings.HasPrefix(note, prefix) {
		return ""
	}
	rest := note[len(prefix):]
	end := strings.IndexByte(rest, ']')
	if end <= 0 {
		return ""
	}
	return rest[:end]
}

func promptClassification(in io.Reader) (doctor.Classification, error) {
	fmt.Println("What kind of project is this?")
	for i, c := range doctor.AllClassifications {
		fmt.Printf("  %d. %s\n", i+1, doctor.ClassificationLabel(c))
	}
	fmt.Print("\nChoice [1-5]: ")
	reader := bufio.NewReader(in)
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
