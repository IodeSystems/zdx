package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_DoctorRungOrdering is the demo for spec 132:
// Given doctor produces findings across multiple rungs, when output is rendered,
// then findings are grouped and ordered by rung (scaffold → identity → planning →
// verification → agents → classification-specific) so the user sees foundational
// issues before higher-rung ones.
//
// Setup: a temp dir with .zdx/config.yaml pointing to an unreachable server and
// .zdx/credentials with a fake key. Scaffold checks pass locally; all remote-derived
// state is absent, so identity/planning/verification/agents all produce findings.
// "tool\n" answers the classification prompt (server unreachable → no remote class).
func TestDemoCLI_DoctorRungOrdering(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_rung_order_test.go", Note: "demo source (spec 132)"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 122, LineEnd: 158, Note: "render loop: groups findings by rung, prints ── {rung} ── header on transition"},
		{FilePath: "internal/doctor/vines.go", LineStart: 83, LineEnd: 137, Note: "commonVine(): canonical rung order scaffold→identity→planning→verification→agents"},
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

	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	configYAML := "remote:\n  url: \"http://localhost:19999\"\n  slug: \"demo-rung-order\"\nhooks: {}\ncomponents: {}\n"
	if err := os.WriteFile(filepath.Join(zdxDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zdxDir, "credentials"), []byte("fake-api-key\n"), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	cmd := exec.Command(dxBin, "doctor")
	cmd.Dir = tmp
	cmd.Stdin = strings.NewReader("tool\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	runErr := cmd.Run()
	dur := time.Since(start).Milliseconds()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	t.Cleanup(func() {
		dir := filepath.Join(root, ".zdx", "demo", "cli")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Logf("demo save: mkdir: %v", err)
			return
		}
		log := demoLog{
			Name:       t.Name(),
			RecordedAt: time.Now().UTC().Format(time.RFC3339),
			Steps: []demoStep{{
				Cmd:        "dx doctor",
				Args:       []string{"dx", "doctor"},
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				ExitCode:   exitCode,
				DurationMs: dur,
			}},
		}
		b, _ := json.MarshalIndent(log, "", "  ")
		path := filepath.Join(dir, t.Name()+".json")
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Logf("demo save: %v", err)
			return
		}
		t.Logf("CLI demo saved → %s", path)
	})

	out := stdout.String()

	// Collect rung header positions in output order.
	canonicalRungs := []string{"scaffold", "identity", "planning", "verification", "agents"}
	rungPos := make(map[string]int, len(canonicalRungs))
	for _, line := range strings.Split(out, "\n") {
		for _, rung := range canonicalRungs {
			header := "── " + rung + " ──"
			if strings.Contains(line, header) {
				if _, seen := rungPos[rung]; !seen {
					rungPos[rung] = len(rungPos)
				}
			}
		}
	}

	// Spec 132 contract 1: all common rungs appear in output.
	for _, rung := range canonicalRungs {
		if _, ok := rungPos[rung]; !ok {
			t.Errorf("rung %q header not found in output:\n%s", rung, out)
		}
	}

	// Spec 132 contract 2: rungs appear in canonical order.
	for i := 1; i < len(canonicalRungs); i++ {
		prev, curr := canonicalRungs[i-1], canonicalRungs[i]
		prevPos, prevOK := rungPos[prev]
		currPos, currOK := rungPos[curr]
		if !prevOK || !currOK {
			continue // missing-rung already reported above
		}
		if prevPos >= currPos {
			t.Errorf("rung %q (pos %d) must appear before rung %q (pos %d) in output:\n%s",
				prev, prevPos, curr, currPos, out)
		}
	}
}
