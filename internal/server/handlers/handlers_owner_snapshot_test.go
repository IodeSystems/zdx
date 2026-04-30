package handlers

import (
	"testing"

	"github.com/iodesystems/zdx-go/internal/db"
)

func TestPctRound2(t *testing.T) {
	cases := []struct {
		num, denom int64
		want       float64
	}{
		{0, 0, 0},     // empty denom — guard against /0
		{0, 5, 0},     // 0 implemented
		{5, 5, 100},   // full coverage
		{1, 3, 33.33}, // rounds half-up to 2 decimals
		{2, 3, 66.67},
		{1, 7, 14.29},
	}
	for _, c := range cases {
		got := pctRound2(c.num, c.denom)
		if got != c.want {
			t.Errorf("pctRound2(%d, %d) = %v, want %v", c.num, c.denom, got, c.want)
		}
	}
}

func TestRound2(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0, 0},
		{1.234, 1.23},
		{1.235, 1.24}, // banker's rounding not required — math.Round is half-away
		{12345.678, 12345.68},
	}
	for _, c := range cases {
		if got := round2(c.in); got != c.want {
			t.Errorf("round2(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestBuildOwnerSnapshotMetrics covers the metric-assembly contract from
// IS-589: every documented metric key must be present, percentages must round
// to 2 decimals, and a fully-empty project must produce an all-zero map.
func TestBuildOwnerSnapshotMetrics(t *testing.T) {
	t.Run("empty project", func(t *testing.T) {
		m := buildOwnerSnapshotMetrics(
			db.OwnerSnapshotFeaturesRow{},
			db.OwnerSnapshotSpecCoverageRow{},
			db.OwnerSnapshotPeriodRow{},
			db.OwnerSnapshotMaturityRow{},
			0,
		)
		for _, k := range expectedOwnerSnapshotKeys() {
			if _, ok := m[k]; !ok {
				t.Errorf("missing key %q in baseline map", k)
			}
			if m[k] != 0 {
				t.Errorf("key %q on baseline = %v, want 0", k, m[k])
			}
		}
	})

	t.Run("populated", func(t *testing.T) {
		m := buildOwnerSnapshotMetrics(
			db.OwnerSnapshotFeaturesRow{FeaturesTotal: 3, FeaturesWithSpecs: 1, FeaturesWithDemos: 1},
			db.OwnerSnapshotSpecCoverageRow{
				SpecsMustTotal: 2, SpecsMustImplemented: 1,
				SpecsShouldTotal: 2, SpecsShouldImplemented: 2,
				SpecsNiceTotal: 1, SpecsNiceImplemented: 0,
			},
			db.OwnerSnapshotPeriodRow{SpecsImplementedInPeriod: 3, IssuesClosedInPeriod: 4},
			db.OwnerSnapshotMaturityRow{RungsPassed: 2, RungsFailed: 1},
			18.456,
		)
		want := map[string]float64{
			"features_total":                          3,
			"features_with_specs":                     1,
			"features_with_demos":                     1,
			"spec_coverage_must_pct":                  50,
			"spec_coverage_should_pct":                100,
			"spec_coverage_nice_pct":                  0,
			"specs_implemented_per_period":            3,
			"issues_closed_per_period":                4,
			"maturity_rungs_passed":                   2,
			"maturity_rungs_failed":                   1,
			"blocker_question_avg_resolution_minutes": 18.46,
		}
		for k, v := range want {
			if got := m[k]; got != v {
				t.Errorf("metric %q = %v, want %v", k, got, v)
			}
		}
		if len(m) != len(want) {
			t.Errorf("unexpected keys: got %d, want %d (%v)", len(m), len(want), keys(m))
		}
	})
}

// expectedOwnerSnapshotKeys is the IS-589 contract — keep aligned with
// trackermetrics.TrackerMetrics struct tags.
func expectedOwnerSnapshotKeys() []string {
	return []string{
		"features_total",
		"features_with_specs",
		"features_with_demos",
		"spec_coverage_must_pct",
		"spec_coverage_should_pct",
		"spec_coverage_nice_pct",
		"specs_implemented_per_period",
		"issues_closed_per_period",
		"maturity_rungs_passed",
		"maturity_rungs_failed",
		"blocker_question_avg_resolution_minutes",
	}
}

func keys(m map[string]float64) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
