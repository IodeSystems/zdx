package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_InitScaffoldsAndRunsDoctor is the demo for spec 135:
// Given a new project, when dx init runs, it scaffolds .zdx/config.yaml
// (and optionally a dev container), then runs doctor --fix which auto-applies
// scaffold fixes and proposes identity items (vision, classification, goals).
//
// The demo runs `dx init --no-container` in an empty temp dir so we can
// inspect the scaffold output and the doctor proposals without network
// access or a live server. The unit tests in servercmd/init_test.go cover
// the with-container variant and idempotent re-runs.
func TestDemoCLI_InitScaffoldsAndRunsDoctor(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_init_test.go", Note: "dx init demo source (spec 135)"},
		{FilePath: "internal/cli/servercmd/init.go", LineStart: 19, LineEnd: 117, Note: "InitCmd: scaffold + optional container + doctor --fix"},
		{FilePath: "internal/cli/servercmd/init.go", LineStart: 119, LineEnd: 190, Note: "scaffoldZdx: creates .zdx/config.yaml and appends .gitignore entries"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	tmp := t.TempDir()

	// git init so scaffoldZdx prints the bind-to-zdx tip (cosmetic) and the
	// .gitignore path is consistent.
	if err := exec.Command("git", "init", tmp).Run(); err != nil {
		t.Logf("git init: %v (continuing)", err)
	}

	cmd := exec.Command(dxBin, "init", "--no-container")
	cmd.Dir = tmp
	// "tool\n" answers the classification prompt from doctor; subsequent
	// ReadString calls hit EOF and are treated as skip/no-op.
	cmd.Stdin = strings.NewReader("tool\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("dx init: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
	}
	out := stdout.String()

	// Spec 135 contract 1: .zdx/config.yaml must be created with skeleton blocks.
	data, err := os.ReadFile(filepath.Join(tmp, ".zdx", "config.yaml"))
	if err != nil {
		t.Fatalf(".zdx/config.yaml not created: %v", err)
	}
	for _, marker := range []string{"hooks:", "components:"} {
		if !strings.Contains(string(data), marker) {
			t.Errorf(".zdx/config.yaml missing %q block", marker)
		}
	}

	// Spec 135 contract 2: doctor runs after scaffold (with --fix).
	if !strings.Contains(out, "running project health check") {
		t.Errorf("expected doctor health check output; stdout:\n%s", out)
	}

	// Spec 135 contract 3: doctor proposes identity items the user must fill in.
	for _, proposal := range []string{"dx vision set", "dx goal add"} {
		if !strings.Contains(out, proposal) {
			t.Errorf("expected identity proposal %q in output:\n%s", proposal, out)
		}
	}
}
