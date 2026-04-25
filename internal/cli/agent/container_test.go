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
