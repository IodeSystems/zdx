// Package trackermetrics is the owner-side mirror of internal/techmetrics:
// it pulls the IS-589 tracker-state metric set from the dx-server and shapes
// it for journal StateJson + KPI sampling.
//
// Where techmetrics measures the codebase (Go/TS LOC, git churn, test count),
// trackermetrics measures the tracker (features, specs, issues, blockers,
// maturity) — i.e. the "what is the project tracking" surface the owner
// reviews on each standup.
package trackermetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// TrackerMetrics mirrors the OwnerSnapshotBody.Metrics map as a typed struct
// for journal persistence and delta computation. JSON tags match the snapshot
// endpoint keys so a round-trip through StateJson is lossless.
type TrackerMetrics struct {
	FeaturesTotal                       int     `json:"features_total"`
	FeaturesWithSpecs                   int     `json:"features_with_specs"`
	FeaturesWithDemos                   int     `json:"features_with_demos"`
	SpecCoverageMustPct                 float64 `json:"spec_coverage_must_pct"`
	SpecCoverageShouldPct               float64 `json:"spec_coverage_should_pct"`
	SpecCoverageNicePct                 float64 `json:"spec_coverage_nice_pct"`
	SpecsImplementedPerPeriod           int     `json:"specs_implemented_per_period"`
	IssuesClosedPerPeriod               int     `json:"issues_closed_per_period"`
	MaturityRungsPassed                 int     `json:"maturity_rungs_passed"`
	MaturityRungsFailed                 int     `json:"maturity_rungs_failed"`
	BlockerQuestionAvgResolutionMinutes float64 `json:"blocker_question_avg_resolution_minutes"`
}

// MetricDelta records a metric's prior/current/diff for journal changelog.
// Mirrors techmetrics.MetricDelta, with a float64 axis to accommodate
// percentages and the resolution-time average.
type MetricDelta struct {
	Name string  `json:"name"`
	Prev float64 `json:"prev"`
	Curr float64 `json:"curr"`
	Diff float64 `json:"diff"`
}

// metricSpec is one snapshot field's wire identity + display label + unit.
// Owns the canonical ordering — used by Collect, ComputeDeltas, FormatSummary,
// and the kpi-sample emission so they cannot drift from each other.
type metricSpec struct {
	Name  string
	Label string
	Unit  string // "count" | "percent" | "min"
}

func metricSpecs() []metricSpec {
	return []metricSpec{
		{"features_total", "Features", "count"},
		{"features_with_specs", "Features w/ specs", "count"},
		{"features_with_demos", "Features w/ demos", "count"},
		{"spec_coverage_must_pct", "Must spec coverage", "percent"},
		{"spec_coverage_should_pct", "Should spec coverage", "percent"},
		{"spec_coverage_nice_pct", "Nice spec coverage", "percent"},
		{"specs_implemented_per_period", "Specs implemented", "count"},
		{"issues_closed_per_period", "Issues closed", "count"},
		{"maturity_rungs_passed", "Maturity passed", "count"},
		{"maturity_rungs_failed", "Maturity failed", "count"},
		{"blocker_question_avg_resolution_minutes", "Avg blocker resolution", "min"},
	}
}

// MetricUnits returns name→unit for every snapshot key. Used by callers that
// need to emit per-metric kpi samples with the correct unit string.
func MetricUnits() map[string]string {
	out := map[string]string{}
	for _, s := range metricSpecs() {
		out[s.Name] = s.Unit
	}
	return out
}

// Collect pulls the snapshot from the server and maps it into a typed struct.
// Returns the zero-valued TrackerMetrics on any error (caller logs + skips).
func Collect(ctx context.Context, c *cli.Client, slug string, periodDays int) (TrackerMetrics, error) {
	period := int32(periodDays)
	resp, err := c.OwnerSnapshotWithResponse(ctx, &dxclient.OwnerSnapshotParams{
		Slug:       slug,
		PeriodDays: &period,
	})
	if err != nil {
		return TrackerMetrics{}, err
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return TrackerMetrics{}, err
	}
	if resp.JSON200 == nil {
		return TrackerMetrics{}, fmt.Errorf("owner snapshot: empty response")
	}
	return fromMap(resp.JSON200.Metrics), nil
}

// FromMetricsMap exposes the wire→struct conversion for tests and callers
// that already have the metric map in hand.
func FromMetricsMap(m map[string]float64) TrackerMetrics {
	return fromMap(m)
}

func fromMap(m map[string]float64) TrackerMetrics {
	get := func(k string) float64 { return m[k] }
	return TrackerMetrics{
		FeaturesTotal:                       int(get("features_total")),
		FeaturesWithSpecs:                   int(get("features_with_specs")),
		FeaturesWithDemos:                   int(get("features_with_demos")),
		SpecCoverageMustPct:                 get("spec_coverage_must_pct"),
		SpecCoverageShouldPct:               get("spec_coverage_should_pct"),
		SpecCoverageNicePct:                 get("spec_coverage_nice_pct"),
		SpecsImplementedPerPeriod:           int(get("specs_implemented_per_period")),
		IssuesClosedPerPeriod:               int(get("issues_closed_per_period")),
		MaturityRungsPassed:                 int(get("maturity_rungs_passed")),
		MaturityRungsFailed:                 int(get("maturity_rungs_failed")),
		BlockerQuestionAvgResolutionMinutes: get("blocker_question_avg_resolution_minutes"),
	}
}

// ToMetricsMap inverts fromMap — used to drive per-metric kpi samples in the
// caller without re-listing every field.
func ToMetricsMap(m TrackerMetrics) map[string]float64 {
	return map[string]float64{
		"features_total":                          float64(m.FeaturesTotal),
		"features_with_specs":                     float64(m.FeaturesWithSpecs),
		"features_with_demos":                     float64(m.FeaturesWithDemos),
		"spec_coverage_must_pct":                  m.SpecCoverageMustPct,
		"spec_coverage_should_pct":                m.SpecCoverageShouldPct,
		"spec_coverage_nice_pct":                  m.SpecCoverageNicePct,
		"specs_implemented_per_period":            float64(m.SpecsImplementedPerPeriod),
		"issues_closed_per_period":                float64(m.IssuesClosedPerPeriod),
		"maturity_rungs_passed":                   float64(m.MaturityRungsPassed),
		"maturity_rungs_failed":                   float64(m.MaturityRungsFailed),
		"blocker_question_avg_resolution_minutes": m.BlockerQuestionAvgResolutionMinutes,
	}
}

// ComputeDeltas pairs prev with curr in canonical order and emits one
// MetricDelta per snapshot key.
func ComputeDeltas(prev, curr TrackerMetrics) []MetricDelta {
	prevMap := ToMetricsMap(prev)
	currMap := ToMetricsMap(curr)
	out := make([]MetricDelta, 0, len(metricSpecs()))
	for _, s := range metricSpecs() {
		p := prevMap[s.Name]
		c := currMap[s.Name]
		out = append(out, MetricDelta{Name: s.Name, Prev: p, Curr: c, Diff: c - p})
	}
	return out
}

// ToJSON serialises a TrackerMetrics to JSON (StateJson payload).
func ToJSON(m TrackerMetrics) string {
	b, _ := json.Marshal(m)
	return string(b)
}

// DeltasToJSON serialises a delta slice (ChangelogJson payload).
func DeltasToJSON(d []MetricDelta) string {
	b, _ := json.Marshal(d)
	return string(b)
}

// Parse hydrates a TrackerMetrics from a StateJson string. Returns ok=false
// when the input is empty or malformed so callers can treat the entry as a
// baseline (no prior metrics to diff against).
func Parse(stateJSON string) (TrackerMetrics, bool) {
	if stateJSON == "" || stateJSON == "{}" {
		return TrackerMetrics{}, false
	}
	var m TrackerMetrics
	if err := json.Unmarshal([]byte(stateJSON), &m); err != nil {
		return TrackerMetrics{}, false
	}
	return m, true
}

// FormatSummary renders one line per metric with its current value and a
// signed delta in parentheses when non-zero. Mirrors techmetrics.FormatSummary.
func FormatSummary(m TrackerMetrics, deltas []MetricDelta) string {
	specMap := map[string]metricSpec{}
	for _, s := range metricSpecs() {
		specMap[s.Name] = s
	}
	var sb strings.Builder
	for _, d := range deltas {
		spec := specMap[d.Name]
		valStr := formatValue(d.Curr, spec.Unit)
		diffStr := ""
		if (d.Prev != 0 || d.Curr != 0) && d.Diff != 0 {
			sign := ""
			if d.Diff > 0 {
				sign = "+"
			}
			diffStr = fmt.Sprintf(" (%s%s)", sign, formatValue(d.Diff, spec.Unit))
		}
		sb.WriteString(fmt.Sprintf("  %-25s %s%s\n", metricLabel(d.Name), valStr, diffStr))
	}
	return sb.String()
}

func metricLabel(name string) string {
	for _, s := range metricSpecs() {
		if s.Name == name {
			return s.Label
		}
	}
	return name
}

func formatValue(v float64, unit string) string {
	switch unit {
	case "percent":
		return fmt.Sprintf("%.2f%%", v)
	case "min":
		return fmt.Sprintf("%.2f min", v)
	default:
		// integer counts
		return fmt.Sprintf("%d", int(v))
	}
}
