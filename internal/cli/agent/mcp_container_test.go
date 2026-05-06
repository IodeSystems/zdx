package agent

import (
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

// buildMCPSlotArgs is the security-critical surface for opencode/local
// container mode. The slot must be sandboxed identically to the
// claude-in-container slot (non-root, no privilege escalation, capability
// drops, resource limits), and the entrypoint must be `sleep infinity` so
// `docker exec dx-agent --mcp-stdio` can spawn MCP servers across many
// sessions instead of the container exiting after the first.
func TestBuildMCPSlotArgs_SecurityFlags(t *testing.T) {
	args := buildMCPSlotArgs(
		"slot-x", "img", "/proj", "/proj/.zdx/agent/slots/x",
		config.AgentConfig{ContainerMemory: "4g", ContainerCPUs: "2"},
		false, nil)

	joined := strings.Join(args, " ")
	for _, must := range []string{
		"run -d", "--name slot-x",
		"--rm",
		"-v /proj/.zdx/agent/slots/x:/workspace", "-w /workspace",
		// .git/ bind-mount lets the worktree's gitdir pointer (host abs path) resolve in-slot.
		"-v /proj/.git:/proj/.git",
		"--user agent",
		"--cap-drop all",
		"--security-opt no-new-privileges",
		"--memory 4g",
		"--cpus 2",
		"img sleep infinity",
	} {
		if !strings.Contains(joined, must) {
			t.Errorf("docker run argv missing %q: %v", must, args)
		}
	}
}

func TestBuildMCPSlotArgs_KeepOnExitOmitsRm(t *testing.T) {
	args := buildMCPSlotArgs("n", "img", "/c", "/c/wt", config.AgentConfig{}, true, nil)
	for _, a := range args {
		if a == "--rm" {
			t.Errorf("--rm must be omitted when keepOnExit is true: %v", args)
		}
	}
}

func TestBuildMCPSlotArgs_DetachedAndSleepEntrypoint(t *testing.T) {
	args := buildMCPSlotArgs("n", "img", "/c", "/c/wt", config.AgentConfig{}, false, nil)
	if len(args) < 2 || args[0] != "run" || args[1] != "-d" {
		t.Errorf("argv must start with `run -d` (detached); got: %v", args)
	}
	// Image tag must precede the entrypoint (`sleep infinity`); subsequent
	// argv runs inside the container, not on the host.
	imgIdx := -1
	for i, a := range args {
		if a == "img" {
			imgIdx = i
			break
		}
	}
	if imgIdx < 0 || imgIdx+2 >= len(args) {
		t.Fatalf("expected image tag followed by entrypoint: %v", args)
	}
	if got := strings.Join(args[imgIdx+1:imgIdx+3], " "); got != "sleep infinity" {
		t.Errorf("slot entrypoint must be `sleep infinity`; got: %s", got)
	}
}

func TestBuildMCPSlotArgs_EnvPairsForwarded(t *testing.T) {
	args := buildMCPSlotArgs("n", "img", "/c", "/c/wt", config.AgentConfig{}, false, []string{"FOO=1", "BAR=baz"})
	pairs := []string{}
	for i, a := range args {
		if a == "-e" && i+1 < len(args) {
			pairs = append(pairs, args[i+1])
		}
	}
	want := map[string]bool{"FOO=1": false, "BAR=baz": false}
	for _, p := range pairs {
		if _, ok := want[p]; ok {
			want[p] = true
		}
	}
	for k, v := range want {
		if !v {
			t.Errorf("env pair %q not forwarded: pairs=%v", k, pairs)
		}
	}
}

func TestBuildMCPSlotArgs_NoResourceLimitsWhenUnset(t *testing.T) {
	args := buildMCPSlotArgs("n", "img", "/c", "/c/wt", config.AgentConfig{}, false, nil)
	for _, banned := range []string{"--memory", "--cpus"} {
		for _, a := range args {
			if a == banned {
				t.Errorf("%s must be omitted when AgentConfig leaves it unset: %v", banned, args)
			}
		}
	}
}
