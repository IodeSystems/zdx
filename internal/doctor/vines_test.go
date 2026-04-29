package doctor

import (
	"testing"
)

// TestVine verifies spec 127: Vine() composes the five common rungs (in order)
// followed by the classification-specific rung(s) for each classification.
func TestVine(t *testing.T) {
	commonRungs := []string{"scaffold", "identity", "planning", "verification", "agents"}

	cases := []struct {
		class    Classification
		extra    []string
		minTotal int
	}{
		{ClassLibrary, []string{"distribution"}, 6},
		{ClassTool, []string{"distribution"}, 6},
		{ClassService, []string{"operations"}, 6},
		{ClassSaaS, []string{"operations", "multi-tenancy"}, 7},
		{ClassSite, []string{"publication"}, 6},
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
