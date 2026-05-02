package doctor

import (
	"strings"
	"testing"
)

func findBranchingFinding(findings []Finding, name string) *Finding {
	for i := range findings {
		if findings[i].Check.Name == name {
			return &findings[i]
		}
	}
	return nil
}

// TestBranchingRungInCommonVine — the rung must appear for every
// classification (it lives on the common vine, between planning and concerns).
func TestBranchingRungInCommonVine(t *testing.T) {
	for _, c := range AllClassifications {
		t.Run(string(c), func(t *testing.T) {
			vine := Vine(c)
			var found *Rung
			for i := range vine {
				if vine[i].Name == "branching" {
					found = &vine[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("branching rung missing from %s vine", c)
			}
			if len(found.Checks) != 1 {
				t.Errorf("expected 1 check on branching rung, got %d", len(found.Checks))
			}
			if found.Checks[0].Name != "branching_strategy_appropriate" {
				t.Errorf("expected branching_strategy_appropriate, got %q", found.Checks[0].Name)
			}
		})
	}
}

func TestComputeBranchingRung(t *testing.T) {
	cases := []struct {
		name string
		in   ProjectState
		want int
	}{
		{"empty", ProjectState{}, 1},
		{"only_rolling_release", ProjectState{BranchingStrategyHasRollingRelease: true}, 1},
		{"pr_target_only", ProjectState{BranchingStrategyHasPRTarget: true}, 2},
		{"dev_only", ProjectState{BranchingStrategyHasDev: true}, 3},
		{"dev_and_pr_target", ProjectState{BranchingStrategyHasDev: true, BranchingStrategyHasPRTarget: true}, 3},
		{"dev_and_named_release", ProjectState{BranchingStrategyHasDev: true, BranchingStrategyHasNamedRelease: true}, 4},
		{"named_release_only", ProjectState{BranchingStrategyHasNamedRelease: true}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := computeBranchingRung(&tc.in)
			if got != tc.want {
				t.Errorf("computeBranchingRung() = %d, want %d", got, tc.want)
			}
		})
	}
}

// Matrix: branching_strategy_appropriate behavior across (classification × rung).
func TestBranchingStrategyMatrix(t *testing.T) {
	type cell struct {
		name          string
		class         Classification
		state         ProjectState
		wantStatus    FindingStatus
		wantNudgeRung string // substring expected in proposal when failing
		wantNudgeCmd  string // substring expected in proposal when failing
	}

	cases := []cell{
		// Library — always pass, regardless of rung.
		{"library_rung1", ClassLibrary, ProjectState{}, StatusPass, "", ""},
		{"library_rung2", ClassLibrary, ProjectState{BranchingStrategyHasPRTarget: true}, StatusPass, "", ""},
		{"library_rung3", ClassLibrary, ProjectState{BranchingStrategyHasDev: true}, StatusPass, "", ""},
		{"library_rung4", ClassLibrary, ProjectState{BranchingStrategyHasDev: true, BranchingStrategyHasNamedRelease: true}, StatusPass, "", ""},

		// Service — rung 1 fails → nudge to rung 3 with dx branch add-role dev.
		{"service_rung1", ClassService, ProjectState{}, StatusFail, "rung 3", "dx branch add-role dev"},
		{"service_rung2", ClassService, ProjectState{BranchingStrategyHasPRTarget: true}, StatusPass, "", ""},
		{"service_rung3", ClassService, ProjectState{BranchingStrategyHasDev: true}, StatusPass, "", ""},
		{"service_rung4", ClassService, ProjectState{BranchingStrategyHasDev: true, BranchingStrategyHasNamedRelease: true}, StatusPass, "", ""},

		// SaaS — same matrix as service.
		{"saas_rung1", ClassSaaS, ProjectState{}, StatusFail, "rung 3", "dx branch add-role dev"},
		{"saas_rung2", ClassSaaS, ProjectState{BranchingStrategyHasPRTarget: true}, StatusPass, "", ""},
		{"saas_rung3", ClassSaaS, ProjectState{BranchingStrategyHasDev: true}, StatusPass, "", ""},
		{"saas_rung4", ClassSaaS, ProjectState{BranchingStrategyHasDev: true, BranchingStrategyHasNamedRelease: true}, StatusPass, "", ""},

		// Tool — rung 1 with deploys fails; rung 1 without deploys passes.
		{"tool_rung1_no_deploys", ClassTool, ProjectState{}, StatusPass, "", ""},
		{"tool_rung1_with_deploys", ClassTool, ProjectState{DeployEventCount: 3}, StatusFail, "rung 2", "dx branch add-role pr-target"},
		{"tool_rung2", ClassTool, ProjectState{BranchingStrategyHasPRTarget: true, DeployEventCount: 3}, StatusPass, "", ""},
		{"tool_rung3", ClassTool, ProjectState{BranchingStrategyHasDev: true, DeployEventCount: 3}, StatusPass, "", ""},
		{"tool_rung4", ClassTool, ProjectState{BranchingStrategyHasDev: true, BranchingStrategyHasNamedRelease: true, DeployEventCount: 3}, StatusPass, "", ""},

		// Site — same matrix as tool.
		{"site_rung1_no_deploys", ClassSite, ProjectState{}, StatusPass, "", ""},
		{"site_rung1_with_deploys", ClassSite, ProjectState{DeployEventCount: 1}, StatusFail, "rung 2", "dx branch add-role pr-target"},
		{"site_rung2", ClassSite, ProjectState{BranchingStrategyHasPRTarget: true, DeployEventCount: 1}, StatusPass, "", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := tc.state
			state.Classification = tc.class
			findings := Evaluate(&state)
			f := findBranchingFinding(findings, "branching_strategy_appropriate")
			if f == nil {
				t.Fatal("branching_strategy_appropriate finding missing")
			}
			if f.Status != tc.wantStatus {
				t.Fatalf("status = %s, want %s (msg=%q proposal=%q)",
					f.Status, tc.wantStatus, f.Message, f.Proposal)
			}
			if tc.wantStatus != StatusFail {
				return
			}
			if !strings.Contains(f.Proposal, tc.wantNudgeRung) {
				t.Errorf("proposal missing target rung %q: %q", tc.wantNudgeRung, f.Proposal)
			}
			if !strings.Contains(f.Proposal, tc.wantNudgeCmd) {
				t.Errorf("proposal missing CLI cmd %q: %q", tc.wantNudgeCmd, f.Proposal)
			}
		})
	}
}
