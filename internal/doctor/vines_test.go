package doctor

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// TestVine verifies spec 127: Vine() composes the eight common rungs (in order)
// followed by the classification-specific rung(s) for each classification.
func TestVine(t *testing.T) {
	commonRungs := []string{"scaffold", "identity", "planning", "branching", "concerns", "verification", "retroactive_audit", "agents", "queue_health"}

	cases := []struct {
		class    Classification
		extra    []string
		minTotal int
	}{
		{ClassLibrary, []string{"distribution"}, 10},
		{ClassTool, []string{"distribution"}, 10},
		{ClassService, []string{"operations"}, 10},
		{ClassSaaS, []string{"operations", "multi-tenancy"}, 11},
		{ClassSite, []string{"publication"}, 10},
	}

	for _, tc := range cases {
		t.Run(string(tc.class), func(t *testing.T) {
			vine := Vine(tc.class)

			if got, want := len(vine), tc.minTotal; got != want {
				t.Fatalf("Vine(%s): got %d rungs, want %d", tc.class, got, want)
			}

			// Common rungs come first, in order.
			for i, name := range commonRungs {
				if vine[i].Name != name {
					t.Errorf("Vine(%s)[%d].Name = %q, want %q", tc.class, i, vine[i].Name, name)
				}
			}

			// Classification-specific rungs are appended after common rungs.
			for i, name := range tc.extra {
				idx := len(commonRungs) + i
				if vine[idx].Name != name {
					t.Errorf("Vine(%s)[%d].Name = %q, want %q", tc.class, idx, vine[idx].Name, name)
				}
			}

			// Every rung must have at least one check.
			for _, r := range vine {
				if len(r.Checks) == 0 {
					t.Errorf("Vine(%s): rung %q has no checks", tc.class, r.Name)
				}
			}
		})
	}
}

// TestVineCoversAllClassifications guards against AllClassifications drifting
// from the Vine switch — every classification must produce a non-empty vine
// with at least one classification-specific rung beyond the common set.
func TestVineCoversAllClassifications(t *testing.T) {
	commonCount := len(commonVine())
	for _, c := range AllClassifications {
		vine := Vine(c)
		if len(vine) <= commonCount {
			t.Errorf("Vine(%s) has %d rungs, expected > %d (missing classification-specific rung)",
				c, len(vine), commonCount)
		}
	}
}

// findFinding returns the first finding with the given check name.
func findFinding(findings []Finding, checkName string) (Finding, bool) {
	idx := slices.IndexFunc(findings, func(f Finding) bool { return f.Check.Name == checkName })
	if idx < 0 {
		return Finding{}, false
	}
	return findings[idx], true
}

// TestConcernRungs verifies the four concern-level doctor checks against
// synthetic ProjectState values — no server or DB needed.
func TestConcernRungs(t *testing.T) {
	t.Run("concerns_defined/pass when ConcernCount>0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 3})
		f, ok := findFinding(findings, "concerns_defined")
		if !ok {
			t.Fatal("concerns_defined check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("concerns_defined/fail when 0 and FixFunc set", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 0})
		f, ok := findFinding(findings, "concerns_defined")
		if !ok {
			t.Fatal("concerns_defined check not found")
		}
		if f.Status != StatusFail {
			t.Errorf("want fail, got %s", f.Status)
		}
		if f.FixFunc == nil {
			t.Error("expected non-nil FixFunc for auto-fix seeding")
		}
	})

	t.Run("features_have_concerns/pass when FeaturesWithConcerns==FeaturesWithSpecs", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 1, FeaturesWithSpecs: 3, FeaturesWithConcerns: 3})
		f, ok := findFinding(findings, "features_have_concerns")
		if !ok {
			t.Fatal("features_have_concerns check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("features_have_concerns/fail when gap", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 1, FeaturesWithSpecs: 3, FeaturesWithConcerns: 1})
		f, ok := findFinding(findings, "features_have_concerns")
		if !ok {
			t.Fatal("features_have_concerns check not found")
		}
		if f.Status != StatusFail {
			t.Errorf("want fail, got %s", f.Status)
		}
		if f.Proposal == "" {
			t.Error("expected non-empty Proposal")
		}
		if f.FixFunc != nil {
			t.Error("features_have_concerns must not have a FixFunc (not deterministic)")
		}
	})

	t.Run("features_have_concerns/pass vacuously when FeaturesWithSpecs==0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 1, FeaturesWithSpecs: 0, FeaturesWithConcerns: 0})
		f, ok := findFinding(findings, "features_have_concerns")
		if !ok {
			t.Fatal("features_have_concerns check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass (vacuous), got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("concerns_have_patterns/pass when ConcernsWithSpecsNoPattern==0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 2, ConcernsWithSpecsNoPattern: 0})
		f, ok := findFinding(findings, "concerns_have_patterns")
		if !ok {
			t.Fatal("concerns_have_patterns check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("concerns_have_patterns/fail when >0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 2, ConcernsWithSpecsNoPattern: 1})
		f, ok := findFinding(findings, "concerns_have_patterns")
		if !ok {
			t.Fatal("concerns_have_patterns check not found")
		}
		if f.Status != StatusFail {
			t.Errorf("want fail, got %s", f.Status)
		}
		if f.Proposal == "" {
			t.Error("expected non-empty Proposal")
		}
	})

	t.Run("security_concern_specs/pass when SecurityConcernSpecCount>0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 1, SecurityConcernSpecCount: 2})
		f, ok := findFinding(findings, "security_concern_specs")
		if !ok {
			t.Fatal("security_concern_specs check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("security_concern_specs/pass when ConcernCount==0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 0, SecurityConcernSpecCount: 0})
		f, ok := findFinding(findings, "security_concern_specs")
		if !ok {
			t.Fatal("security_concern_specs check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass (no concerns yet), got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("security_concern_specs/fail when ConcernCount>0 and SecurityConcernSpecCount==0", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ConcernCount: 3, SecurityConcernSpecCount: 0})
		f, ok := findFinding(findings, "security_concern_specs")
		if !ok {
			t.Fatal("security_concern_specs check not found")
		}
		if f.Status != StatusFail {
			t.Errorf("want fail, got %s", f.Status)
		}
		if f.Proposal == "" {
			t.Error("expected non-empty Proposal")
		}
	})
}

// TestForceClosesHaveWorkLog covers the planning-rung accountability check
// that flags closed issues with a non-done close reason (wontfix/duplicate/
// link) and zero substantive work-log entries.
func TestForceClosesHaveWorkLog(t *testing.T) {
	t.Run("pass when total is zero", func(t *testing.T) {
		findings := Evaluate(&ProjectState{ForceClosedNoSubstanceTotal: 0})
		f, ok := findFinding(findings, "force_closes_have_work_log")
		if !ok {
			t.Fatal("force_closes_have_work_log check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("fail with sample message", func(t *testing.T) {
		findings := Evaluate(&ProjectState{
			ForceClosedNoSubstanceTotal: 2,
			ForceClosedNoSubstance: []ForceClosedIssue{
				{ID: "IS-101", Title: "stale dup", Reason: "duplicate"},
				{ID: "IS-102", Title: "abandoned", Reason: "wontfix"},
			},
		})
		f, ok := findFinding(findings, "force_closes_have_work_log")
		if !ok {
			t.Fatal("force_closes_have_work_log check not found")
		}
		if f.Status != StatusFail {
			t.Fatalf("want fail, got %s: %s", f.Status, f.Message)
		}
		if !strings.Contains(f.Message, "IS-101 (duplicate)") || !strings.Contains(f.Message, "IS-102 (wontfix)") {
			t.Errorf("message missing issue IDs/reasons: %q", f.Message)
		}
		if f.FixFunc != nil {
			t.Error("force_closes_have_work_log must be advisory only (no FixFunc)")
		}
	})

	t.Run("fail truncates sample at 3 with remaining count", func(t *testing.T) {
		state := &ProjectState{ForceClosedNoSubstanceTotal: 5}
		for i := 0; i < 5; i++ {
			state.ForceClosedNoSubstance = append(state.ForceClosedNoSubstance, ForceClosedIssue{
				ID:     fmt.Sprintf("IS-%d", 200+i),
				Reason: "wontfix",
			})
		}
		findings := Evaluate(state)
		f, ok := findFinding(findings, "force_closes_have_work_log")
		if !ok {
			t.Fatal("force_closes_have_work_log check not found")
		}
		if !strings.Contains(f.Message, "+2 more") {
			t.Errorf("expected '+2 more' suffix, got: %q", f.Message)
		}
		if strings.Contains(f.Message, "IS-204") {
			t.Errorf("expected IS-204 to be truncated, got: %q", f.Message)
		}
	})
}

// TestRetroactiveAuditRung covers IS-632: the closed_issues_pass_gates check
// surfaces historical close-gate failures grouped by gate (no-worklog,
// open-tasks, missing-demo). Advisory only — no FixFunc, no Proposal.
func TestRetroactiveAuditRung(t *testing.T) {
	t.Run("rung exists in every classification", func(t *testing.T) {
		for _, c := range AllClassifications {
			vine := Vine(c)
			found := false
			for _, r := range vine {
				if r.Name == "retroactive_audit" {
					found = true
					if len(r.Checks) == 0 || r.Checks[0].Name != "closed_issues_pass_gates" {
						t.Errorf("Vine(%s) retroactive_audit rung missing closed_issues_pass_gates check", c)
					}
				}
			}
			if !found {
				t.Errorf("Vine(%s) missing retroactive_audit rung", c)
			}
		}
	})

	t.Run("pass when zero offenders", func(t *testing.T) {
		findings := Evaluate(&ProjectState{HistoricalOffenderCount: 0})
		f, ok := findFinding(findings, "closed_issues_pass_gates")
		if !ok {
			t.Fatal("closed_issues_pass_gates check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("fail summarizes per-gate counts and first offender", func(t *testing.T) {
		state := &ProjectState{
			HistoricalOffenderCount: 4,
			HistoricalOffendersByGate: map[string]int{
				"no-worklog":   2,
				"open-tasks":   1,
				"missing-demo": 3,
			},
			HistoricalOffenderSample: []HistoricalOffender{
				{IssueID: "IS-88", Gate: "no-worklog"},
				{IssueID: "IS-101", Gate: "missing-demo"},
			},
		}
		findings := Evaluate(state)
		f, ok := findFinding(findings, "closed_issues_pass_gates")
		if !ok {
			t.Fatal("closed_issues_pass_gates check not found")
		}
		if f.Status != StatusFail {
			t.Fatalf("want fail, got %s: %s", f.Status, f.Message)
		}
		want := []string{"4 historical close", "2 no-worklog", "1 open-tasks", "3 missing-demo", "IS-88", "no-worklog"}
		for _, w := range want {
			if !strings.Contains(f.Message, w) {
				t.Errorf("message missing %q: %q", w, f.Message)
			}
		}
		if f.FixFunc != nil {
			t.Error("closed_issues_pass_gates must be advisory only (no FixFunc)")
		}
		if f.Proposal != "" {
			t.Errorf("closed_issues_pass_gates must be advisory only (no Proposal), got %q", f.Proposal)
		}
	})
}

// TestShipConfigDefinedInOperationsRung verifies that ship_config_defined appears
// in the operations rung for service and saas classifications.
func TestShipConfigDefinedInOperationsRung(t *testing.T) {
	for _, class := range []Classification{ClassService, ClassSaaS} {
		t.Run(string(class), func(t *testing.T) {
			vine := Vine(class)
			var found bool
			for _, r := range vine {
				if r.Name != "operations" {
					continue
				}
				for _, c := range r.Checks {
					if c.Name == "ship_config_defined" {
						found = true
						if c.Action != ActionPropose {
							t.Errorf("ship_config_defined: want ActionPropose, got %v", c.Action)
						}
					}
				}
			}
			if !found {
				t.Errorf("Vine(%s) operations rung missing ship_config_defined check", class)
			}
		})
	}
}
