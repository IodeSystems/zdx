package trackermetrics

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFromMapBaseline(t *testing.T) {
	m := FromMetricsMap(map[string]float64{})
	if (m != TrackerMetrics{}) {
		t.Errorf("expected zero TrackerMetrics from empty map, got %+v", m)
	}
}

func TestFromMapAndToMapRoundTrip(t *testing.T) {
	want := TrackerMetrics{
		FeaturesTotal:                       3,
		FeaturesWithSpecs:                   2,
		FeaturesWithDemos:                   1,
		SpecCoverageMustPct:                 75.5,
		SpecCoverageShouldPct:               50,
		SpecCoverageNicePct:                 0,
		SpecsImplementedPerPeriod:           4,
		IssuesClosedPerPeriod:               6,
		MaturityRungsPassed:                 2,
		MaturityRungsFailed:                 1,
		BlockerQuestionAvgResolutionMinutes: 18.5,
	}
	got := FromMetricsMap(ToMetricsMap(want))
	if got != want {
		t.Errorf("round-trip mismatch:\n got  = %+v\n want = %+v", got, want)
	}
}

func TestComputeDeltasBaseline(t *testing.T) {
	curr := TrackerMetrics{FeaturesTotal: 3, IssuesClosedPerPeriod: 5}
	deltas := ComputeDeltas(TrackerMetrics{}, curr)
	if len(deltas) != len(metricSpecs()) {
		t.Fatalf("len(deltas) = %d, want %d", len(deltas), len(metricSpecs()))
	}
	by := indexDeltas(deltas)
	if by["features_total"].Diff != 3 || by["features_total"].Curr != 3 || by["features_total"].Prev != 0 {
		t.Errorf("features_total delta wrong: %+v", by["features_total"])
	}
	if by["issues_closed_per_period"].Diff != 5 {
		t.Errorf("issues_closed_per_period diff wrong: %+v", by["issues_closed_per_period"])
	}
	if by["maturity_rungs_passed"].Diff != 0 {
		t.Errorf("maturity_rungs_passed should be 0 against baseline: %+v", by["maturity_rungs_passed"])
	}
}

func TestComputeDeltasWithPrior(t *testing.T) {
	prev := TrackerMetrics{FeaturesTotal: 2, SpecCoverageMustPct: 50, BlockerQuestionAvgResolutionMinutes: 10}
	curr := TrackerMetrics{FeaturesTotal: 3, SpecCoverageMustPct: 75, BlockerQuestionAvgResolutionMinutes: 8}
	by := indexDeltas(ComputeDeltas(prev, curr))
	if by["features_total"].Diff != 1 {
		t.Errorf("features_total diff wrong: %+v", by["features_total"])
	}
	if by["spec_coverage_must_pct"].Diff != 25 {
		t.Errorf("spec_coverage_must_pct diff wrong: %+v", by["spec_coverage_must_pct"])
	}
	if by["blocker_question_avg_resolution_minutes"].Diff != -2 {
		t.Errorf("avg_resolution diff wrong: %+v", by["blocker_question_avg_resolution_minutes"])
	}
}

func TestParseEmptyAndMalformed(t *testing.T) {
	if _, ok := Parse(""); ok {
		t.Error("Parse(\"\") should return ok=false")
	}
	if _, ok := Parse("{}"); ok {
		t.Error("Parse(\"{}\") should return ok=false (no prior metrics)")
	}
	if _, ok := Parse("not json"); ok {
		t.Error("Parse on garbage should return ok=false")
	}
}

func TestParseRoundTrip(t *testing.T) {
	want := TrackerMetrics{FeaturesTotal: 7, MaturityRungsPassed: 4}
	got, ok := Parse(ToJSON(want))
	if !ok {
		t.Fatal("Parse(ToJSON(...)) should return ok=true")
	}
	if got != want {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, want)
	}
}

// TestComputeDeltasMissingPriorVsZeroStruct: when no prior entry exists the
// caller diffs against an empty TrackerMetrics{} — exercise that path.
func TestComputeDeltasMissingPriorVsZeroStruct(t *testing.T) {
	curr := TrackerMetrics{FeaturesTotal: 5, SpecCoverageNicePct: 12.5}
	from := ComputeDeltas(TrackerMetrics{}, curr)
	prev, ok := Parse("")
	if ok {
		t.Fatal("expected ok=false on empty stateJSON")
	}
	from2 := ComputeDeltas(prev, curr)
	if len(from) != len(from2) {
		t.Fatalf("delta lengths differ: %d vs %d", len(from), len(from2))
	}
	for i := range from {
		if from[i] != from2[i] {
			t.Errorf("delta %d mismatch: %+v vs %+v", i, from[i], from2[i])
		}
	}
}

func TestFormatSummaryStable(t *testing.T) {
	m := TrackerMetrics{FeaturesTotal: 3, SpecCoverageMustPct: 50, BlockerQuestionAvgResolutionMinutes: 12.5}
	deltas := ComputeDeltas(TrackerMetrics{}, m)
	summary := FormatSummary(m, deltas)
	if !strings.Contains(summary, "Features") {
		t.Errorf("summary missing Features label:\n%s", summary)
	}
	if !strings.Contains(summary, "50.00%") {
		t.Errorf("summary missing percent unit:\n%s", summary)
	}
	if !strings.Contains(summary, "12.50 min") {
		t.Errorf("summary missing min unit:\n%s", summary)
	}
}

// TestToJSONIsObject checks the serialized state is a JSON object (not array)
// so it round-trips cleanly through journal StateJson columns.
func TestToJSONIsObject(t *testing.T) {
	s := ToJSON(TrackerMetrics{FeaturesTotal: 1})
	if !strings.HasPrefix(s, "{") {
		t.Errorf("expected JSON object, got %q", s)
	}
	var probe map[string]any
	if err := json.Unmarshal([]byte(s), &probe); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := probe["features_total"]; !ok {
		t.Errorf("missing features_total key in JSON: %s", s)
	}
}

// TestMetricUnitsCoversAllSpecs guards against a metric being added to
// metricSpecs but missed by callers iterating MetricUnits.
func TestMetricUnitsCoversAllSpecs(t *testing.T) {
	units := MetricUnits()
	for _, s := range metricSpecs() {
		if _, ok := units[s.Name]; !ok {
			t.Errorf("MetricUnits missing %q", s.Name)
		}
	}
	if len(units) != len(metricSpecs()) {
		t.Errorf("unit count mismatch: %d vs %d", len(units), len(metricSpecs()))
	}
}

func indexDeltas(d []MetricDelta) map[string]MetricDelta {
	out := make(map[string]MetricDelta, len(d))
	for _, x := range d {
		out[x.Name] = x
	}
	return out
}
