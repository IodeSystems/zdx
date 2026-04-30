package doctor

import (
	"slices"
	"testing"
)

// TestVine verifies spec 127: Vine() composes the six common rungs (in order)
// followed by the classification-specific rung(s) for each classification.
func TestVine(t *testing.T) {
	commonRungs := []string{"scaffold", "identity", "planning", "concerns", "verification", "agents"}

	cases := []struct {
		class    Classification
		extra    []string
		minTotal int
	}{
		{ClassLibrary, []string{"distribution"}, 7},
		{ClassTool, []string{"distribution"}, 7},
		{ClassService, []string{"operations"}, 7},
		{ClassSaaS, []string{"operations", "multi-tenancy"}, 8},
		{ClassSite, []string{"publication"}, 7},
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
