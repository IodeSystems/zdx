package doctor

import (
	"testing"
)

func TestClassificationBranchSeeds(t *testing.T) {
	cases := []struct {
		class Classification
		want  []BranchSeed
	}{
		{ClassLibrary, []BranchSeed{{Name: "main", Role: "rolling-release"}}},
		{ClassTool, []BranchSeed{{Name: "main", Role: "rolling-release"}}},
		{ClassSite, []BranchSeed{{Name: "main", Role: "rolling-release"}}},
		{ClassService, []BranchSeed{
			{Name: "dev", Role: "dev"},
			{Name: "main", Role: "rolling-release", SourceBranchName: "dev"},
		}},
		{ClassSaaS, []BranchSeed{
			{Name: "dev", Role: "dev"},
			{Name: "main", Role: "rolling-release", SourceBranchName: "dev"},
		}},
		{Classification(""), nil},
		{Classification("unknown"), nil},
	}
	for _, tc := range cases {
		t.Run(string(tc.class), func(t *testing.T) {
			got := ClassificationBranchSeeds(tc.class)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch for %q: got %d want %d (%+v vs %+v)", tc.class, len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("seed %d for %q: got %+v want %+v", i, tc.class, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestClassificationBranchSeedsCoversAll guards against AllClassifications
// drifting from the seeding switch — every supported classification must
// produce at least one seed row, otherwise dx init would silently no-op.
func TestClassificationBranchSeedsCoversAll(t *testing.T) {
	for _, c := range AllClassifications {
		seeds := ClassificationBranchSeeds(c)
		if len(seeds) == 0 {
			t.Errorf("classification %q has no seed rows", c)
		}
	}
}
