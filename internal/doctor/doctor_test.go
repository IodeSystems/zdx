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
