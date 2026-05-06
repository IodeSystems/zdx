package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_AgentForceContainerBlocksHost is the demo for spec 117: each
// agent session runs inside a Docker container; no direct execution on the
// host process.
//
// The enforcement mechanism is the DX_AGENT_FORCE_CONTAINER environment
// variable. When it is set, host execution (--container=local) exits
// immediately with a spec-117 error. When it is unset (the dev default),
// host execution is allowed.
//
// History: --container started as a bool flag on `dx agent loop` only; later
// became a string flag with values docker|local on the parent (default
// docker). The test now exercises `dx agent --provider=claude
// --container=local` as the host path, since that's the only way to opt out
// of the new docker default — and that's what spec-117 needs to refuse when
// DX_AGENT_FORCE_CONTAINER=1.
//
// The demo exercises both branches end-to-end through the compiled dx binary:
//
//   - gate unset: --container=local is allowed; the command reaches past
//     enforceContainerExecution and fails later (no remote config or claude
//     binary), but NOT with a spec-117 error.
//   - gate set: --container=local exits immediately; stderr contains
//     "spec-117" and "container" and "DX_AGENT_FORCE_CONTAINER".
func TestDemoCLI_AgentForceContainerBlocksHost(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_agent_force_container_test.go", Note: "agent force-container demo source"},
		{FilePath: "internal/cli/agent/container.go", LineStart: 108, LineEnd: 120, Note: "enforceContainerExecution: gate DX_AGENT_FORCE_CONTAINER blocks host path"},
		{FilePath: "internal/cli/agent/claude.go", LineStart: 82, LineEnd: 91, Note: "call site: enforceContainerExecution checked before host-path claude spawn"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}
	dxBin := filepath.Join(root, "bin/dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Fatalf("dx binary not found at %q — run: make build", dxBin)
	}

	// Minimal project dir: llm_provider=local so claude-installed check is skipped.
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".zdx"), 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".zdx", "config.yaml"),
		[]byte("agent:\n  llm_provider: local\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	run := func(extraEnv []string) (stderr string, exitCode int) {
		t.Helper()
		cmd := exec.Command(dxBin, "agent", "--provider=claude", "--container=local")
		cmd.Dir = projectDir
		env := os.Environ()
		env = append(env, extraEnv...)
		cmd.Env = env
		var errBuf bytes.Buffer
		cmd.Stderr = &errBuf
		err := cmd.Run()
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		return errBuf.String(), exitCode
	}

	// ── Gate unset: host path is allowed through enforceContainerExecution ────
	gateUnsetStderr, gateUnsetCode := run([]string{"DX_AGENT_FORCE_CONTAINER="})
	if strings.Contains(gateUnsetStderr, "spec-117") {
		t.Errorf("gate unset: must not produce spec-117 error, got stderr: %q", gateUnsetStderr)
	}
	// Non-zero is expected (no remote, no claude binary), just not spec-117.
	_ = gateUnsetCode

	// ── Gate set: host path is blocked immediately with a spec-117 error ──────
	gateSetStderr, gateSetCode := run([]string{"DX_AGENT_FORCE_CONTAINER=1"})
	if gateSetCode == 0 {
		t.Errorf("gate set: expected non-zero exit when --container omitted, got 0")
	}
	for _, want := range []string{"spec-117", "container", "DX_AGENT_FORCE_CONTAINER"} {
		if !strings.Contains(gateSetStderr, want) {
			t.Errorf("gate set: error missing %q in stderr: %q", want, gateSetStderr)
		}
	}
}
