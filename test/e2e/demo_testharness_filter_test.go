package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

// TestDemoCLI_FilterScopesAllAdapters is the demo for spec 35: given a filter
// pattern, when passed to harness.Run, then only matching tests across all
// adapters execute.
//
// The demo compiles two throwaway Go test binaries — one for "api" and one for
// "worker" — each containing one matching test and one non-matching test. It
// registers both as GoBinAdapters on a Harness, then calls harness.Run with
// Filter{Name: "MatchFilter"}. The assertions confirm:
//
//   - Results come from BOTH adapters (filter is forwarded to all, not just one).
//   - Only the matching test ("TestMatchFilter*") appears in each adapter's output.
//   - The non-matching test ("TestSkip*") is absent from all results.
func TestDemoCLI_FilterScopesAllAdapters(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_testharness_filter_test.go", Note: "filter-scopes-all-adapters demo source"},
		{FilePath: "internal/testharness/harness.go", LineStart: 149, LineEnd: 165, Note: "Harness.Run loops all adapters and passes Filter unchanged"},
		{FilePath: "internal/testharness/gobin.go", LineStart: 54, LineEnd: 64, Note: "gobinRunArgs: Filter.Name → -test.run pattern"},
	})

	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain not available")
	}

	apiBin := buildFilterTestBin(t, "api", "Api")
	workerBin := buildFilterTestBin(t, "worker", "Worker")

	h := testharness.New()
	h.Register(&testharness.GoBinAdapter{Bin: apiBin, Comp: "api"})
	h.Register(&testharness.GoBinAdapter{Bin: workerBin, Comp: "worker"})

	results, err := h.Run(context.Background(), testharness.Filter{Name: "MatchFilter"})
	if err != nil {
		t.Fatalf("harness.Run: %v", err)
	}

	// Both adapters must contribute results — filter is forwarded to all, not skipped.
	components := map[string]bool{}
	for _, r := range results {
		components[r.Component] = true
	}
	for _, comp := range []string{"api", "worker"} {
		if !components[comp] {
			t.Errorf("no results from adapter %q — filter was not forwarded to it", comp)
		}
	}

	// Only the matching test must appear; non-matching tests must be absent.
	for _, r := range results {
		if !strings.Contains(r.Test, "MatchFilter") {
			t.Errorf("unexpected result %q from %q — filter should have excluded it", r.Test, r.Component)
		}
	}
	for _, r := range results {
		if strings.Contains(r.Test, "TestSkip") {
			t.Errorf("non-matching test %q from %q appeared despite filter", r.Test, r.Component)
		}
	}
}

// buildFilterTestBin compiles a tiny test binary for component comp with
// two tests: one that matches "MatchFilter" and one that does not ("TestSkip*").
func buildFilterTestBin(t *testing.T, comp, suffix string) string {
	t.Helper()

	dir := t.TempDir()
	src := fmt.Sprintf(`package filter_test
import "testing"
func TestMatchFilter%s(t *testing.T) {}
func TestSkip%s(t *testing.T)        {}
`, suffix, suffix)

	if err := os.WriteFile(filepath.Join(dir, "filter_test.go"), []byte(src), 0644); err != nil {
		t.Fatalf("write test source for %s: %v", comp, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module filterdemo\ngo 1.21\n"), 0644); err != nil {
		t.Fatalf("write go.mod for %s: %v", comp, err)
	}

	bin := filepath.Join(t.TempDir(), comp+".test")
	cmd := exec.Command("go", "test", "-c", "-o", bin, ".")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile test bin for %s: %v\n%s", comp, err, out)
	}
	return bin
}
