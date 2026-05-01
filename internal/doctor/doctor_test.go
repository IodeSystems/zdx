package doctor

import (
	"os"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

// TestEvaluateProposalsForNonAutoFixFailures verifies spec 129:
// failing checks with no auto-fix function emit a concrete proposed
// dx CLI command in Finding.Proposal.
func TestEvaluateProposalsForNonAutoFixFailures(t *testing.T) {
	state := &ProjectState{
		// scaffold: pass auto-fixable checks so they don't pollute findings
		ZdxDirExists: true,
		ConfigValid:  true,
		// scaffold: force credentials_exist (ActionPropose, no FixFunc) to fail
		CredentialsExist: false,

		// identity: force has_vision, classification_set, has_goals, has_constraints to fail
		Classification:  "",
		HasVision:       false,
		GoalCount:       0,
		ConstraintCount: 0,

		// planning: force has_features to fail
		FeatureCount: 0,

		// verification: force goals_quantified to fail
		// (GoalsTotal>0 && GoalsQuantified<GoalsTotal)
		GoalsTotal:      2,
		GoalsQuantified: 1,

		// agents: force agent_config_set to fail
		AgentConfigSet: false,
	}

	findings := Evaluate(state)
	byName := make(map[string]Finding, len(findings))
	for _, f := range findings {
		byName[f.Check.Name] = f
	}

	// Each non-auto-fix check that should fail must propose a concrete dx command.
	wantFail := []string{
		"credentials_exist",
		"classification_set",
		"has_vision",
		"has_goals",
		"has_constraints",
		"has_features",
		"goals_quantified",
		"agent_config_set",
	}
	for _, name := range wantFail {
		t.Run(name, func(t *testing.T) {
			f, ok := byName[name]
			if !ok {
				t.Fatalf("check %q not present in findings", name)
			}
			if f.Status != StatusFail {
				t.Fatalf("check %q: expected StatusFail, got %s (msg=%q)", name, f.Status, f.Message)
			}
			if f.FixFunc != nil {
				t.Errorf("check %q: expected nil FixFunc (non-auto-fix), got non-nil", name)
			}
			if f.Proposal == "" {
				t.Errorf("check %q: expected non-empty Proposal", name)
			}
			if !strings.Contains(f.Proposal, "dx ") {
				t.Errorf("check %q: expected proposal to contain a `dx ` command, got %q", name, f.Proposal)
			}
		})
	}

	// Guardrail: any ActionPropose check that fails with no FixFunc MUST set Proposal.
	// Catches future regressions where a new non-fixable check forgets remediation guidance.
	for _, f := range findings {
		if f.Check.Action != ActionPropose {
			continue
		}
		if f.Status != StatusFail {
			continue
		}
		if f.FixFunc != nil {
			continue
		}
		if f.Proposal == "" {
			t.Errorf("guardrail: ActionPropose check %q failed with nil FixFunc but empty Proposal", f.Check.Name)
		}
	}
}

// TestEvaluateMissingVisionIsFirstGap verifies spec 115:
// Given a project with no vision, when doctor evaluates, then the missing
// vision is flagged as the first maturity gap with an explanation of why a
// guiding star matters.
func TestEvaluateMissingVisionIsFirstGap(t *testing.T) {
	state := &ProjectState{
		// scaffold rung: all pass so identity rung is reached first
		ZdxDirExists:     true,
		ConfigValid:      true,
		CredentialsExist: true,
		RemoteReachable:  true,

		// identity rung: vision missing — the gap under test
		HasVision: false,
	}

	findings := Evaluate(state)
	if len(findings) == 0 {
		t.Fatal("expected non-empty findings")
	}

	var firstGap *Finding
	for i := range findings {
		if findings[i].Status == StatusFail {
			firstGap = &findings[i]
			break
		}
	}
	if firstGap == nil {
		t.Fatal("expected at least one failing check, got none")
	}

	if firstGap.Check.Name != "has_vision" {
		t.Errorf("expected first gap to be %q, got %q (msg=%q)",
			"has_vision", firstGap.Check.Name, firstGap.Message)
	}
	if firstGap.Rung != "identity" {
		t.Errorf("expected first gap on rung %q, got %q", "identity", firstGap.Rung)
	}
	if !strings.Contains(firstGap.Proposal, "guiding star") {
		t.Errorf("expected proposal to explain why a guiding star matters, got %q", firstGap.Proposal)
	}
	if !strings.Contains(firstGap.Proposal, "dx vision set") {
		t.Errorf("expected proposal to recommend `dx vision set`, got %q", firstGap.Proposal)
	}
}

// TestRunCheckGoalsQuantified verifies spec 112:
// goals_quantified passes when every goal has a metric, and fails with a
// concrete X/Y count and a `dx goal add` proposal that names --metric-name
// and --metric-unit when goals are missing metrics.
func TestRunCheckGoalsQuantified(t *testing.T) {
	cases := []struct {
		name         string
		state        ProjectState
		wantPass     bool
		wantMsgPart  string
		wantPropPart []string
	}{
		{
			name:     "all goals quantified",
			state:    ProjectState{GoalsTotal: 2, GoalsQuantified: 2},
			wantPass: true,
		},
		{
			name:         "partial coverage fails",
			state:        ProjectState{GoalsTotal: 3, GoalsQuantified: 1},
			wantPass:     false,
			wantMsgPart:  "1/3 goals have metrics",
			wantPropPart: []string{"dx goal add", "--metric-name", "--metric-unit"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pass, msg, _, proposal := runCheck("goals_quantified", &tc.state)
			if pass != tc.wantPass {
				t.Fatalf("pass=%v, want %v (msg=%q)", pass, tc.wantPass, msg)
			}
			if tc.wantPass {
				return
			}
			if !strings.Contains(msg, tc.wantMsgPart) {
				t.Errorf("msg=%q, want substring %q", msg, tc.wantMsgPart)
			}
			for _, want := range tc.wantPropPart {
				if !strings.Contains(proposal, want) {
					t.Errorf("proposal=%q, want substring %q", proposal, want)
				}
			}
		})
	}
}

// TestFindingAutoFixable verifies spec 128 (detection layer):
// a failing check with an auto-fix produces a non-nil FixFunc.
func TestFindingAutoFixable(t *testing.T) {
	state := &ProjectState{ZdxDirExists: false, ConfigValid: true}
	findings := Evaluate(state)
	byName := make(map[string]Finding, len(findings))
	for _, f := range findings {
		byName[f.Check.Name] = f
	}
	f, ok := byName["zdx_dir_exists"]
	if !ok {
		t.Fatal("zdx_dir_exists not present in findings")
	}
	if f.Status != StatusFail {
		t.Fatalf("expected StatusFail, got %s", f.Status)
	}
	if f.FixFunc == nil {
		t.Error("expected non-nil FixFunc for auto-fixable check")
	}
}

// TestFixFuncApplied verifies spec 128 (fix application):
// calling FixFunc() creates the expected artifact on disk.
func TestFixFuncApplied(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	state := &ProjectState{ZdxDirExists: false, ConfigValid: true}
	findings := Evaluate(state)
	byName := make(map[string]Finding, len(findings))
	for _, f := range findings {
		byName[f.Check.Name] = f
	}
	f, ok := byName["zdx_dir_exists"]
	if !ok {
		t.Fatal("zdx_dir_exists not present in findings")
	}
	if f.FixFunc == nil {
		t.Fatal("expected non-nil FixFunc")
	}
	if err := f.FixFunc(); err != nil {
		t.Fatalf("FixFunc() error: %v", err)
	}
	info, statErr := os.Stat(".zdx")
	if statErr != nil || !info.IsDir() {
		t.Error("expected .zdx/ directory to be created by FixFunc")
	}
}

// TestPassingCheckNoFixFunc verifies spec 128 (passing path):
// a passing check has nil FixFunc and StatusPass.
func TestPassingCheckNoFixFunc(t *testing.T) {
	state := &ProjectState{ZdxDirExists: true, ConfigValid: true}
	findings := Evaluate(state)
	byName := make(map[string]Finding, len(findings))
	for _, f := range findings {
		byName[f.Check.Name] = f
	}
	f, ok := byName["zdx_dir_exists"]
	if !ok {
		t.Fatal("zdx_dir_exists not present in findings")
	}
	if f.Status != StatusPass {
		t.Fatalf("expected StatusPass, got %s", f.Status)
	}
	if f.FixFunc != nil {
		t.Error("expected nil FixFunc for passing check")
	}
}

// TestEvaluateCheckDeferred verifies spec 131:
// TestEvaluateCheckDeferred verifies spec 131:
// evaluateCheck returns StatusDeferred when the check name is in state.Deferred,
// and returns StatusFail after clearing the deferral.
func TestEvaluateCheckDeferred(t *testing.T) {
	check := Check{Name: "credentials_exist", Action: ActionPropose}

	// Deferred: must skip and return StatusDeferred.
	f := evaluateCheck(check, "scaffold", &ProjectState{
		ZdxDirExists:     true,
		ConfigValid:      true,
		CredentialsExist: false,
		Deferred:         map[string]bool{"credentials_exist": true},
	})
	if f.Status != StatusDeferred {
		t.Errorf("expected StatusDeferred, got %s (msg=%q)", f.Status, f.Message)
	}

	// Cleared: must return StatusFail again.
	f2 := evaluateCheck(check, "scaffold", &ProjectState{
		ZdxDirExists:     true,
		ConfigValid:      true,
		CredentialsExist: false,
		Deferred:         map[string]bool{},
	})
	if f2.Status != StatusFail {
		t.Errorf("expected StatusFail after deferral cleared, got %s (msg=%q)", f2.Status, f2.Message)
	}
}

// TestEvaluateFindingsGroupedByRung verifies spec 132:
// Evaluate() returns findings grouped and ordered by rung — all scaffold

// TestEvaluateFindingsGroupedByRung verifies spec 132:
// Evaluate() returns findings grouped and ordered by rung — all scaffold
// findings appear before any identity finding, identity before planning, etc.,
// with classification-specific rungs appended after the common ones.
func TestEvaluateFindingsGroupedByRung(t *testing.T) {
	// SaaS has the richest vine: scaffold → identity → planning → verification →
	// agents → operations → multi-tenancy. Use it to cover common + two
	// classification-specific rungs.
	state := &ProjectState{
		Classification: ClassSaaS,
		// all fields left at zero/false → every check can fire, giving us
		// findings on every rung including the classification-specific ones.
	}

	findings := Evaluate(state)
	if len(findings) == 0 {
		t.Fatal("expected non-empty findings for SaaS with empty state")
	}

	// Build canonical rung order from the vine itself.
	vine := Vine(ClassSaaS)
	rungIndex := make(map[string]int, len(vine))
	for i, r := range vine {
		rungIndex[r.Name] = i
	}

	// Walk findings: each finding's rung position must be >= the previous one,
	// and all findings with the same rung name must be contiguous.
	prevRungPos := -1
	prevRungName := ""
	seenRungs := make(map[string]bool)

	for _, f := range findings {
		pos, ok := rungIndex[f.Rung]
		if !ok {
			t.Errorf("finding %q has rung %q not present in vine", f.Check.Name, f.Rung)
			continue
		}
		if pos < prevRungPos {
			t.Errorf("finding %q (rung %q, pos %d) appears after rung %q (pos %d) — not ordered",
				f.Check.Name, f.Rung, pos, prevRungName, prevRungPos)
		}
		if pos > prevRungPos && seenRungs[f.Rung] {
			t.Errorf("finding %q (rung %q) appears after a later rung — not grouped",
				f.Check.Name, f.Rung)
		}
		seenRungs[f.Rung] = true
		prevRungPos = pos
		prevRungName = f.Rung
	}

	// Every vine rung must contribute at least one finding.
	for _, r := range vine {
		if !seenRungs[r.Name] {
			t.Errorf("rung %q produced no findings — expected at least one", r.Name)
		}
	}
}

// TestRunCheckFeaturesAreUserVisible verifies spec 113:
// features_are_user_visible passes when no features look like code modules,
// and fails with a count + proposal containing "demonstrable value driver"
// guidance when impl-detail features are present.
func TestRunCheckFeaturesAreUserVisible(t *testing.T) {
	cases := []struct {
		name         string
		state        ProjectState
		wantPass     bool
		wantMsgPart  string
		wantPropPart []string
	}{
		{
			name:     "no impl-detail features — passes",
			state:    ProjectState{FeaturesImplDetail: 0},
			wantPass: true,
		},
		{
			name:         "one impl-detail feature — fails with guidance",
			state:        ProjectState{FeaturesImplDetail: 1},
			wantPass:     false,
			wantMsgPart:  "1 feature(s) describe implementation details",
			wantPropPart: []string{"demonstrable value driver", "not code modules"},
		},
		{
			name:         "multiple impl-detail features — count in message",
			state:        ProjectState{FeaturesImplDetail: 3},
			wantPass:     false,
			wantMsgPart:  "3 feature(s) describe implementation details",
			wantPropPart: []string{"demonstrable value driver"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pass, msg, _, proposal := runCheck("features_are_user_visible", &tc.state)
			if pass != tc.wantPass {
				t.Fatalf("pass=%v, want %v (msg=%q)", pass, tc.wantPass, msg)
			}
			if tc.wantPass {
				return
			}
			if !strings.Contains(msg, tc.wantMsgPart) {
				t.Errorf("msg=%q, want substring %q", msg, tc.wantMsgPart)
			}
			for _, want := range tc.wantPropPart {
				if !strings.Contains(proposal, want) {
					t.Errorf("proposal=%q, want substring %q", proposal, want)
				}
			}
		})
	}
}

// TestIsImplDetailFeature verifies spec 113 (detection layer):
// IsImplDetailFeature correctly classifies feature names and descriptions.
func TestIsImplDetailFeature(t *testing.T) {
	yes := []struct{ name, desc string }{
		{"redis-cache-layer", "Add Redis caching to the service layer"},
		{"auth-middleware", "JWT validation middleware"},
		{"db-migration-service", ""},
		{"refactor-handler", ""},
		{"", "Add a cron scheduler for cleanup jobs"},
	}
	no := []struct{ name, desc string }{
		{"faster search", "Users can find results in under 200ms"},
		{"onboarding flow", "New users complete setup in one step"},
		{"multi-tenant billing", "Each tenant sees their own invoices"},
	}

	for _, tc := range yes {
		if !IsImplDetailFeature(tc.name, tc.desc) {
			t.Errorf("IsImplDetailFeature(%q, %q) = false, want true", tc.name, tc.desc)
		}
	}
	for _, tc := range no {
		if IsImplDetailFeature(tc.name, tc.desc) {
			t.Errorf("IsImplDetailFeature(%q, %q) = true, want false", tc.name, tc.desc)
		}
	}
}

// TestRunCheckHasDeployConfigGateBranch verifies spec 166:
// Given a project shipping to production from the dev branch directly,
// when doctor runs, then it suggests introducing a tested gate branch.
func TestRunCheckHasDeployConfigGateBranch(t *testing.T) {
	cases := []struct {
		name          string
		shipsFromDev  bool
		wantPass      bool
		wantMsgPart   string
		wantPropParts []string
	}{
		{
			name:         "gate branch present — passes",
			shipsFromDev: false,
			wantPass:     true,
		},
		{
			name:          "ships from dev directly — suggests gate branch",
			shipsFromDev:  true,
			wantPass:      false,
			wantMsgPart:   "dev/main directly to production",
			wantPropParts: []string{"gate branch", "staging", "production"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := ProjectState{ShipsFromDevDirectly: tc.shipsFromDev}
			pass, msg, _, proposal := runCheck("has_deploy_config", &state)
			if pass != tc.wantPass {
				t.Fatalf("pass=%v, want %v (msg=%q)", pass, tc.wantPass, msg)
			}
			if tc.wantPass {
				return
			}
			if !strings.Contains(msg, tc.wantMsgPart) {
				t.Errorf("msg=%q, want substring %q", msg, tc.wantMsgPart)
			}
			for _, want := range tc.wantPropParts {
				if !strings.Contains(proposal, want) {
					t.Errorf("proposal=%q, want substring %q", proposal, want)
				}
			}
		})
	}
}

// TestShipConfigDefined covers the ship_config_defined runCheck cases.
func TestShipConfigDefined(t *testing.T) {
	configWithShip := &config.Config{
		Components: map[string]config.Component{
			"api": {Ship: config.Ship{Strategy: "simple"}},
		},
	}
	configMissingShip := &config.Config{
		Components: map[string]config.Component{
			"api":    {Ship: config.Ship{Strategy: "simple"}},
			"worker": {},
		},
	}
	configNoComponents := &config.Config{}

	cases := []struct {
		name        string
		state       *ProjectState
		wantPass    bool
		wantMsgPart string
	}{
		{
			name:     "nil config passes",
			state:    &ProjectState{},
			wantPass: true,
		},
		{
			name:     "no components passes",
			state:    &ProjectState{Config: configNoComponents},
			wantPass: true,
		},
		{
			name:     "all components have ship passes",
			state:    &ProjectState{Config: configWithShip, ComponentsWithoutShip: 0},
			wantPass: true,
		},
		{
			name:        "one component missing ship fails with count",
			state:       &ProjectState{Config: configMissingShip, ComponentsWithoutShip: 1},
			wantPass:    false,
			wantMsgPart: "1/2 components missing ship config",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pass, msg, fixFunc, proposal := runCheck("ship_config_defined", tc.state)
			if pass != tc.wantPass {
				t.Errorf("pass=%v, want %v (msg=%q)", pass, tc.wantPass, msg)
			}
			if !tc.wantPass {
				if fixFunc != nil {
					t.Error("expected nil FixFunc for ActionPropose check")
				}
				if proposal == "" {
					t.Error("expected non-empty proposal on failure")
				}
				if tc.wantMsgPart != "" && !strings.Contains(msg, tc.wantMsgPart) {
					t.Errorf("msg=%q, want substring %q", msg, tc.wantMsgPart)
				}
			}
		})
	}
}
