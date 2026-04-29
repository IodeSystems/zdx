package doctor

import (
	"strings"
	"testing"
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
