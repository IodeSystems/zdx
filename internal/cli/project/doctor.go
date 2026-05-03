package project

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/doctor"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func DoctorCmd() *cobra.Command {
	var autoFix bool
	var reQuestionnaire bool
	var classType string
	var noDefer bool
	var autoDeferAll bool
	var nonInteractive bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose project health and propose fixes",
		Long:  "Doctor checks the project against its maturity vine, auto-fixes what it can, and proposes actions for the rest. Deferred proposals don't nag until the next rung.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), autoFix, reQuestionnaire, classType, noDefer, autoDeferAll, nonInteractive)
		},
	}
	cmd.Flags().BoolVar(&autoFix, "fix", false, "auto-apply fixable issues without prompting")
	cmd.Flags().BoolVar(&reQuestionnaire, "re-questionnaire", false, "re-prompt already-answered maturity questions")
	cmd.Flags().StringVar(&classType, "type", "", "project classification (library|tool|service|saas|site); skips prompt")
	cmd.Flags().BoolVar(&noDefer, "no-defer", false, "skip defer prompts; report only")
	cmd.Flags().BoolVar(&autoDeferAll, "auto-defer-all", false, "defer every failed proposal non-interactively")
	cmd.Flags().BoolVar(&nonInteractive, "non-interactive", false, "implies --no-defer; skip maturity questions with a warning; requires --type if unclassified")
	return cmd
}

func runDoctor(ctx context.Context, autoFix bool, reQuestionnaire bool, classType string, noDefer bool, autoDeferAll bool, nonInteractive bool) error {
	if noDefer && autoDeferAll {
		return fmt.Errorf("--no-defer and --auto-defer-all are mutually exclusive")
	}
	if nonInteractive {
		noDefer = true
	}

	state := &doctor.ProjectState{
		Deferred: map[string]bool{},
	}

	// 1. Local detection
	doctor.DetectLocal(state)

	// 2. Remote state (if credentials exist)
	if state.CredentialsExist {
		populateRemoteState(ctx, state)
	}

	// 3. Classification: --type flag takes priority; fall back to interactive prompt
	if classType != "" {
		class, err := parseClassification(classType)
		if err != nil {
			return err
		}
		state.Classification = class
		if state.RemoteReachable {
			c := cli.MustClient()
			_, _ = c.SetClassificationWithResponse(ctx, dxclient.SetClassificationRequest{
				Slug:           c.SlugOrDie(),
				Classification: string(class),
			})
			_, _ = c.SeedVersionBranchesWithResponse(ctx, c.SlugOrDie())
		}
	} else if state.Classification == "" {
		if nonInteractive {
			return fmt.Errorf("project has no classification; re-run with --type=<library|tool|service|saas|site>")
		}
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
			_, _ = c.SeedVersionBranchesWithResponse(ctx, c.SlugOrDie())
		}
	} else if state.RemoteReachable {
		// Classification was already set (read from server in step 2). Re-seed
		// idempotently so dx init on an existing project fills in any rows
		// that haven't been seeded yet (e.g. older projects predating IS-966).
		c := cli.MustClient()
		_, _ = c.SeedVersionBranchesWithResponse(ctx, c.SlugOrDie())
	}

	// 4. Walk unanswered maturity questions
	if state.RemoteReachable && len(state.MaturityQuestions) > 0 {
		c := cli.MustClient()
		slug := c.SlugOrDie()
		classification := string(state.Classification)
		var reader *bufio.Reader
		if !nonInteractive {
			reader = bufio.NewReader(os.Stdin)
		}
		skipped := 0
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
			if nonInteractive {
				skipped++
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
		if skipped > 0 {
			fmt.Printf("  warning: %d maturity question(s) skipped — re-run interactively or with answers\n", skipped)
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

	// 9. Defer prompt / auto-defer / skip
	if sum.Failed > 0 && !autoFix {
		switch {
		case autoDeferAll:
			for i := range findings {
				f := &findings[i]
				if f.Status != doctor.StatusFail || f.FixFunc != nil || f.Proposal == "" {
					continue
				}
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
				sum.Deferred++
			}
		case noDefer:
			// report-only; no prompts
		default:
			reader := bufio.NewReader(os.Stdin)
			for i := range findings {
				f := &findings[i]
				if f.Status != doctor.StatusFail || f.FixFunc != nil || f.Proposal == "" {
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
					sum.Deferred++
				}
			}
		}
	}

	// Return non-zero when running non-interactively and failures remain
	remaining := sum.Failed - sum.Deferred
	if remaining > 0 && (nonInteractive || noDefer) {
		return fmt.Errorf("%d check(s) failed", remaining)
	}

	return nil
}

func populateRemoteState(ctx context.Context, state *doctor.ProjectState) {
	c, err := cli.DefaultClient()
	if err != nil {
		return
	}
	slug := c.SlugOrDie()

	// Health check + embedder/index subsystem state (IS-811).
	if hResp, err := c.HealthWithResponse(ctx); err == nil && hResp.JSON200 != nil {
		if hResp.JSON200.Status == "ok" {
			state.RemoteReachable = true
		}
		subs := hResp.JSON200.Subsystems
		if emb, ok := subs["embedder"]; ok && emb.State == "ok" {
			state.EmbedderConfigured = true
			state.EmbedderResponsive = emb.Reason == nil
		}
		if idx, ok := subs["index"]; ok && idx.State == "ok" {
			state.IndexBuilt = true
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
			mustShouldCount := 0
			if f.Specs != nil {
				for _, s := range *f.Specs {
					specCount++
					if s.Importance == "must" || s.Importance == "should" {
						mustShouldCount++
					}
				}
				state.SpecCount += specCount
			}
			if specCount > 0 {
				state.FeaturesWithSpecs++
			}
			if f.GoalId > 0 || f.ParentFeatureId > 0 {
				state.FeaturesAttributed++
			}
			state.SpecsTotal += specCount
			if mustShouldCount > 8 {
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

	// Untriaged issues + open issue count (drives queue_has_claimable_work)
	if iResp, err := c.ListIssuesWithResponse(ctx, &dxclient.ListIssuesParams{Slug: slug}); err == nil && iResp.JSON200 != nil && iResp.JSON200.Issues != nil {
		for _, iss := range *iResp.JSON200.Issues {
			if iss.Status != "closed" {
				state.OpenIssueCount++
			}
			if iss.Priority == "" && iss.Status != "closed" {
				state.UntriagedIssues++
			}
		}
	}

	// Todo queue health (IS-956)
	if qResp, err := c.GetTodoQueueHealthWithResponse(ctx, &dxclient.GetTodoQueueHealthParams{Slug: slug}); err == nil && qResp.JSON200 != nil {
		state.TodoQueueHealth = doctor.TodoQueueHealth{
			TotalOpen:             int(qResp.JSON200.TotalOpen),
			BlockedCount:          int(qResp.JSON200.BlockedCount),
			UnblockedCount:        int(qResp.JSON200.UnblockedCount),
			DominantBlockedReason: qResp.JSON200.DominantBlockedReason,
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

	// Historical close-gate audit (IS-632) — closed issues that fail the
	// IS-560 close-gate retroactively, grouped by gate.
	if hcResp, err := c.ListHistoricalCloseGateOffendersWithResponse(ctx, &dxclient.ListHistoricalCloseGateOffendersParams{Slug: slug}); err == nil && hcResp.JSON200 != nil && hcResp.JSON200.Offenders != nil {
		state.HistoricalOffendersByGate = map[string]int{}
		seen := map[string]bool{}
		const sampleCap = 5
		for _, off := range *hcResp.JSON200.Offenders {
			state.HistoricalOffendersByGate[off.Gate]++
			if !seen[off.IssueId] {
				seen[off.IssueId] = true
				state.HistoricalOffenderCount++
			}
			if len(state.HistoricalOffenderSample) < sampleCap {
				state.HistoricalOffenderSample = append(state.HistoricalOffenderSample, doctor.HistoricalOffender{
					IssueID: off.IssueId,
					Gate:    off.Gate,
					Detail:  off.Detail,
				})
			}
		}
	}

	// Stale agent sessions (still open past the sweeper's idle threshold)
	if sResp, err := c.ListStaleOpenClaudeSessionsWithResponse(ctx, &dxclient.ListStaleOpenClaudeSessionsParams{Slug: slug}); err == nil && sResp.JSON200 != nil {
		state.StaleAgentSessions = int(sResp.JSON200.Total)
		state.StaleAgentSessionsMinutes = sResp.JSON200.Minutes
	}

	// Token hygiene (IS-841): admin-only endpoint. Non-admin callers receive
	// a 403 and the rung silently passes (state slices remain empty).
	if thResp, err := c.DoctorTokenHygieneWithResponse(ctx); err == nil && thResp.JSON200 != nil {
		if thResp.JSON200.LegacyAgentTokens != nil {
			state.LegacyAgentTokens = *thResp.JSON200.LegacyAgentTokens
		}
		if thResp.JSON200.RecentAdminTokens != nil {
			state.RecentAdminTokens = *thResp.JSON200.RecentAdminTokens
		}
	}

	// KPI breaches: any check whose trailing median exceeds 15s, or whose
	// recent samples exceed 2× the trailing median. Skips silently when the
	// table is empty.
	{
		scope := "tech"
		n := int32(20)
		if kResp, err := c.GetKpiTrendWithResponse(ctx, &dxclient.GetKpiTrendParams{Slug: &slug, Scope: &scope, N: &n}); err == nil && kResp.JSON200 != nil && kResp.JSON200.Items != nil {
			byCheck := map[string][]float64{}
			for _, item := range *kResp.JSON200.Items {
				byCheck[item.CheckName] = append(byCheck[item.CheckName], item.Value)
			}
			names := make([]string, 0, len(byCheck))
			for name := range byCheck {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				vals := byCheck[name]
				median := kpiMedian(vals)
				if median > 15 {
					state.KpiBreachedChecks = append(state.KpiBreachedChecks, name)
					continue
				}
				for _, v := range vals {
					if v > 2*median {
						state.KpiBreachedChecks = append(state.KpiBreachedChecks, name)
						break
					}
				}
			}
		}
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

	// Version branches — drives the branching_strategy_appropriate rung.
	// We read `Type` (the API still publishes the legacy passthrough; new
	// roles like pr-target/rolling-release surface verbatim as Type until
	// the API fully renames in IS-967). The legacy alias "named" maps to
	// the named-release role.
	if vbResp, err := c.ListVersionBranchesWithResponse(ctx, slug); err == nil && vbResp.JSON200 != nil && vbResp.JSON200.Branches != nil {
		for _, b := range *vbResp.JSON200.Branches {
			switch b.Type {
			case "dev":
				state.BranchingStrategyHasDev = true
			case "pr-target":
				state.BranchingStrategyHasPRTarget = true
			case "named", "named-release":
				state.BranchingStrategyHasNamedRelease = true
			case "rolling-release":
				state.BranchingStrategyHasRollingRelease = true
			}
		}
	}

	// Deploy summary — best-effort: count recorded deploys across environments and
	// capture the most-recent deploy's status. On any API error we leave
	// LastDeployStatus as "unknown" so the check passes by default.
	state.LastDeployStatus = "unknown"
	if eResp, err := c.ListEnvironmentsWithResponse(ctx, slug); err == nil && eResp.JSON200 != nil && eResp.JSON200.Items != nil {
		limit := int32(1)
		var latestAt string
		var latestStatus string
		for _, env := range *eResp.JSON200.Items {
			dResp, err := c.ListEnvironmentDeploysWithResponse(ctx, slug, env.Name, &dxclient.ListEnvironmentDeploysParams{Limit: &limit})
			if err != nil || dResp.JSON200 == nil {
				continue
			}
			state.DeployEventCount += int(dResp.JSON200.Total)
			if dResp.JSON200.Items == nil {
				continue
			}
			for _, d := range *dResp.JSON200.Items {
				if d.DeployedAt > latestAt {
					latestAt = d.DeployedAt
					latestStatus = d.Status
				}
			}
		}
		if latestStatus != "" {
			state.LastDeployStatus = latestStatus
		}
	}
}

// kpiMedian returns the median of vals (sorts in place).
func kpiMedian(vals []float64) float64 {
	sort.Float64s(vals)
	n := len(vals)
	if n == 0 {
		return 0
	}
	if n%2 == 0 {
		return (vals[n/2-1] + vals[n/2]) / 2
	}
	return vals[n/2]
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

func parseClassification(s string) (doctor.Classification, error) {
	for _, c := range doctor.AllClassifications {
		if strings.EqualFold(s, string(c)) {
			return c, nil
		}
	}
	return "", fmt.Errorf("unknown classification %q; must be one of: library, tool, service, saas, site", s)
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
