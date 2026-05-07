package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// buildMCPSlotArgs constructs `docker run` argv for an idle slot container.
// Slots run `sleep infinity` so they stay alive while the host runs the LLM
// loop; each session does `docker exec -i <name> dx-agent --mcp-stdio` to
// spawn the MCP server inside. The slot is the long-lived sandbox, MCP
// servers are per-session.
//
// The slot mounts:
//   - the per-slot worktree at /workspace (isolation from the operator)
//   - the host project root's .git/ at the same absolute host path inside
//     the container (so the worktree's .git file, which contains an
//     absolute host path to .git/worktrees/<name>, resolves)
//
// Mounting .git/ at the same path means the slot can both write commits
// (they land in the parent repo's .git/ where the operator can fetch them)
// and read history. It does open a concurrent-write surface on .git/
// itself (multiple slots + operator can race on .git/index.lock), but git
// handles this with its built-in locking and retry.
//
// Security profile: non-root, no privilege escalation, all capabilities
// dropped, configurable memory/cpu limits.
func buildMCPSlotArgs(name, imageTag, projectRoot, worktreePath string, agentCfg config.AgentConfig, keepOnExit bool, envPairs []string) []string {
	args := []string{"run", "-d", "--name", name}
	if !keepOnExit {
		args = append(args, "--rm")
	}
	args = append(args, "-v", worktreePath+":/workspace", "-w", "/workspace")
	// Mount the parent repo's .git at the same absolute host path so
	// `gitdir: <host>/.git/worktrees/<name>` references inside the
	// worktree's .git file resolve. Without this, every git operation
	// in the slot (commit, status, log, branch, fetch) fails.
	if projectRoot != "" {
		hostGit := filepath.Join(projectRoot, ".git")
		args = append(args, "-v", hostGit+":"+hostGit)
	}

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
// ~/.zdx/projects/<slug>/slots/<alias>-<i>/ on a fresh branch
// wip/<alias>-<i> rooted at the operator's current HEAD. Each slot's
// container then bind-mounts its OWN worktree (not the shared project
// root), so:
//
//   - operator can edit/commit in the host tree without racing the slots
//   - slots can edit/commit in isolation; their work lands on their own
//     branch
//   - if two slots touch the same files, conflict resolution is just
//     normal git merge between two branches, not last-writer-wins on
//     a shared working tree
//
// **Workspace location is global** — the slot worktree lives under the
// operator's home directory, NOT inside the operator's project tree.
// This matches the srcless flow (~/.zdx/projects/<slug>/main +
// /worktrees/<sid>) so global-pool agents and project-scoped agents
// share a single workspace root. Pinning a global to a project is
// metadata, not a filesystem move. (GAPD: workspace relocation.)
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
func slotWorktree(cwd, slug, alias string, slotIdx int) (path, branch string, err error) {
	if slug == "" {
		return "", "", fmt.Errorf("slotWorktree: empty slug — workspace path is rooted at ~/.zdx/projects/<slug>/")
	}
	home, herr := os.UserHomeDir()
	if herr != nil {
		return "", "", fmt.Errorf("resolve home dir: %w", herr)
	}
	branch = fmt.Sprintf("wip/%s-%d", alias, slotIdx)
	path = filepath.Join(home, ".zdx", "projects", slug, "slots", fmt.Sprintf("%s-%d", alias, slotIdx))

	// Idempotent: if the worktree already exists from a prior crashed run,
	// remove it before re-adding so `git worktree add -b` doesn't fail with
	// "branch already checked out".
	//
	// Critical: do NOT blow away a branch that has commits past HEAD —
	// that's real work from the prior run (a crashed babysit or a
	// self-update re-exec mid-session). Lost commits showed up as
	// dangling objects and operators had to fsck-and-recover them by
	// hand. If commits are present, refuse to reuse the slot path; the
	// caller's choice is to surface the surviving branch and let the
	// operator integrate it before retrying.
	if _, statErr := os.Stat(path); statErr == nil {
		hasCommits := false
		if out, lsErr := exec.Command("git", "-C", cwd, "rev-list", "HEAD.."+branch, "--count").Output(); lsErr == nil {
			if strings.TrimSpace(string(out)) != "0" {
				hasCommits = true
			}
		}
		if hasCommits {
			return "", "", fmt.Errorf("slot path %s has branch %s with commits ahead of HEAD — integrate or delete the branch (`git log %s` then `git branch -D %s`) before re-running this alias", path, branch, branch, branch)
		}
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

	if err := copyDxBinaries(cwd, path); err != nil {
		// Roll back the worktree if we can't seed bin/{dx,dx-agent} —
		// without dx-agent MCP dispatch fails and the slot is unusable;
		// without dx the agent's tracker calls 401/exec-fail.
		_ = exec.Command("git", "worktree", "remove", "--force", path).Run()
		_ = exec.Command("git", "branch", "-D", branch).Run()
		return "", "", err
	}
	if err := seedSlotZdxFiles(cwd, path); err != nil {
		// Without credentials the in-slot dx CLI 401s on every tracker
		// call. Roll back so a half-set-up slot doesn't end up running
		// in an unrecoverable state.
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

// provisionedSlot is a fully-set-up slot ready for a session or loop:
// docker container running, worktree mounted at /workspace, MCPCmd ready
// to forward tool calls into the slot. The Cleanup func tears it all down
// (docker stop, worktree remove, branch -D if no commits).
//
// This is the executor primitive that makes "single session in container"
// and "loop in container" composable: both paths provision one or more
// slots via the same helper, then differ only in what they run inside.
type provisionedSlot struct {
	Name    string   // docker container name
	Mount   string   // host path of the slot's worktree (mounted as /workspace)
	Branch  string   // git branch backing the slot worktree
	MCPCmd  []string // argv for `docker exec ...` to dispatch in-slot MCP calls
	Cleanup func()   // tears down the container + worktree (idempotent)
}

// provisionSlot creates a slot worktree under
// ~/.zdx/projects/<slug>/slots/<alias>-<idx>/ and starts a docker container
// bind-mounting that worktree at /workspace. Returns once the container is
// `docker run -d`-ready (sleep infinity entrypoint) so subsequent
// `docker exec` MCP dispatches work.
//
// On failure, partial state is rolled back before returning the error —
// callers don't need to clean up after a failed provisionSlot call.
func provisionSlot(ctx context.Context, providerName, imageTag, cwd string, opts ProviderOpts, slotIdx int, agentCfg config.AgentConfig, envPairs []string) (*provisionedSlot, error) {
	wtStart := time.Now()
	wtPath, branch, err := slotWorktree(cwd, opts.RC.slug, opts.Alias, slotIdx)
	if err != nil {
		return nil, fmt.Errorf("slot %d worktree: %w", slotIdx, err)
	}
	if opts.TraceLog != nil {
		opts.TraceLog.Info("worktree.create",
			"sid", opts.SID,
			"slot_idx", slotIdx,
			"path", wtPath,
			"branch", branch,
			"took_ms", time.Since(wtStart).Milliseconds())
	}
	cleanWT := func() { cleanupSlotWorktree(cwd, wtPath, branch) }

	slotName := fmt.Sprintf("zdx-agent-mcp-%s-%s-%d", providerName, opts.Alias, slotIdx)
	runArgs := buildMCPSlotArgs(slotName, imageTag, cwd, wtPath, agentCfg, opts.KeepContainer, envPairs)
	containerStart := time.Now()
	startCmd := exec.CommandContext(ctx, "docker", runArgs...)
	startCmd.Stdout = os.Stderr // docker run -d prints the container ID
	startCmd.Stderr = os.Stderr
	if err := startCmd.Run(); err != nil {
		cleanWT()
		return nil, fmt.Errorf("start slot %d (%s): %w", slotIdx, slotName, err)
	}
	if opts.TraceLog != nil {
		opts.TraceLog.Info("container.start",
			"sid", opts.SID,
			"slot_idx", slotIdx,
			"name", slotName,
			"image", imageTag,
			"worktree", wtPath,
			"env_count", len(envPairs),
			"keep_container", opts.KeepContainer,
			"memory", agentCfg.ContainerMemory,
			"cpus", agentCfg.ContainerCPUs,
			"took_ms", time.Since(containerStart).Milliseconds())
	}

	cleanup := func() {
		out, err := exec.Command("docker", "stop", slotName).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: docker stop %s: %s\n", slotName, strings.TrimSpace(string(out)))
		}
		cleanWT()
	}

	return &provisionedSlot{
		Name:   slotName,
		Mount:  wtPath,
		Branch: branch,
		// /workspace/bin/dx-agent — absolute path, not bare `dx-agent`. The
		// dev image (dev.Dockerfile) builds without /workspace/bin on $PATH,
		// so a bare command resolves to "executable not found" and claude's
		// --mcp-config silently fails to wire up the dx-tools server.
		MCPCmd:  []string{"docker", "exec", "-i", slotName, "/workspace/bin/dx-agent", "--mcp-stdio"},
		Cleanup: cleanup,
	}, nil
}

// copyDxBinaries mirrors the operator's bin/{dx,dx-agent} into the slot
// worktree's bin/ so:
//
//	dx-agent → MCP dispatch via `docker exec /workspace/bin/dx-agent --mcp-stdio`
//	dx       → tracker calls (issue show, comment add, todo dev done)
//	          the agent runs via run_bash inside the slot
//
// bin/ is gitignored so a fresh worktree has none of these. dx-server / db
// are operator-side tools and not seeded.
func copyDxBinaries(srcRoot, dstRoot string) error {
	for _, name := range []string{"dx", "dx-agent"} {
		if err := copyFilePreservingMode(
			filepath.Join(srcRoot, "bin", name),
			filepath.Join(dstRoot, "bin", name),
		); err != nil {
			return err
		}
	}
	return nil
}

// seedSlotZdxFiles copies credentials + config.yaml + ingest-token from the
// operator's .zdx/ into the slot's worktree. .zdx/ entries are gitignored so
// a fresh worktree has none of them — and without credentials the in-slot
// dx CLI returns 401 on every tracker call. We do not bind-mount the
// operator's .zdx/ (it contains state we don't want shared) — only the
// minimal auth+config the slot needs to run.
func seedSlotZdxFiles(srcRoot, dstRoot string) error {
	dstZdx := filepath.Join(dstRoot, ".zdx")
	if err := os.MkdirAll(dstZdx, 0o755); err != nil {
		return fmt.Errorf("mkdir slot .zdx/: %w", err)
	}
	for _, name := range []string{"credentials", "config.yaml", "ingest-token"} {
		srcPath := filepath.Join(srcRoot, ".zdx", name)
		// Optional files: ingest-token may be absent in srcless mode.
		if _, err := os.Stat(srcPath); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat .zdx/%s: %w", name, err)
		}
		if err := copyFilePreservingMode(srcPath, filepath.Join(dstZdx, name)); err != nil {
			return fmt.Errorf("seed .zdx/%s: %w", name, err)
		}
	}
	return nil
}

// copyFilePreservingMode copies src to dst, creating parent dirs as needed
// and matching src's file mode (so binaries stay executable).
func copyFilePreservingMode(srcPath, dstPath string) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()
	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dstPath), err)
	}
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("create %s: %w", dstPath, err)
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("copy %s -> %s: %w", srcPath, dstPath, err)
	}
	return nil
}

// reapOrphanSlotContainers force-removes any existing slot containers whose
// names match this orchestrator's alias prefix. Triggered at startup so a
// re-execed babysit (self-update) can recover cleanly: the prior process
// died without docker-stopping its --rm containers, so they linger. Without
// this reap, the new process would hit "container name in use" on slot
// creation and fail to start.
//
// Best-effort: docker errors are logged but don't block startup. If the
// orphans persist, slot.start_failed will surface the conflict and the
// operator can intervene.
func reapOrphanSlotContainers(providerName, alias string) {
	prefix := fmt.Sprintf("zdx-agent-mcp-%s-%s-", providerName, alias)
	out, err := exec.Command("docker", "ps", "-a", "--format", "{{.Names}}", "--filter", "name="+prefix).Output()
	if err != nil {
		return
	}
	for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if name == "" || !strings.HasPrefix(name, prefix) {
			continue
		}
		if rmOut, rmErr := exec.Command("docker", "rm", "-f", name).CombinedOutput(); rmErr != nil {
			fmt.Fprintf(os.Stderr, "reap-orphan-slot: docker rm %s: %s\n", name, strings.TrimSpace(string(rmOut)))
			continue
		}
		fmt.Fprintf(os.Stderr, "reap-orphan-slot: removed %s (likely from a self-update re-exec)\n", name)
	}
}
