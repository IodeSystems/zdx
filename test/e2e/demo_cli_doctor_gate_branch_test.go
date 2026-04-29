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

// TestDemoCLI_DoctorGateBranchSuggestion is the demo for spec 166:
// Given a project shipping to production from the dev branch directly,
// when doctor runs, then it suggests introducing a tested gate branch
// between dev and production for safer deploys.
//
// Setup: a temp dir with a git repo (no staging/gate/release branches),
// a bin/ship file, and a .zdx config pointing to an unreachable server.
// Answering "service" to the classification prompt activates the operations
// rung which contains has_deploy_config. detectShipsFromDevDirectly sees
// bin/ship + no gate branch and returns true, triggering the proposal.
func TestDemoCLI_DoctorGateBranchSuggestion(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_gate_branch_test.go", Note: "demo source (spec 166)"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 168, LineEnd: 186, Note: "detectShipsFromDevDirectly: bin/ship + no gate branch → direct deploy detected"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 455, LineEnd: 463, Note: "has_deploy_config: flags direct-to-prod deploys and proposes gate branch"},
		{FilePath: "internal/doctor/vines.go", LineStart: 169, LineEnd: 181, Note: "serviceRungs: has_deploy_config in operations rung"},
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

	// Init a git repo with no staging/gate branch so detectShipsFromDevDirectly returns true.
	if out, err := exec.Command("git", "-C", tmp, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}

	// Create bin/ship to signal that this project ships to production.
	binDir := filepath.Join(tmp, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "ship"), []byte("#!/bin/bash\necho ship\n"), 0755); err != nil {
		t.Fatalf("write bin/ship: %v", err)
	}

	// .zdx scaffold: config pointing to unreachable server + fake credentials.
	// All local scaffold checks pass; remote is unreachable so classification
	// falls through to the prompt.
	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	configYAML := "remote:\n  url: \"http://localhost:19999\"\n  slug: \"demo-gate-branch\"\nhooks: {}\ncomponents: {}\n"
	if err := os.WriteFile(filepath.Join(zdxDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zdxDir, "credentials"), []byte("fake-api-key\n"), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	cmd := exec.Command(dxBin, "doctor")
	cmd.Dir = tmp
	// "service\n" selects the service classification, activating the operations
	// rung which includes has_deploy_config.
	cmd.Stdin = strings.NewReader("service\n")

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

	// Spec 166 contract 1: the failure message identifies direct-to-prod deploys.
	if !strings.Contains(out, "dev/main directly to production") {
		t.Errorf("expected 'dev/main directly to production' in output:\n%s", out)
	}

	// Spec 166 contract 2: the proposal recommends a gate branch.
	if !strings.Contains(out, "gate branch") {
		t.Errorf("expected 'gate branch' in proposal:\n%s", out)
	}

	// Spec 166 contract 3: the proposal names staging as a concrete next step.
	if !strings.Contains(out, "staging") {
		t.Errorf("expected 'staging' in proposal:\n%s", out)
	}
}
