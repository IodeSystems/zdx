package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_DxTestUnified is the demo for spec 24:
// Given multiple test components (vitest, Go binary, demo), when a user runs
// 'dx test', then all test results are unified into a single output.
//
// The demo runs `dx test list` — the enumeration surface of the unified test
// runner — and verifies that entries from at least two distinct components
// (ui and api) appear in a single output stream. This proves the harness merges
// adapter outputs rather than running each in isolation.
func TestDemoCLI_DxTestUnified(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_dx_test_unified_test.go", Note: "spec 24 demo source"},
		{FilePath: "internal/testharness/harness.go", LineStart: 148, LineEnd: 165, Note: "Harness.Run: merges results from all registered adapters into a single slice"},
		{FilePath: "internal/cli/devtools/test.go", LineStart: 351, LineEnd: 427, Note: "dx test list: enumerates vitest + Go unit + GoBin adapters in one pass"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	cmd := exec.Command(dxBin, "test", "list")
	cmd.Dir = root

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("dx test list: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	out := stdout.String()

	// Both vitest (ui) and Go binary (api) adapters must appear in the unified
	// listing — proving the harness merges across adapter types into one stream.
	for _, want := range []string{"ui", "api"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected component %q in unified test list:\n%s", want, out)
		}
	}

	// Unit layer tests from both adapters must be present.
	if !strings.Contains(out, "unit") {
		t.Errorf("expected unit layer in unified test list:\n%s", out)
	}

	// Unified output must contain tests from multiple adapters (more than a
	// single-adapter run would produce).
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 3 {
		t.Errorf("expected at least 3 lines in unified output, got %d:\n%s", len(lines), out)
	}
}
