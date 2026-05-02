package doctor

// BranchSeed is one row to insert into zdx_version_branches when seeding a
// project's branch vine from its classification.
type BranchSeed struct {
	Name             string
	Role             string
	SourceBranchName string
}

// ClassificationBranchSeeds returns the auto-seed rows for a classification.
// Library / tool / site start at the rung-2 minimum (a single rolling-release
// "main"); service / saas start at rung 3 with a dev branch and a main that
// rolls forward from dev. Empty / unknown classifications return nil — the
// caller treats that as "not ready to seed."
func ClassificationBranchSeeds(c Classification) []BranchSeed {
	switch c {
	case ClassLibrary, ClassTool, ClassSite:
		return []BranchSeed{
			{Name: "main", Role: "rolling-release"},
		}
	case ClassService, ClassSaaS:
		return []BranchSeed{
			{Name: "dev", Role: "dev"},
			{Name: "main", Role: "rolling-release", SourceBranchName: "dev"},
		}
	default:
		return nil
	}
}
