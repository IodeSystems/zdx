package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// buildMCPSlotArgs constructs `docker run` argv for an idle slot container.
// Unlike buildContainerArgs (which dispatches `dx agent loop --provider=
// claude` inside), the MCP-slot container runs `sleep infinity` so it stays
// alive while the host runs the LLM loop. Each session does `docker exec
// -i <name> dx-agent --mcp-stdio` to spawn the MCP server inside; the slot
// container is the long-lived sandbox, MCP servers are per-session.
//
// Same security flags as buildContainerArgs (non-root, no privilege
// escalation, capability drops, resource limits) — the slot is just as
// sandboxed as the claude-in-container slot, and runs across many sessions.
func buildMCPSlotArgs(name, imageTag, cwd string, agentCfg config.AgentConfig, keepOnExit bool, envPairs []string) []string {
	args := []string{"run", "-d", "--name", name}
	if !keepOnExit {
		args = append(args, "--rm")
	}
	args = append(args, "-v", cwd+":/workspace", "-w", "/workspace")

	// Same security profile as the claude-in-container path.
	args = append(args, "--user", "agent")
	args = append(args, "--cap-drop", "all")
	args = append(args, "--security-opt", "no-new-privileges")

	if agentCfg.ContainerMemory != "" {
		args = append(args, "--memory", agentCfg.ContainerMemory)
	}
	if agentCfg.ContainerCPUs != "" {
		args = append(args, "--cpus", agentCfg.ContainerCPUs)
	}

	for _, kv := range envPairs {
		args = append(args, "-e", kv)
	}

	args = append(args, imageTag, "sleep", "infinity")
	return args
}

// runMCPContainerLoop is the opencode/local equivalent of runContainerLoop.
// Where runContainerLoop ships the entire agent inside each slot, this
// function ships only the MCP server inside the slot — the LLM loop runs
// on the host, tool calls cross the boundary via `docker exec` per session.
//
// Lifecycle: build dev image, spawn N detached slot containers running
// `sleep infinity`, run N RunManagedLoop instances on the host (each with
// MCPCommand pointed at one slot), wait for shutdown signal, stop slots.
//
// Provider must be a non-claude registered name. claude continues to use
// runContainerLoop because its CLI talks to its own tool surface, not MCP.
func runMCPContainerLoop(parentCtx context.Context, providerName string, opts ProviderOpts) error {
	if opts.RC.slug == "" {
		return fmt.Errorf("--container requires a project config with a remote slug")
	}
	agentCfg := opts.AgentCfg
	if agentCfg.ContainerMemory == "" {
		agentCfg.ContainerMemory = "4g"
	}
	if agentCfg.ContainerCPUs == "" {
		agentCfg.ContainerCPUs = "2"
	}

	imageTag, err := buildDevImage()
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	maxSlots := agentCfg.MaxWorktrees
	if maxSlots <= 0 {
		maxSlots = 1
	}

	logf := func(format string, args ...any) {
		fmt.Printf("[%s] "+format+"\n",
			append([]any{time.Now().Format(time.RFC3339)}, args...)...)
	}
	logf("mcp-container mode: provider=%s image=%s slots=%d memory=%s cpus=%s",
		providerName, imageTag, maxSlots, agentCfg.ContainerMemory, agentCfg.ContainerCPUs)

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Signal handling: stop slots then exit.
	var slotMu sync.Mutex
	var slotNames []string
	stopSlots := func() {
		slotMu.Lock()
		names := append([]string(nil), slotNames...)
		slotMu.Unlock()
		for _, n := range names {
			out, err := exec.Command("docker", "stop", n).CombinedOutput()
			if err != nil {
				fmt.Fprintf(os.Stderr, "mcp-container: stop %s: %s\n", n, strings.TrimSpace(string(out)))
			}
		}
	}
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logf("received signal %s: stopping slots and draining loops", sig)
		cancel()
		stopSlots()
		select {
		case <-time.After(10 * time.Second):
		case <-sigCh:
		}
		os.Exit(130)
	}()

	envPairs := collectContainerEnv([]string{"ANTHROPIC_API_KEY", "DATABASE_URL", "NO_COLOR"})

	var wg sync.WaitGroup
	for i := 0; i < maxSlots; i++ {
		slotName := fmt.Sprintf("zdx-agent-mcp-%s-%s-%d", providerName, opts.Alias, i)

		runArgs := buildMCPSlotArgs(slotName, imageTag, cwd, agentCfg, opts.KeepContainer, envPairs)
		startCmd := exec.CommandContext(ctx, "docker", runArgs...)
		startCmd.Stdout = os.Stderr // docker run -d prints the container ID
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
			cancel()
			stopSlots()
			return fmt.Errorf("start slot %d: %w", i, err)
		}
		slotMu.Lock()
		slotNames = append(slotNames, slotName)
		slotMu.Unlock()
		logf("slot %d started: %s", i, slotName)

		wg.Add(1)
		go func(slot int, slotName string) {
			defer wg.Done()
			slotOpts := opts
			slotOpts.Alias = fmt.Sprintf("%s-%d", opts.Alias, slot)
			slotOpts.MCPCommand = []string{"docker", "exec", "-i", slotName, "dx-agent", "--mcp-stdio"}
			if err := RunManagedLoop(ctx, providerName, slotOpts); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "slot %d (%s) loop error: %v\n", slot, slotName, err)
			}
		}(i, slotName)
	}
	wg.Wait()
	stopSlots()
	return nil
}
