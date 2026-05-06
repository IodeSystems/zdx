package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
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

// slotWorktree provisions a per-slot git worktree under
// .zdx/agent/slots/<alias>-<i>/ on a fresh branch wip/<alias>-<i> rooted
// at the operator's current HEAD. Each slot's container then bind-mounts
// its OWN worktree (not the shared project root), so:
//
//   - operator can edit/commit in the host tree without racing the slots
//   - slots can edit/commit in isolation; their work lands on their own
//     branch
//   - if two slots touch the same files, conflict resolution is just
//     normal git merge between two branches, not last-writer-wins on
//     a shared working tree
//
// Branches are intentionally namespaced under wip/* (not agent/*) — the
// agent/* namespace is reserved for the planned per-issue lifecycle
// (branch=agent/IS-N, persistent across babysit lifetimes, with a review-
// as-agent verdict driving merge/comment/BQ outcomes). Using a separate
// wip/* namespace today keeps that door open: when the per-issue model
// lands it can claim agent/* without colliding with these transient slots.
//
// Returns the worktree path and the branch name. The caller is
// responsible for cleanup via cleanupSlotWorktree on exit.
//
// bin/dx-agent (the in-slot MCP server) is gitignored, so a fresh
// worktree won't have it. Copy it from the host's bin/ so docker exec
// /workspace/bin/dx-agent --mcp-stdio resolves inside the slot.
func slotWorktree(cwd, alias string, slotIdx int) (path, branch string, err error) {
	branch = fmt.Sprintf("wip/%s-%d", alias, slotIdx)
	path = filepath.Join(cwd, ".zdx", "agent", "slots", fmt.Sprintf("%s-%d", alias, slotIdx))

	// Idempotent: if the worktree already exists from a prior crashed run,
	// remove it before re-adding so `git worktree add -b` doesn't fail with
	// "branch already checked out".
	if _, statErr := os.Stat(path); statErr == nil {
		_ = exec.Command("git", "worktree", "remove", "--force", path).Run()
		_ = exec.Command("git", "branch", "-D", branch).Run()
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir worktree parent: %w", err)
	}
	out, addErr := exec.Command("git", "worktree", "add", "-b", branch, path).CombinedOutput()
	if addErr != nil {
		return "", "", fmt.Errorf("git worktree add %s: %s", branch, strings.TrimSpace(string(out)))
	}

	if err := copyDxAgentBinary(cwd, path); err != nil {
		// Roll back the worktree if we can't seed bin/dx-agent — without
		// it the slot's MCP dispatch will fail and the slot is unusable.
		_ = exec.Command("git", "worktree", "remove", "--force", path).Run()
		_ = exec.Command("git", "branch", "-D", branch).Run()
		return "", "", err
	}
	return path, branch, nil
}

// cleanupSlotWorktree removes a slot's worktree on shutdown. If the slot
// made any commits beyond the starting HEAD, the branch is preserved so
// the operator can `dx merge-train run` it. If no commits were made, the
// branch is deleted too (no detritus from dry runs).
func cleanupSlotWorktree(cwd, path, branch string) {
	if path == "" {
		return
	}
	// Check if branch advanced past its starting point. `git rev-list
	// HEAD..branch --count` from the worktree's branch vs. the parent tree's
	// dev tip. We compare against the cwd's current HEAD as the reference.
	hasCommits := false
	if out, err := exec.Command("git", "-C", cwd, "rev-list", "HEAD.."+branch, "--count").Output(); err == nil {
		if strings.TrimSpace(string(out)) != "0" {
			hasCommits = true
		}
	}
	if out, err := exec.Command("git", "worktree", "remove", "--force", path).CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "cleanup: git worktree remove %s: %s\n", path, strings.TrimSpace(string(out)))
	}
	if !hasCommits {
		// Empty branch — no value in keeping it.
		_ = exec.Command("git", "-C", cwd, "branch", "-D", branch).Run()
	} else {
		fmt.Printf("[slot] preserved branch %s (has commits — inspect with `git log %s` and integrate as you see fit)\n", branch, branch)
	}
}

// copyDxAgentBinary mirrors the operator's bin/dx-agent into the slot
// worktree's bin/ so the slot's MCP dispatch can locate it under
// /workspace/bin/dx-agent. bin/ is gitignored so a fresh worktree
// otherwise has no bin/.
func copyDxAgentBinary(srcRoot, dstRoot string) error {
	srcPath := filepath.Join(srcRoot, "bin", "dx-agent")
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open bin/dx-agent (run `make build` first): %w", err)
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat bin/dx-agent: %w", err)
	}

	dstDir := filepath.Join(dstRoot, "bin")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("mkdir slot bin/: %w", err)
	}
	dstPath := filepath.Join(dstDir, "dx-agent")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create slot bin/dx-agent: %w", err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy bin/dx-agent: %w", err)
	}
	return nil
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
	// Worktree cleanup hook installed by the slot-provisioning step below
	// (worktrees don't exist yet at this point, so we route through a func
	// pointer that's swapped in once they're created). os.Exit(130) bypasses
	// deferred functions, so the signal handler must invoke cleanup itself
	// before exiting — otherwise SIGTERM leaves orphan worktrees + branches
	// in .zdx/agent/slots/ that the next run has to forcibly remove.
	cleanupOnSignal := func() {}
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
		cleanupOnSignal()
		os.Exit(130)
	}()

	envPairs := collectContainerEnv([]string{"ANTHROPIC_API_KEY", "DATABASE_URL", "NO_COLOR"})

	// Per-slot worktrees: each slot gets its own .zdx/agent/slots/<alias>-<i>/
	// checkout on a fresh agent/<alias>-<i> branch. Without this, every slot
	// (and the operator) shared the same working tree and stomped each
	// other's edits silently.
	type slotPaths struct {
		path   string
		branch string
	}
	worktrees := make([]slotPaths, maxSlots)
	cleanupWorktrees := func() {
		for _, w := range worktrees {
			cleanupSlotWorktree(cwd, w.path, w.branch)
		}
	}
	for i := 0; i < maxSlots; i++ {
		path, branch, err := slotWorktree(cwd, opts.Alias, i)
		if err != nil {
			emit("slot.worktree_failed", "slot_index", i, "err", err.Error())
			// Roll back any worktrees created so far before bailing.
			for j := 0; j < i; j++ {
				cleanupSlotWorktree(cwd, worktrees[j].path, worktrees[j].branch)
			}
			return fmt.Errorf("slot %d worktree: %w", i, err)
		}
		worktrees[i] = slotPaths{path: path, branch: branch}
		emit("slot.worktree_created", "slot_index", i, "path", path, "branch", branch)
	}
	defer cleanupWorktrees()
	cleanupOnSignal = cleanupWorktrees

	var wg sync.WaitGroup
	for i := 0; i < maxSlots; i++ {
		slotName := fmt.Sprintf("zdx-agent-mcp-%s-%s-%d", providerName, opts.Alias, i)
		emit("slot.starting", "slot_index", i, "name", slotName)

		// Mount the slot's own worktree into the container, not the operator's
		// shared cwd. Each slot now has an isolated working tree on its own
		// branch — operator can edit/commit in cwd without racing slots.
		slotMount := worktrees[i].path
		runArgs := buildMCPSlotArgs(slotName, imageTag, slotMount, agentCfg, opts.KeepContainer, envPairs)
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
