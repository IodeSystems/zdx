package agent

import (
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

func TestBuildContainerArgs_SecurityFlags(t *testing.T) {
	args := buildContainerArgs(
		"zdx-agent-test-0",
		"zdx-agent:abc",
		"/host/proj",
		0,
		"test",
		config.AgentConfig{ContainerMemory: "4g", ContainerCPUs: "2", ClaudeModel: "claude-sonnet-4-6"},
		false,
		nil,
	)

	joined := strings.Join(args, " ")
	for _, want := range []string{
		"--user agent",
		"--cap-drop all",
		"--security-opt no-new-privileges",
		"--memory 4g",
		"--cpus 2",
		"--rm",
		"-v /host/proj:/workspace",
		"-w /workspace",
		"--model claude-sonnet-4-6",
		"--chrome=false",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in args: %s", want, joined)
		}
	}
}

func TestBuildContainerArgs_KeepOnExitOmitsRm(t *testing.T) {
	args := buildContainerArgs("n", "img", "/c", 0, "a", config.AgentConfig{}, true, nil)
	for _, a := range args {
		if a == "--rm" {
			t.Fatalf("--rm should be omitted when keepOnExit=true: %v", args)
		}
	}
}

func TestBuildContainerArgs_NoResourceLimitsWhenUnset(t *testing.T) {
	args := buildContainerArgs("n", "img", "/c", 0, "a", config.AgentConfig{}, false, nil)
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "--memory") {
		t.Errorf("--memory must be absent when ContainerMemory=\"\": %s", joined)
	}
	if strings.Contains(joined, "--cpus") {
		t.Errorf("--cpus must be absent when ContainerCPUs=\"\": %s", joined)
	}
}

func TestBuildContainerArgs_EnvPairsForwarded(t *testing.T) {
	args := buildContainerArgs("n", "img", "/c", 0, "a", config.AgentConfig{}, false, []string{"FOO=1", "BAR=baz"})
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-e FOO=1") || !strings.Contains(joined, "-e BAR=baz") {
		t.Errorf("env pairs not forwarded: %s", joined)
	}
}

// Spec 117: agent sessions must run inside a Docker container; no direct
// execution on the host. enforceContainerExecution gates the host path behind
// DX_AGENT_FORCE_CONTAINER so operators can require container isolation.
func TestEnforceContainerExecution_GateUnsetAllowsHost(t *testing.T) {
	t.Setenv("DX_AGENT_FORCE_CONTAINER", "")
	if err := enforceContainerExecution(false); err != nil {
		t.Fatalf("gate unset + no --container should not error: %v", err)
	}
}

func TestEnforceContainerExecution_GateSetBlocksHost(t *testing.T) {
	t.Setenv("DX_AGENT_FORCE_CONTAINER", "1")
	err := enforceContainerExecution(false)
	if err == nil {
		t.Fatal("expected error when DX_AGENT_FORCE_CONTAINER is set and --container is omitted")
	}
	for _, want := range []string{"spec-117", "container", "DX_AGENT_FORCE_CONTAINER"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestEnforceContainerExecution_GateSetAllowsContainer(t *testing.T) {
	t.Setenv("DX_AGENT_FORCE_CONTAINER", "1")
	if err := enforceContainerExecution(true); err != nil {
		t.Fatalf("gate set + --container should not error: %v", err)
	}
}

func TestBuildContainerArgs_ScopedToken(t *testing.T) {
	envPairs := []string{"DX_REMOTE_API_KEY=scoped-xyz", "ANTHROPIC_API_KEY=ak"}
	args := buildContainerArgs("n", "img", "/c", 0, "a", config.AgentConfig{}, false, envPairs)
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-e DX_REMOTE_API_KEY=scoped-xyz") {
		t.Errorf("scoped token not present: %s", joined)
	}
	count := strings.Count(joined, "DX_REMOTE_API_KEY=")
	if count != 1 {
		t.Errorf("expected exactly 1 DX_REMOTE_API_KEY= entry, got %d: %s", count, joined)
	}
}

func TestCollectContainerEnv_AllowlistDoesNotIncludeAdminToken(t *testing.T) {
	t.Setenv("DX_REMOTE_API_KEY", "admin-secret")
	pairs := collectContainerEnv([]string{"ANTHROPIC_API_KEY", "DATABASE_URL", "NO_COLOR"})
	for _, p := range pairs {
		if strings.HasPrefix(p, "DX_REMOTE_API_KEY=") {
			t.Errorf("DX_REMOTE_API_KEY must not appear in restricted allowlist output: %v", pairs)
		}
	}
}

// buildContainerArgs is the only way the agent harness reaches docker — its
// argv must always begin with `run` so a session is dispatched through
// `docker run`. Inside the container the entrypoint is `./bin/dx agent
// loop --provider=claude`, which keeps the claude binary execution scoped
// to the container's filesystem (not the host's PATH).
func TestBuildContainerArgs_DispatchesViaDockerRun(t *testing.T) {
	args := buildContainerArgs("n", "img", "/c", 0, "a", config.AgentConfig{}, false, nil)
	if len(args) == 0 || args[0] != "run" {
		t.Fatalf("docker dispatch must start with `run`, got: %v", args)
	}
	// The image tag must precede the entrypoint so subsequent argv runs
	// inside the container, not on the host.
	imgIdx := -1
	for i, a := range args {
		if a == "img" {
			imgIdx = i
			break
		}
	}
	if imgIdx < 0 || imgIdx+4 >= len(args) {
		t.Fatalf("expected image tag followed by container entrypoint: %v", args)
	}
	entry := strings.Join(args[imgIdx+1:imgIdx+5], " ")
	if !strings.HasPrefix(entry, "./bin/dx agent loop --provider=claude") {
		t.Errorf("container entrypoint must be `./bin/dx agent loop --provider=claude` (runs inside container), got: %s", entry)
	}
}
