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

// TestDemoCLI_DoctorVisionFirstGap is the demo for spec 115:
// Given a project with no vision, when dx doctor runs, it flags the missing
// vision as the first maturity gap — the first proposal (→) in the output —
// with an explanation that frames it as a guiding star for goals and features.
//
// Setup: a temp dir with a valid .zdx/config.yaml and .zdx/credentials pointing
// to an unreachable server. All scaffold checks pass locally; remote_reachable
// fails silently (ActionInfo, no proposal arrow), so has_vision produces the
// first → line.
func TestDemoCLI_DoctorVisionFirstGap(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_vision_test.go", Note: "demo source (spec 115)"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 263, LineEnd: 268, Note: "has_vision check: returns guiding-star proposal when vision missing"},
		{FilePath: "internal/doctor/vines.go", LineStart: 99, LineEnd: 99, Note: "has_vision is first check in identity rung"},
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

	// Scaffold: .zdx/ dir + config.yaml (unreachable URL) + credentials (fake key).
	// This makes all local scaffold checks pass while ensuring RemoteReachable=false
	// so HasVision stays false — the gap under test.
	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	configYAML := "remote:\n  url: \"http://localhost:19999\"\n  slug: \"demo-vision\"\nhooks: {}\ncomponents: {}\n"
	if err := os.WriteFile(filepath.Join(zdxDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zdxDir, "credentials"), []byte("fake-api-key\n"), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	cmd := exec.Command(dxBin, "doctor")
	cmd.Dir = tmp
	// "tool\n" answers the classification prompt (server unreachable → no remote classification).
	// Subsequent ReadString calls hit EOF and are treated as skip/no-op by the defer loop.
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

	// Spec 115 contract 1: the vision proposal explains why a guiding star matters.
	if !strings.Contains(out, "guiding star") {
		t.Errorf("expected 'guiding star' explanation in output:\n%s", out)
	}

	// Spec 115 contract 2: the proposal recommends `dx vision set`.
	if !strings.Contains(out, "dx vision set") {
		t.Errorf("expected 'dx vision set' proposal in output:\n%s", out)
	}

	// Spec 115 contract 3: vision is the first proposal (→) — the first maturity gap.
	// remote_reachable fails silently (ActionInfo, no →), so has_vision is first.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "      → ") {
			if !strings.Contains(line, "dx vision set") {
				t.Errorf("expected first proposal to be about vision, got:\n  %s\nfull output:\n%s", line, out)
			}
			break
		}
	}
}
