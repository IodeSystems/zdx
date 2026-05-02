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

// TestDemoCLI_DoctorDeployStrategy is the demo for IS-927:
// the has_deploy_config rung consults the project-declared deploy.strategy
// in .zdx/config.yaml. trunk-deploy projects (zdx-go, iodesystems) pass the
// rung; gate-branch projects keep the original "ships from dev directly"
// failure when no staging branch exists.
//
// Setup mirrors demo_cli_doctor_gate_branch_test.go: a temp service project
// with bin/ship and only main+dev branches. The same project shape is run
// through dx doctor with three different deploy.strategy values to prove
// the same project changes its rung outcome based on the declared strategy.
func TestDemoCLI_DoctorDeployStrategy(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_deploy_strategy_test.go", Note: "demo source (IS-927)"},
		{FilePath: "internal/config/config.go", Note: "DeployConfig + ResolvedDeploy"},
		{FilePath: "internal/doctor/doctor.go", Note: "has_deploy_config switches on state.DeployStrategy"},
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

	// Init a git repo with no staging/gate branch so detectShipsFromDevDirectly
	// returns true. The deploy.strategy declaration is what flips the outcome.
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

	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zdxDir, "credentials"), []byte("fake-api-key\n"), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	configPath := filepath.Join(zdxDir, "config.yaml")
	configFor := func(strategy string) string {
		return "remote:\n  url: \"http://localhost:19999\"\n  slug: \"demo-deploy-strategy\"\n" +
			"hooks: {}\ncomponents: {}\n" +
			"deploy:\n  strategy: " + strategy + "\n"
	}

	// dx doctor prints failing rows with the message attached ("✗  Deploy
	// pipeline configured  — <message>") but passing rows only show the
	// description ("✓  Deploy pipeline configured"). Assertions look at the
	// visible row + absence/presence of the gate-branch proposal.
	type step struct {
		strategy   string
		assertions func(t *testing.T, out string)
	}
	steps := []step{
		{
			strategy: "trunk",
			assertions: func(t *testing.T, out string) {
				if !strings.Contains(out, "✓  Deploy pipeline configured") {
					t.Errorf("[trunk] expected passing 'Deploy pipeline configured' row:\n%s", out)
				}
				// Trunk should NOT trigger the gate-branch proposal.
				if strings.Contains(out, "dev/main directly to production") {
					t.Errorf("[trunk] should not flag direct-to-prod when strategy=trunk:\n%s", out)
				}
			},
		},
		{
			strategy: "gate-branch",
			assertions: func(t *testing.T, out string) {
				if !strings.Contains(out, "✗  Deploy pipeline configured") {
					t.Errorf("[gate-branch] expected failing 'Deploy pipeline configured' row:\n%s", out)
				}
				if !strings.Contains(out, "dev/main directly to production") {
					t.Errorf("[gate-branch] expected 'dev/main directly to production' in output:\n%s", out)
				}
				if !strings.Contains(out, "gate branch") {
					t.Errorf("[gate-branch] expected 'gate branch' in proposal:\n%s", out)
				}
				if !strings.Contains(out, "staging") {
					t.Errorf("[gate-branch] expected 'staging' in proposal:\n%s", out)
				}
			},
		},
		{
			strategy: "manual",
			assertions: func(t *testing.T, out string) {
				if !strings.Contains(out, "✓  Deploy pipeline configured") {
					t.Errorf("[manual] expected passing 'Deploy pipeline configured' row:\n%s", out)
				}
				if strings.Contains(out, "dev/main directly to production") {
					t.Errorf("[manual] should not flag direct-to-prod when strategy=manual:\n%s", out)
				}
			},
		},
	}

	demoSteps := make([]demoStep, 0, len(steps))

	for _, s := range steps {
		t.Run("strategy="+s.strategy, func(t *testing.T) {
			if err := os.WriteFile(configPath, []byte(configFor(s.strategy)), 0644); err != nil {
				t.Fatalf("write config.yaml: %v", err)
			}

			cmd := exec.Command(dxBin, "doctor", "--type", "service", "--non-interactive")
			cmd.Dir = tmp

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

			demoSteps = append(demoSteps, demoStep{
				Cmd:        "dx doctor (strategy=" + s.strategy + ")",
				Args:       []string{"dx", "doctor"},
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				ExitCode:   exitCode,
				DurationMs: dur,
			})

			s.assertions(t, stdout.String())
		})
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
			Steps:      demoSteps,
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
