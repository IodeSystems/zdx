package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

// TestDemoCLI_VitestAdapterParsesOutput is the demo for spec 37: given vitest is
// configured in the UI project, when VitestAdapter runs, then vitest JSON output
// is parsed into testharness Result format.
//
// The demo installs a fake vitest binary that emits a fixture JSON report, then
// runs VitestAdapter directly to prove that the parsing layer maps vitest
// "passed"/"failed"/"skipped" → testharness "pass"/"fail"/"skip" and attaches
// the correct Component and Layer to every Result.
func TestDemoCLI_VitestAdapterParsesOutput(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_testharness_vitest_test.go", Note: "vitest adapter parsing demo source"},
		{FilePath: "internal/testharness/vitest.go", LineStart: 84, LineEnd: 117, Note: "parseVitestReport maps vitest JSON to testharness Result"},
		{FilePath: "internal/testharness/vitest.go", LineStart: 44, LineEnd: 63, Note: "VitestAdapter.Run writes JSON report and delegates to parseVitestReport"},
	})

	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not available")
	}

	// Build a temp project directory with a fake vitest binary that emits a
	// fixture JSON report without requiring an actual Node.js project.
	tmpDir := t.TempDir()

	if err := os.WriteFile(
		filepath.Join(tmpDir, "package.json"),
		[]byte(`{"name":"vitest-demo","private":true}`),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(tmpDir, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	// The fake vitest script: parse --outputFile=<path> from argv, write fixture JSON.
	const fakeVitest = `#!/bin/sh
for arg; do
  case "$arg" in --outputFile=*) output="${arg#--outputFile=}" ;; esac
done
cat > "$output" <<'ENDJSON'
{
  "testResults": [
    {
      "name": "/app/ui/src/foo.test.ts",
      "status": "failed",
      "assertionResults": [
        {"fullName": "foo renders correctly", "status": "passed",  "duration": 12.0, "failureMessages": []},
        {"fullName": "foo fails on bad input","status": "failed",  "duration":  3.0, "failureMessages": ["Expected true, got false"]},
        {"fullName": "foo skips in CI",       "status": "skipped", "duration":  0.0, "failureMessages": []}
      ]
    }
  ]
}
ENDJSON
`
	vitestBin := filepath.Join(binDir, "vitest")
	if err := os.WriteFile(vitestBin, []byte(fakeVitest), 0755); err != nil {
		t.Fatal(err)
	}

	adapter := &testharness.VitestAdapter{Dir: tmpDir, Comp: "ui"}
	results, err := adapter.Run(context.Background(), testharness.Filter{})
	if err != nil {
		t.Fatalf("VitestAdapter.Run: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify status mapping: vitest "passed" → testharness "pass"
	if got := results[0].Status; got != "pass" {
		t.Errorf("results[0].Status = %q, want pass", got)
	}
	// Verify failure output is captured
	if got := results[1].Status; got != "fail" {
		t.Errorf("results[1].Status = %q, want fail", got)
	}
	if results[1].Output == "" {
		t.Errorf("results[1].Output empty, want failure message")
	}
	// Verify status mapping: vitest "skipped" → testharness "skip"
	if got := results[2].Status; got != "skip" {
		t.Errorf("results[2].Status = %q, want skip", got)
	}

	// Every result must carry the adapter's component and unit layer.
	for i, r := range results {
		if r.Component != "ui" {
			t.Errorf("results[%d].Component = %q, want ui", i, r.Component)
		}
		if r.Layer != testharness.LayerUnit {
			t.Errorf("results[%d].Layer = %q, want unit", i, r.Layer)
		}
	}
}
