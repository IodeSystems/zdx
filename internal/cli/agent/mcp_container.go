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
// Slots run `sleep infinity` so they stay alive while the host runs the LLM
// loop; each session does `docker exec -i <name> dx-agent --mcp-stdio` to
// spawn the MCP server inside. The slot is the long-lived sandbox, MCP
// servers are per-session.
//
// Security profile: non-root, no privilege escalation, all capabilities
// dropped, configurable memory/cpu limits.
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

// runMCPContainerLoop is --container's universal implementation across
// every provider that supports MCP-based tool dispatch (today: claude via
// --mcp-config, opencode, local). Slot containers stay idle as sandboxes;
// the LLM loop runs on the host; tool calls cross the boundary via
// `docker exec` per session.
//
// Lifecycle: build dev image, spawn N detached slot containers running
// `sleep infinity`, run N RunManagedLoop instances on the host (each with
// MCPCommand pointed at one slot), wait for shutdown signal, stop slots.
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

	maxSlots := agentCfg.MaxWorktrees
	if maxSlots <= 0 {
		maxSlots = 1
	}

	// Orchestrator-scope tracelog: alias=<base>, scope=container-orchestrator,
	// cluster_id=<base>. Per-slot loops will emit alias=<base>-<N> with the
	// same cluster_id so the whole cluster is filterable as one chain in
	// the UI. Best-effort — failure to set up the logger doesn't block the
	// orchestrator (logf prints to stdout regardless).
	clusterID := opts.Alias
	loopLog, loopSink := setupLoopTracelog(opts.RC, providerName, opts.Alias)
	if loopLog != nil {
		loopLog = loopLog.With(map[string]string{
			"scope":      "container-orchestrator",
			"cluster_id": clusterID,
		})
		defer loopLog.Close()
		if u := loopLog.FilteredURL(opts.RC.url, opts.RC.slug); u != "" {
			fmt.Printf("View orchestrator events: %s\n", u)
		}
	}
	if loopSink != nil {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = loopSink.Close(closeCtx)
		}()
	}
	emit := func(name string, kv ...any) {
		if loopLog != nil {
			loopLog.Info(name, kv...)
		}
	}
	logf := func(format string, args ...any) {
		fmt.Printf("[%s] "+format+"\n",
			append([]any{time.Now().Format(time.RFC3339)}, args...)...)
	}

	emit("image.building")
	imageStart := time.Now()
	imageTag, err := buildDevImage()
	if err != nil {
		emit("image.build_failed", "err", err.Error())
		return err
	}
	emit("image.built", "tag", imageTag, "duration_ms", time.Since(imageStart).Milliseconds())

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	logf("mcp-container mode: provider=%s image=%s slots=%d memory=%s cpus=%s",
		providerName, imageTag, maxSlots, agentCfg.ContainerMemory, agentCfg.ContainerCPUs)
	emit("orchestrator.started",
		"provider", providerName,
		"image", imageTag,
		"slots", maxSlots,
		"memory", agentCfg.ContainerMemory,
		"cpus", agentCfg.ContainerCPUs)
	defer emit("orchestrator.exited")

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Signal handling: stop slots then exit.
	var slotMu sync.Mutex
	var slotNames []string
	stopSlot := func(idx int, name string) {
		emit("slot.stopping", "slot_index", idx, "name", name)
		out, err := exec.Command("docker", "stop", name).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "mcp-container: stop %s: %s\n", name, strings.TrimSpace(string(out)))
			emit("slot.stop_failed", "slot_index", idx, "name", name, "err", err.Error())
			return
		}
		emit("slot.stopped", "slot_index", idx, "name", name)
	}
	stopSlots := func() {
		slotMu.Lock()
		names := append([]string(nil), slotNames...)
		slotMu.Unlock()
		for i, n := range names {
			stopSlot(i, n)
		}
	}
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logf("received signal %s: stopping slots and draining loops", sig)
		emit("drain.received", "signal", sig.String())
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
		emit("slot.starting", "slot_index", i, "name", slotName)

		runArgs := buildMCPSlotArgs(slotName, imageTag, cwd, agentCfg, opts.KeepContainer, envPairs)
		startCmd := exec.CommandContext(ctx, "docker", runArgs...)
		startCmd.Stdout = os.Stderr // docker run -d prints the container ID
		startCmd.Stderr = os.Stderr
		if err := startCmd.Run(); err != nil {
			emit("slot.start_failed", "slot_index", i, "name", slotName, "err", err.Error())
			cancel()
			stopSlots()
			return fmt.Errorf("start slot %d: %w", i, err)
		}
		slotMu.Lock()
		slotNames = append(slotNames, slotName)
		slotMu.Unlock()
		logf("slot %d started: %s", i, slotName)
		emit("slot.started", "slot_index", i, "name", slotName)

		wg.Add(1)
		go func(slot int, slotName string) {
			defer wg.Done()
			slotOpts := opts
			slotOpts.Alias = fmt.Sprintf("%s-%d", opts.Alias, slot)
			slotOpts.ClusterID = clusterID
			// /workspace/bin/dx-agent — absolute path, not bare `dx-agent`. The
			// dev image (dev.Dockerfile) builds without /workspace/bin on $PATH,
			// so a bare command resolves to "executable not found in $PATH" and
			// claude's --mcp-config silently fails to wire up the dx-tools
			// server. With --tools "" disabling claude's built-ins, that left
			// the in-slot session with only the host's chrome/LSP MCP tools and
			// no Bash/Read/Edit — the agents kept reporting they couldn't act.
			slotOpts.MCPCommand = []string{"docker", "exec", "-i", slotName, "/workspace/bin/dx-agent", "--mcp-stdio"}
			if err := RunManagedLoop(ctx, providerName, slotOpts); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "slot %d (%s) loop error: %v\n", slot, slotName, err)
			}
		}(i, slotName)
	}
	wg.Wait()
	stopSlots()
	return nil
}
