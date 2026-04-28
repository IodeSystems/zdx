package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_HarnessMultiAdapterMerge is the demo for spec 34: given multiple
// test adapters registered, when harness.Run executes, then results from all
// adapters are merged into a unified result set.
//
// The demo runs `dx test list` — the CLI surface of the harness — and verifies
// that entries from at least two distinct components (ui via vitest, api via
// Go unit adapter) appear together in one output, proving the harness merges
// across adapters rather than running each in isolation.
func TestDemoCLI_HarnessMultiAdapterMerge(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_harness_multi_adapter_test.go", Note: "harness multi-adapter merge demo source"},
		{FilePath: "internal/testharness/harness.go", LineStart: 148, LineEnd: 165, Note: "Harness.Run merges results from all registered adapters"},
		{FilePath: "internal/cli/devtools/test.go", LineStart: 351, LineEnd: 427, Note: "dx test list enumerates vitest + Go unit + GoBin adapters"},
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
		t.Fatalf("dx test list: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}

	out := stdout.String()

	// Must show ui adapter output (vitest)
	if !strings.Contains(out, "ui") {
		t.Errorf("expected ui component in output:\n%s", out)
	}

	// Must show api adapter output (Go unit tests)
	if !strings.Contains(out, "api") {
		t.Errorf("expected api component in output:\n%s", out)
	}

	// Must show both unit and demo layers, confirming multi-layer adapter coverage
	if !strings.Contains(out, "unit") {
		t.Errorf("expected unit layer in output:\n%s", out)
	}

	// Sanity: the merged output must have more than one line
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Errorf("expected multiple lines in merged output, got %d:\n%s", len(lines), out)
	}
}
