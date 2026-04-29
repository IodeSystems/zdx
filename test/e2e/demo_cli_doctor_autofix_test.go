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

// TestDemoCLI_DoctorAutoFix is the demo for spec 128:
// Given a failing check that defines a fix function, when dx doctor runs with
// --fix, the fix is applied automatically (→ fixed); when --fix is not passed,
// the finding is reported with an 'auto-fixable' hint instead of mutating the
// project.
//
// The demo uses the dev_container_defined check as the representative
// auto-fixable finding: a project without dev.Dockerfile /
// docker-compose.dev.yml triggers a FixFunc that scaffolds both files. A
// hermetic temp dir with .zdx/credentials ensures the scaffold rungs pass
// locally while the unreachable server keeps all remote-derived state empty.
func TestDemoCLI_DoctorAutoFix(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_autofix_test.go", Note: "demo source (spec 128)"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 141, LineEnd: 151, Note: "auto-fix dispatch: FixFunc != nil → apply or hint"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 359, LineEnd: 376, Note: "dev_container_defined check with FixFunc that scaffolds dev.Dockerfile + docker-compose.dev.yml"},
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

	// Scaffold .zdx/config.yaml + .zdx/credentials (unreachable server) so that
	// all local scaffold checks pass while remote_reachable fails silently
	// (ActionInfo, no proposal arrow). This isolates the dev_container_defined
	// check as the representative auto-fixable finding.
	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	configYAML := "remote:\n  url: \"http://localhost:19999\"\n  slug: \"demo-autofix\"\nhooks: {}\ncomponents: {}\n"
	if err := os.WriteFile(filepath.Join(zdxDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zdxDir, "credentials"), []byte("fake-api-key\n"), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	runDx := func(args ...string) (stdout, stderr string, exitCode int, dur int64) {
		cmd := exec.Command(dxBin, args...)
		cmd.Dir = tmp
		cmd.Stdin = strings.NewReader("tool\n")
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		start := time.Now()
		runErr := cmd.Run()
		dur = time.Since(start).Milliseconds()
		exitCode = 0
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
			}
		}
		return outBuf.String(), errBuf.String(), exitCode, dur
	}

	// Step 1: without --fix — expect hint, no files created.
	stdout1, stderr1, exitCode1, dur1 := runDx("doctor")
	if !strings.Contains(stdout1, "auto-fixable (run with --fix)") {
		t.Errorf("expected 'auto-fixable (run with --fix)' hint without --fix:\n%s", stdout1)
	}
	if _, statErr := os.Stat(filepath.Join(tmp, "dev.Dockerfile")); statErr == nil {
		t.Error("dev.Dockerfile must not exist before --fix is passed")
	}

	// Step 2: with --fix — expect fix applied, files created.
	stdout2, stderr2, exitCode2, dur2 := runDx("doctor", "--fix")
	if !strings.Contains(stdout2, "→ fixed") {
		t.Errorf("expected '→ fixed' with --fix:\n%s", stdout2)
	}
	if _, statErr := os.Stat(filepath.Join(tmp, "dev.Dockerfile")); statErr != nil {
		t.Errorf("dev.Dockerfile should exist after --fix: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(tmp, "docker-compose.dev.yml")); statErr != nil {
		t.Errorf("docker-compose.dev.yml should exist after --fix: %v", statErr)
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
			Steps: []demoStep{
				{
					Cmd:        "dx doctor",
					Args:       []string{"dx", "doctor"},
					Stdout:     stdout1,
					Stderr:     stderr1,
					ExitCode:   exitCode1,
					DurationMs: dur1,
				},
				{
					Cmd:        "dx doctor --fix",
					Args:       []string{"dx", "doctor", "--fix"},
					Stdout:     stdout2,
					Stderr:     stderr2,
					ExitCode:   exitCode2,
					DurationMs: dur2,
				},
			},
		}
		b, _ := json.MarshalIndent(log, "", "  ")
		path := filepath.Join(dir, t.Name()+".json")
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Logf("demo save: %v", err)
			return
		}
		t.Logf("CLI demo saved → %s", path)
	})
}
