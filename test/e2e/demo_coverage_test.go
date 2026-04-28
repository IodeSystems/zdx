package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

// TestDemoCLI_GoCoverDirCollection is the demo for spec 32:
// Given a Go binary built with -cover, when tests run with GOCOVERDIR set,
// then coverage data is collected in .zdx/coverage/.
//
// The demo builds a coverage-instrumented binary from the testharness package,
// runs it with GOCOVERDIR pointing to .zdx/coverage/, and asserts that raw
// coverage data files are written. This mirrors exactly what GoBinAdapter does
// when dx test --coverage is invoked.
func TestDemoCLI_GoCoverDirCollection(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_coverage_test.go", Note: "coverage collection demo (spec 32)"},
		{FilePath: "internal/testharness/gobin.go", LineStart: 68, LineEnd: 77, Note: "GoBinAdapter injects -test.gocoverdir and GOCOVERDIR"},
		{FilePath: "internal/cli/devtools/test.go", LineStart: 211, LineEnd: 226, Note: "dx test --coverage: MergeCoverage → CoverageReport pipeline"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}

	// Build a coverage-instrumented test binary from the testharness package.
	bin := filepath.Join(t.TempDir(), "testharness.test")
	build := exec.Command("go", "test", "-c", "-cover", "-o", bin,
		"github.com/iodesystems/zdx-go/internal/testharness")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go test -c -cover: %v\n%s", err, out)
	}

	// Simulate the .zdx/coverage/ path that dx test --coverage creates.
	projectDir := t.TempDir()
	coverDir := filepath.Join(projectDir, ".zdx", "coverage")
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx/coverage/: %v", err)
	}

	// Run the binary with GOCOVERDIR set to .zdx/coverage/.
	// -test.gocoverdir is required for go test -c -cover binaries (Go 1.20+);
	// GOCOVERDIR env covers go build -cover binaries. Both are set so the
	// mechanism works for either binary type, exactly as GoBinAdapter does.
	run := exec.Command(bin,
		"-test.run=TestHarness",
		"-test.gocoverdir="+coverDir,
	)
	run.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	// Ignore exit code: coverage files are written regardless of test pass/fail.
	run.CombinedOutput() //nolint:errcheck

	// Spec 32 contract: GOCOVERDIR must be populated with raw coverage data.
	entries, err := os.ReadDir(coverDir)
	if err != nil {
		t.Fatalf("ReadDir .zdx/coverage/: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal(".zdx/coverage/ not populated: no coverage files produced (GOCOVERDIR not respected)")
	}
	t.Logf("coverage data in .zdx/coverage/: %d file(s)", len(entries))
}

// TestDemoCLI_GoCovdataMerge is the demo for spec 33:
// Given collected coverage data, when merged via go tool covdata, then a
// unified coverage profile and HTML report are generated.
//
// The demo builds a coverage-instrumented binary, runs it to populate a raw
// GOCOVERDIR directory (the precondition from spec 32), then calls
// MergeCoverage (go tool covdata textfmt) and CoverageReport (go tool cover -html)
// to assert that both output files are produced — exactly what dx test --coverage does.
func TestDemoCLI_GoCovdataMerge(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_coverage_test.go", Note: "coverage merge demo (spec 33)"},
		{FilePath: "internal/testharness/gobin.go", LineStart: 178, LineEnd: 198, Note: "MergeCoverage + CoverageReport implementations"},
		{FilePath: "internal/cli/devtools/test.go", LineStart: 211, LineEnd: 226, Note: "dx test --coverage: MergeCoverage → CoverageReport pipeline"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}

	// Build a coverage-instrumented test binary.
	bin := filepath.Join(t.TempDir(), "testharness.test")
	build := exec.Command("go", "test", "-c", "-cover", "-o", bin,
		"github.com/iodesystems/zdx-go/internal/testharness")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go test -c -cover: %v\n%s", err, out)
	}

	// Populate raw GOCOVERDIR data (precondition: spec 32).
	coverDir := filepath.Join(t.TempDir(), ".zdx", "coverage", "api")
	if err := os.MkdirAll(coverDir, 0755); err != nil {
		t.Fatalf("mkdir coverDir: %v", err)
	}
	run := exec.Command(bin, "-test.run=TestHarness", "-test.gocoverdir="+coverDir)
	run.Env = append(os.Environ(), "GOCOVERDIR="+coverDir)
	run.CombinedOutput() //nolint:errcheck

	entries, err := os.ReadDir(coverDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("precondition failed: GOCOVERDIR not populated (%v)", err)
	}

	// Spec 33: merge raw data → unified text profile.
	outDir := filepath.Join(t.TempDir(), ".zdx", "coverage")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		t.Fatalf("mkdir outDir: %v", err)
	}
	profile := filepath.Join(outDir, "api.txt")
	if err := testharness.MergeCoverage(coverDir, profile); err != nil {
		t.Fatalf("MergeCoverage: %v", err)
	}
	data, err := os.ReadFile(profile)
	if err != nil || len(data) == 0 {
		t.Fatalf("api.txt missing or empty: %v", err)
	}

	// Spec 33: render HTML report from unified profile.
	html := filepath.Join(outDir, "api.html")
	if err := testharness.CoverageReport(profile, html); err != nil {
		t.Fatalf("CoverageReport: %v", err)
	}
	htmlData, err := os.ReadFile(html)
	if err != nil {
		t.Fatalf("api.html not created: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(htmlData)), "coverage") {
		t.Error("html report missing 'coverage'")
	}
	t.Logf("unified profile: %s (%d bytes), html report: %s (%d bytes)",
		profile, len(data), html, len(htmlData))
}
