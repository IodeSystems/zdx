package e2e

import (
	"testing"
)

// TestDemoCLI_SpecLayerCoverage is the demo for spec 30:
// Given a feature spec, when test layers (unit/integration/demo) are linked
// via test-refs, then coverage is tracked per spec per layer.
//
// Walks the same link/unlink flow as TestSpecLayerCoverage (the integration
// counterpart) but at the demo layer — driving the live devserver via
// ApiDriver. Because the test name is TestDemo*, the server tags its row with
// component=demo, satisfying the GreenDemos condition that flips spec 30
// from planned to implemented.
func TestDemoCLI_SpecLayerCoverage(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_spec_layer_coverage_test.go", Note: "spec 30 demo source"},
		{FilePath: "internal/server/handlers/handlers_features.go", LineStart: 297, LineEnd: 414, Note: "link-test / unlink-test / list-spec-tests handlers (per-layer coverage endpoint)"},
		{FilePath: "internal/db/tests.sql.go", Note: "ListTestsForSpec joins zdx_spec_tests + zdx_tests.layer column"},
		{FilePath: "queries/tests.sql", LineStart: 60, LineEnd: 89, Note: "ListTestsForSpec / LinkSpecTest / UnlinkSpecTest sources"},
	})

	d := NewApiDriver(t, "demo-spec-layer-cov", "Demo Spec Layer Coverage")
	Given(d).
		Feature("layered-demo", "Feature with all-layer coverage (demo)").
		Spec("layered-demo", "must", "Layered behavior — demo").
		Build()

	var specID int32
	for _, s := range d.ListUncoveredSpecs() {
		if s.FeatureName == "layered-demo" {
			specID = s.ID
			break
		}
	}
	if specID == 0 {
		t.Fatal("expected uncovered spec for feature 'layered-demo'")
	}

	unitID := d.RegisterTestWithLayer("TestLayeredUnitDemo", "unit")
	integID := d.RegisterTestWithLayer("TestLayeredIntegrationDemo", "integration")
	demoID := d.RegisterTestWithLayer("TestDemoLayeredDemo", "demo")

	d.LinkTestToSpec(specID, unitID)
	d.LinkTestToSpec(specID, integID)
	d.LinkTestToSpec(specID, demoID)

	rows := d.ListSpecTests(specID)
	if len(rows) != 3 {
		t.Fatalf("want 3 spec-tests after linking 3 layers, got %d (%+v)", len(rows), rows)
	}
	got := layersOf(rows)
	want := []string{"demo", "integration", "unit"}
	if !equalStrings(got, want) {
		t.Errorf("layers: want %v, got %v", want, got)
	}

	d.UnlinkTestFromSpec(specID, integID)

	rows = d.ListSpecTests(specID)
	if len(rows) != 2 {
		t.Fatalf("want 2 spec-tests after unlinking integration, got %d (%+v)", len(rows), rows)
	}
	got = layersOf(rows)
	want = []string{"demo", "unit"}
	if !equalStrings(got, want) {
		t.Errorf("layers after unlink: want %v, got %v", want, got)
	}
	for _, r := range rows {
		if r.Layer == "integration" {
			t.Errorf("integration layer should be removed, found row %+v", r)
		}
	}
}
