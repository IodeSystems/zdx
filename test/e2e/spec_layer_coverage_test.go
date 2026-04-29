package e2e

import (
	"sort"
	"testing"
)

// TestSpecLayerCoverage covers spec 30: when test layers (unit/integration/demo)
// are linked via test-refs, coverage is tracked per spec per layer. Registering
// three tests for one spec — one per layer — should yield three rows from
// /api/dx/specs/tests, one per layer. Unlinking the integration test should
// drop only that layer from the result.
func TestSpecLayerCoverage(t *testing.T) {
	d := NewApiDriver(t, "spec-layer-cov", "Spec Layer Coverage")
	Given(d).
		Feature("layered", "Feature with all-layer coverage").
		Spec("layered", "must", "Layered behavior").
		Build()

	uncovered := d.ListUncoveredSpecs()
	var specID int32
	for _, s := range uncovered {
		if s.FeatureName == "layered" {
			specID = s.ID
		}
	}
	if specID == 0 {
		t.Fatal("expected uncovered spec for feature 'layered'")
	}

	unitID := d.RegisterTestWithLayer("TestLayeredUnit", "unit")
	integID := d.RegisterTestWithLayer("TestLayeredIntegration", "integration")
	demoID := d.RegisterTestWithLayer("TestDemoLayered", "demo")

	d.LinkTestToSpec(specID, unitID)
	d.LinkTestToSpec(specID, integID)
	d.LinkTestToSpec(specID, demoID)

	rows := d.ListSpecTests(specID)
	if len(rows) != 3 {
		t.Fatalf("want 3 spec-tests after linking 3 layers, got %d (%+v)", len(rows), rows)
	}
	gotLayers := layersOf(rows)
	wantLayers := []string{"demo", "integration", "unit"}
	if !equalStrings(gotLayers, wantLayers) {
		t.Errorf("layers: want %v, got %v", wantLayers, gotLayers)
	}

	d.UnlinkTestFromSpec(specID, integID)

	rows = d.ListSpecTests(specID)
	if len(rows) != 2 {
		t.Fatalf("want 2 spec-tests after unlinking integration, got %d (%+v)", len(rows), rows)
	}
	gotLayers = layersOf(rows)
	wantLayers = []string{"demo", "unit"}
	if !equalStrings(gotLayers, wantLayers) {
		t.Errorf("layers after unlink: want %v, got %v", wantLayers, gotLayers)
	}
	for _, r := range rows {
		if r.Layer == "integration" {
			t.Errorf("integration layer should be removed, found row %+v", r)
		}
	}
}

func layersOf(rows []SpecTestRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Layer
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
