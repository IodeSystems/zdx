package agent

import (
	"strings"
	"testing"
)

// enforceContainerExecution gates the host-process agent path behind
// DX_AGENT_FORCE_CONTAINER so operators (CI workers, prod-style agent images)
// can require container isolation. Local dev keeps the host path available
// for fast iteration.

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

func TestCollectContainerEnv_AllowlistDoesNotIncludeAdminToken(t *testing.T) {
	t.Setenv("DX_REMOTE_API_KEY", "admin-secret")
	pairs := collectContainerEnv([]string{"ANTHROPIC_API_KEY", "DATABASE_URL", "NO_COLOR"})
	for _, p := range pairs {
		if strings.HasPrefix(p, "DX_REMOTE_API_KEY=") {
			t.Errorf("DX_REMOTE_API_KEY must not appear in restricted allowlist output: %v", pairs)
		}
	}
}
