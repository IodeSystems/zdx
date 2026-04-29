package doctor

import (
	"strings"
	"testing"
)

// TestRunCheckLayeredBDDTests verifies spec 134:
// the layered-bdd-tests check activates only for service/saas/site projects
// with 5+ specs and recorded demo-layer test results, and is skipped or
// neutralized otherwise.
func TestRunCheckLayeredBDDTests(t *testing.T) {
	cases := []struct {
		name         string
		state        ProjectState
		wantPass     bool
		wantMsgPart  string
		wantPropPart string
	}{
		{
			name: "library classification skips check",
			state: ProjectState{
				Classification:       ClassLibrary,
				SpecCount:            10,
				DemoTestResultsExist: true,
				UsesStepDriver:       false,
			},
			wantPass:    true,
			wantMsgPart: "not applicable",
		},
		{
			name: "tool classification skips check",
			state: ProjectState{
				Classification:       ClassTool,
				SpecCount:            10,
				DemoTestResultsExist: true,
				UsesStepDriver:       false,
			},
			wantPass:    true,
			wantMsgPart: "not applicable",
		},
		{
			name: "service with fewer than 5 specs is not yet gated",
			state: ProjectState{
				Classification:       ClassService,
				SpecCount:            4,
				DemoTestResultsExist: true,
				UsesStepDriver:       false,
			},
			wantPass:    true,
			wantMsgPart: "4 specs (need 5+)",
		},
		{
			name: "saas with 5+ specs but no demo test results is not yet gated",
			state: ProjectState{
				Classification:       ClassSaaS,
				SpecCount:            7,
				DemoTestResultsExist: false,
				UsesStepDriver:       false,
			},
			wantPass:    true,
			wantMsgPart: "no demo-layer test results",
		},
		{
			name: "site with 5+ specs, demo results, and StepDriver passes",
			state: ProjectState{
				Classification:       ClassSite,
				SpecCount:            6,
				DemoTestResultsExist: true,
				UsesStepDriver:       true,
			},
			wantPass:    true,
			wantMsgPart: "StepDriver pattern already in use",
		},
		{
			name: "service with 5+ specs and demo results but no StepDriver fails",
			state: ProjectState{
				Classification:       ClassService,
				SpecCount:            5,
				DemoTestResultsExist: true,
				UsesStepDriver:       false,
			},
			wantPass:     false,
			wantMsgPart:  "no StepDriver pattern",
			wantPropPart: "layered BDD",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pass, msg, _, proposal := runCheck("layered-bdd-tests", &tc.state)
			if pass != tc.wantPass {
				t.Fatalf("pass=%v, want %v (msg=%q)", pass, tc.wantPass, msg)
			}
			if !strings.Contains(msg, tc.wantMsgPart) {
				t.Errorf("msg=%q, want substring %q", msg, tc.wantMsgPart)
			}
			if tc.wantPropPart != "" && !strings.Contains(proposal, tc.wantPropPart) {
				t.Errorf("proposal=%q, want substring %q", proposal, tc.wantPropPart)
			}
		})
	}
}
