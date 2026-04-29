package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_AgentLocalModeNoDocker is the demo for spec 126: the agent harness
// supports a local mode for machines without a Docker daemon.
//
// For non-isolated project classifications (library, tool, site), dx agent start
// emits a warning to stderr and continues — no Docker required. For isolated
// classifications (service, saas), the command fails immediately with a clear
// docker-specific error message so the developer knows what to fix.
//
// The demo creates a minimal project dir with llm_provider=local (to bypass the
// claude check) and a fake docker that always fails (to simulate no daemon). It
// then invokes dx agent start against a library project and a service project:
//
//   - library: warning on stderr mentioning "docker"; command proceeds past the
//     docker gate (fails later for an unrelated reason — no git repo)
//   - service: immediate failure; error mentions "docker" + "service"
func TestDemoCLI_AgentLocalModeNoDocker(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_agent_no_docker_test.go", Note: "agent local-mode no-docker demo source"},
		{FilePath: "internal/cli/agent/agent.go", LineStart: 205, LineEnd: 216, Note: "checkDockerRequirement: warn (non-isolated) vs error (isolated) when docker unavailable"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 134, LineEnd: 137, Note: "DetectLocal: docker info determines DockerAvailable"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}
	dxBin := filepath.Join(root, "bin/dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Fatalf("dx binary not found at %q — run: make build", dxBin)
	}

	// Fake docker: exits 1 unconditionally so `docker info` reports no daemon.
	fakeDockerDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeDockerDir, "docker"), []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	// Project dir: llm_provider=local bypasses the claude-CLI-installed check.
	projectDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(projectDir, ".zdx"), 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, ".zdx", "config.yaml"),
		[]byte("agent:\n  llm_provider: local\n"), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Build env: override PATH so fake docker shadows the real one.
	fakePath := fakeDockerDir + ":" + os.Getenv("PATH")
	baseEnv := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PATH=") {
			baseEnv = append(baseEnv, e)
		}
	}
	baseEnv = append(baseEnv, "PATH="+fakePath)

	agentStart := func(slug string) (stdout, stderr string, code int) {
		t.Helper()
		cmd := exec.Command(dxBin, "agent", "start")
		cmd.Dir = projectDir
		cmd.Env = append(baseEnv,
			"DX_REMOTE_URL="+srv.URL,
			"DX_REMOTE_API_KEY="+srv.AdminToken,
			"DX_REMOTE_SLUG="+slug,
		)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
		return outBuf.String(), errBuf.String(), code
	}

	// ── Library (non-isolated): warning, then proceeds past the docker gate ───
	apiDo(t, "POST", "/api/project",
		map[string]string{
			"slug":           "demo-agent-no-docker-lib",
			"name":           "Demo Agent No Docker Lib",
			"classification": "library",
		}, nil)

	_, libStderr, _ := agentStart("demo-agent-no-docker-lib")

	if !strings.Contains(libStderr, "warning") || !strings.Contains(libStderr, "docker") {
		t.Errorf("library: expected docker warning on stderr, got: %q", libStderr)
	}
	// Must NOT fail with the docker-gate error — that would mean the gate blocked a non-isolated project.
	if strings.Contains(libStderr, "docker daemon is not running") {
		t.Errorf("library: should not get docker-required error, got: %q", libStderr)
	}

	// ── Service (isolated): fails immediately with docker-specific error ───────
	apiDo(t, "POST", "/api/project",
		map[string]string{
			"slug":           "demo-agent-no-docker-svc",
			"name":           "Demo Agent No Docker Svc",
			"classification": "service",
		}, nil)

	_, svcStderr, svcCode := agentStart("demo-agent-no-docker-svc")

	if svcCode == 0 {
		t.Errorf("service: expected non-zero exit when docker unavailable, got 0")
	}
	if !strings.Contains(svcStderr, "docker") {
		t.Errorf("service: error should mention docker, got: %q", svcStderr)
	}
	if !strings.Contains(svcStderr, "service") {
		t.Errorf("service: error should mention classification, got: %q", svcStderr)
	}
}
