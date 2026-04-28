package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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
