package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

// TestDemoCLI_VitestAdapterForwardsFilter is the demo for spec 38: given a
// filter or feature name, when passed to VitestAdapter, then vitest runs with
// --testNamePattern to filter matching tests.
//
// The demo installs a fake vitest binary that records its argv to a sidecar
// file, then runs VitestAdapter twice — once with Filter.Name and once with
// Filter.Feature — to prove both inputs translate into a --testNamePattern
// flag on the npx vitest invocation.
func TestDemoCLI_VitestAdapterForwardsFilter(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_testharness_vitest_filter_test.go", Note: "vitest filter forwarding demo source"},
		{FilePath: "internal/testharness/vitest.go", LineStart: 30, LineEnd: 41, Note: "vitestRunArgs translates Filter.Name / Filter.Feature into --testNamePattern"},
		{FilePath: "internal/testharness/filter_test.go", LineStart: 43, LineEnd: 64, Note: "unit tests: ForwardsNameFilter / ForwardsFeatureFilter / EmptyFilter_NoPatternFlag"},
	})

	if _, err := exec.LookPath("npx"); err != nil {
		t.Skip("npx not available")
	}

	tmpDir := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(tmpDir, "package.json"),
		[]byte(`{"name":"vitest-filter-demo","private":true}`),
		0644,
	); err != nil {
		t.Fatal(err)
	}

	binDir := filepath.Join(tmpDir, "node_modules", ".bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}

	argvRecord := filepath.Join(tmpDir, "argv.txt")

	// The fake vitest script appends its argv to argvRecord (one invocation per
	// line) and writes an empty but valid JSON report so adapter parsing
	// succeeds without depending on real vitest behaviour.
	fakeVitest := `#!/bin/sh
printf '%s\n' "$*" >> "` + argvRecord + `"
for arg; do
  case "$arg" in --outputFile=*) output="${arg#--outputFile=}" ;; esac
done
echo '{"testResults": []}' > "$output"
`
	vitestBin := filepath.Join(binDir, "vitest")
	if err := os.WriteFile(vitestBin, []byte(fakeVitest), 0755); err != nil {
		t.Fatal(err)
	}

	adapter := &testharness.VitestAdapter{Dir: tmpDir, Comp: "ui"}

	if _, err := adapter.Run(context.Background(), testharness.Filter{Name: "renders correctly"}); err != nil {
		t.Fatalf("VitestAdapter.Run with Name filter: %v", err)
	}
	if _, err := adapter.Run(context.Background(), testharness.Filter{Feature: "login"}); err != nil {
		t.Fatalf("VitestAdapter.Run with Feature filter: %v", err)
	}

	recorded, err := os.ReadFile(argvRecord)
	if err != nil {
		t.Fatalf("read argv record: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(recorded)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 vitest invocations, got %d:\n%s", len(lines), recorded)
	}

	// Filter.Name → --testNamePattern=<name>
	if !strings.Contains(lines[0], "--testNamePattern=renders correctly") {
		t.Errorf("Name filter argv missing --testNamePattern=renders correctly:\n%s", lines[0])
	}
	// Filter.Feature → --testNamePattern=<feature>
	if !strings.Contains(lines[1], "--testNamePattern=login") {
		t.Errorf("Feature filter argv missing --testNamePattern=login:\n%s", lines[1])
	}
}
