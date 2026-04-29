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

// TestDemoCLI_DoctorDevContainerScaffold is the demo for spec 133:
// Given a project missing a dev container, when dx doctor --fix runs, then
// dev.Dockerfile and docker-compose.dev.yml are scaffolded based on detected
// stack (Go/Node/Python plus optional Postgres/Valkey/sqlc), and existing dev
// container files are never overwritten.
//
// Setup: a temp dir with go.mod (Go stack detected) and no dev container files.
// Step 1 verifies stack-aware content (golang: base image). Step 2 runs --fix
// again and verifies the previously scaffolded file is not overwritten.
func TestDemoCLI_DoctorDevContainerScaffold(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_dev_container_scaffold_test.go", Note: "demo source (spec 133)"},
		{FilePath: "internal/doctor/stack.go", LineStart: 24, LineEnd: 44, Note: "DetectStack: reads go.mod/package.json/pyproject.toml to determine toolchain"},
		{FilePath: "internal/doctor/dockerfile.go", LineStart: 10, LineEnd: 57, Note: "RenderDevDockerfile: emits stack-specific base image (golang:/node:/python:)"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 359, LineEnd: 376, Note: "dev_container_defined fix: writes only missing files — existing files never touched"},
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

	// go.mod signals a Go project so DetectStack returns HasGo=true.
	goMod := "module demo-scaffold\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	configYAML := "remote:\n  url: \"http://localhost:19999\"\n  slug: \"demo-scaffold\"\nhooks: {}\ncomponents: {}\n"
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

	// Step 1: --fix on a Go project with no container files.
	// Expect golang: base image in dev.Dockerfile (stack detection).
	stdout1, stderr1, exitCode1, dur1 := runDx("doctor", "--fix")
	if !strings.Contains(stdout1, "→ fixed") {
		t.Errorf("expected '→ fixed' after --fix:\n%s", stdout1)
	}
	dockerfilePath := filepath.Join(tmp, "dev.Dockerfile")
	composePath := filepath.Join(tmp, "docker-compose.dev.yml")
	dfContent, dfErr := os.ReadFile(dockerfilePath)
	if dfErr != nil {
		t.Fatalf("dev.Dockerfile not created after --fix: %v", dfErr)
	}
	if !strings.Contains(string(dfContent), "golang:") {
		t.Errorf("expected 'golang:' base image in dev.Dockerfile for Go project:\n%s", dfContent)
	}
	if _, err := os.Stat(composePath); err != nil {
		t.Errorf("docker-compose.dev.yml not created after --fix: %v", err)
	}

	// Step 2: --fix again — both files now exist, no overwrite should occur.
	// Sentinel content verifies the original bytes are preserved.
	sentinel := "# sentinel-do-not-overwrite\n"
	if err := os.WriteFile(dockerfilePath, []byte(sentinel), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	stdout2, stderr2, exitCode2, dur2 := runDx("doctor", "--fix")
	after, _ := os.ReadFile(dockerfilePath)
	if string(after) != sentinel {
		t.Errorf("dev.Dockerfile was overwritten on second --fix run:\nbefore: %q\nafter: %q\ndoctor output:\n%s", sentinel, string(after), stdout2)
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
					Cmd:        "dx doctor --fix  # Go project, no container files",
					Args:       []string{"dx", "doctor", "--fix"},
					Stdout:     stdout1,
					Stderr:     stderr1,
					ExitCode:   exitCode1,
					DurationMs: dur1,
				},
				{
					Cmd:        "dx doctor --fix  # second run — files already exist",
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
