package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func agentClaudeCmd() *cobra.Command {
	var loop bool
	var alias string
	var issue string
	var chrome bool
	var model string
	var level string
	var container bool
	var keepContainer bool
	var maxWorktrees int
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Run Claude agent sessions with zdx integration",
		Long:  "Launch Claude CLI sessions with automatic session streaming, subagent discovery, and token usage tracking.",
		RunE: func(cmd *cobra.Command, args []string) error {
			global, _ := cmd.Flags().GetBool("global")
			var cfg *config.Config
			if !global {
				cfg = config.Load()
			}
			var rc remoteConfig
			var agentCfg config.AgentConfig
			var srcless bool
			var workDir string
			if cfg != nil {
				rc = remoteConfig{
					url:  cfg.RemoteURL(),
					slug: cfg.RemoteSlug(),
					key:  config.RemoteAPIKey(),
				}
				agentCfg = cfg.ResolvedAgent()
			} else if globalCfg := config.LoadGlobal(); globalCfg != nil {
				fmt.Fprintln(os.Stderr, "srcless mode: using ~/.zdx/config.yaml (no project config found)")
				srcless = true
				ga := globalCfg.ResolvedGlobalAgent()
				workDir = ga.WorkDir
				rc = remoteConfig{
					url: globalCfg.Remote.URL,
					key: config.GlobalRemoteAPIKey(),
					// slug intentionally empty — per-project slug comes from each claimed todo.
				}
				agentCfg = config.AgentConfig{
					ClaudeModel:  ga.ClaudeModel,
					MaxWorktrees: ga.MaxWorktrees,
					LeaseMinutes: ga.LeaseMinutes,
				}
			}

			// --max-worktrees flag overrides config value when explicitly set.
			if cmd.Flags().Changed("max-worktrees") && maxWorktrees > 0 {
				agentCfg.MaxWorktrees = maxWorktrees
			}

			if container {
				if !loop {
					return fmt.Errorf("--container requires --loop")
				}
				return runContainerLoop(alias, agentCfg, keepContainer)
			}

			sel := modelSelector{modelFlag: model, levelFlag: level, agentCfg: agentCfg}

			if loop {
				return runLoop(rc, alias, chrome, sel, srcless, workDir)
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			installReleaseOnSignal(rc, alias, "", nil, cancel)
			sid := uuid.New().String()
			resolved := sel.resolve(rc, 0)
			return runSession(ctx, rc, sid, issue, alias, chrome, "", false, resolved, 0, nil)
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().BoolVar(&chrome, "chrome", true, "pass --chrome to claude CLI")
	cmd.Flags().StringVar(&model, "model", "", "claude model name (passes through as --model to claude CLI; wins over --level)")
	cmd.Flags().StringVar(&level, "level", "", "task complexity tier: low|med|high (resolved against admin /llm-config; falls back to sensible defaults)")
	cmd.Flags().BoolVar(&container, "container", false, "run agent loop inside the project's dev container (requires --loop and dev.Dockerfile)")
	cmd.Flags().BoolVar(&keepContainer, "keep-container", false, "keep containers after exit (skip --rm; useful for debugging)")
	cmd.Flags().IntVar(&maxWorktrees, "max-worktrees", 0, "override agent.max_worktrees from config (container slots in --container mode)")
	return cmd
}

type remoteConfig struct {
	url  string
	slug string
	key  string
}

func (r remoteConfig) valid() bool {
	return r.url != "" && r.slug != "" && r.key != ""
}

// installReleaseOnSignal traps SIGINT/SIGTERM once and, before exiting,
// cancels the shared loop context (so the main loop and any in-flight
// session stop picking new work), then releases every task claimed by
// alias (treated as the agent-id) and clears the crash-recovery state
// file. A second signal triggers immediate exit.
//
// Cancel-first ordering matters: if release ran before cancel, the main
// loop could pick and start a new session during the release RTT and
// orphan its freshly-claimed task. Cancelling first stops the picker at
// its next ctx check, then we give the in-flight session a short window
// to release its own claim, then we sweep as a safety net.
//
// stateFile may be empty (single-session mode); logFn may be nil; cancel
// may be nil (no-op).
func installReleaseOnSignal(rc remoteConfig, alias, stateFile string, logFn func(string, ...any), cancel context.CancelFunc) {
	if logFn == nil {
		logFn = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "[%s] "+format+"\n",
				append([]any{time.Now().Format(time.RFC3339)}, args...)...)
		}
	}
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logFn("received signal %s: cancelling loop and releasing claimed tasks...", sig)

		if cancel != nil {
			cancel()
		}

		// Give the in-flight session up to 2s to see the cancellation
		// and release its own claim. A second signal short-circuits.
		select {
		case <-time.After(2 * time.Second):
		case sig = <-sigCh:
			logFn("second signal %s: force exit", sig)
			os.Exit(130)
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			if alias != "" {
				released, err := releaseClaimedTasks(rc, alias)
				if len(released) > 0 {
					logFn("released %d task(s): %s", len(released), strings.Join(released, ","))
				}
				if err != nil {
					logFn("release error: %v", err)
				}
			}
			if stateFile != "" {
				os.Remove(stateFile)
			}
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logFn("release timeout (5s), exiting anyway")
		case sig = <-sigCh:
			logFn("second signal %s: force exit", sig)
		}
		os.Exit(130)
	}()
}

// releaseClaimedTasks lists every task claimed by agentID and asks the server
// to release them (admin release — empty agent_id in body). Best-effort: any
// error is logged and returned for callers to surface, but we keep going so a
// single bad request does not prevent the rest from being released.
func releaseClaimedTasks(rc remoteConfig, agentID string) (released []string, err error) {
	if !rc.valid() || agentID == "" {
		return nil, nil
	}
	client := &http.Client{Timeout: 5 * time.Second}

	listURL := fmt.Sprintf("%s/api/agents/%s/tasks", rc.url, url.PathEscape(agentID))
	req, _ := http.NewRequest("GET", listURL, nil)
	req.Header.Set("X-Api-Key", rc.key)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list-agent-tasks: HTTP %d", resp.StatusCode)
	}

	var listBody struct {
		Tasks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listBody); err != nil {
		return nil, err
	}

	for _, t := range listBody.Tasks {
		if t.Status == "done" {
			continue
		}
		relURL := fmt.Sprintf("%s/api/tasks/%s/release", rc.url, url.PathEscape(t.ID))
		body := bytes.NewBufferString(`{"agent_id":""}`)
		r, _ := http.NewRequest("POST", relURL, body)
		r.Header.Set("X-Api-Key", rc.key)
		r.Header.Set("Content-Type", "application/json")
		rr, rerr := client.Do(r)
		if rerr != nil {
			err = rerr
			continue
		}
		rr.Body.Close()
		if rr.StatusCode != 200 && rr.StatusCode != 204 {
			err = fmt.Errorf("release-task %s: HTTP %d", t.ID, rr.StatusCode)
			continue
		}
		released = append(released, t.ID)
	}
	return released, err
}

func claudeProjectDir() string {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	// claude CLI replaces / with - and keeps the leading dash (e.g. cwd
	// /home/foo → slug -home-foo). Stripping it sends the tailer to an
	// empty sibling directory and silently drops every event.
	slug := strings.ReplaceAll(cwd, "/", "-")
	return filepath.Join(home, ".claude", "projects", slug)
}

// runLoop implements the --loop behavior: claim work, run sessions, repeat.
//
// In srcless mode (no project config in cwd, global ~/.zdx/config.yaml present)
// each claimed todo carries a project_slug; the loop ensures a persistent main
// clone exists at ${workDir}/${slug}/main and creates a per-session worktree
// to run the session in. workDir is empty when srcless is false.
func runLoop(rc remoteConfig, alias string, chrome bool, sel modelSelector, srcless bool, workDir string) error {
	// In srcless mode the cwd is the agent home; .zdx state lives next to the
	// global config (the cwd has no project to attach state to).
	stateFile := ".zdx/cache/claude-work-state"
	logFile := ".zdx/logs/claude-work.log"
	if srcless {
		home, _ := os.UserHomeDir()
		base := filepath.Join(home, ".zdx")
		stateFile = filepath.Join(base, "cache", "claude-work-state")
		logFile = filepath.Join(base, "logs", "claude-work.log")
		os.MkdirAll(filepath.Join(base, "logs"), 0o755)
		os.MkdirAll(filepath.Join(base, "cache"), 0o755)
	} else {
		os.MkdirAll(".zdx/logs", 0o755)
		os.MkdirAll(".zdx/cache", 0o755)
	}

	cfg := config.Load()
	var agentCfg config.AgentConfig
	if cfg != nil {
		agentCfg = cfg.ResolvedAgent()
	} else if gc := config.LoadGlobal(); gc != nil {
		ga := gc.ResolvedGlobalAgent()
		agentCfg = config.AgentConfig{
			ClaudeModel:  ga.ClaudeModel,
			MaxWorktrees: ga.MaxWorktrees,
			LeaseMinutes: ga.LeaseMinutes,
		}
	} else {
		agentCfg = (&config.Config{}).ResolvedAgent()
	}

	logf, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	defer logf.Close()

	log := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		line := fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), msg)
		fmt.Print(line)
		if logf != nil {
			logf.WriteString(line)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installReleaseOnSignal(rc, alias, stateFile, log, cancel)

	selfPath, _ := os.Executable()
	selfHash := fileHash(selfPath)

	agentID := alias
	if agentID == "" {
		agentID = "agent-" + shortID()
	}

	// Capture the loop's original cwd so per-session chdir into a srcless
	// worktree can be reverted before the next iteration.
	homeCwd, _ := os.Getwd()

	// Startup GC: drop srcless worktrees older than 2× lease (no live agent
	// could legitimately still be using them).
	if srcless && workDir != "" {
		maxAge := time.Duration(2*agentCfg.LeaseMinutes) * time.Minute
		if removed := gcStaleWorktrees(workDir, maxAge, log); len(removed) > 0 {
			log("gc: pruned %d stale srcless worktree(s)", len(removed))
		}
	}

	sessionIdx := 0
	consecutiveChurns := 0
	lastChurnTodoID := int32(0)
	for {
		if ctx.Err() != nil {
			return nil
		}

		// Self-update detection.
		if h := fileHash(selfPath); h != "" && selfHash != "" && h != selfHash {
			log("self-update: %s → %s, re-execing", shortHash(selfHash), shortHash(h))
			if err := selfReexec(selfPath, os.Args); err != nil {
				log("re-exec failed: %v", err)
			}
		}

		var issueID, sid string
		var activeTodo *claimedTodo
		resumed := false

		// Try to resume interrupted session.
		if data, err := os.ReadFile(stateFile); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 && lines[0] != "" {
				savedIssue, savedSID := lines[0], lines[1]
				status := issueStatus(savedIssue)
				if status == "open" || status == "wip" {
					log("resuming interrupted session: issue=%s sid=%s", savedIssue, savedSID)
					issueID = savedIssue
					sid = savedSID
					resumed = true
				} else {
					log("stale state: %s is %s, clearing", savedIssue, status)
					os.Remove(stateFile)
				}
			}
		}

		if !resumed {
			// Claim the next available todo via the API.
			todo, err := claimNextTodo(rc, agentID, int32(agentCfg.LeaseMinutes))
			if err != nil || todo == nil {
				log("idle (no claimable todos); sleeping 60s")
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(60 * time.Second):
				}
				continue
			}
			activeTodo = todo
			log("claimed todo %d [%s]: %s", todo.ID, todo.Kind, todo.Text)

			// Extract issue ID from the todo's issue_ref or target.
			issueID = todo.IssueRef
			if issueID == "" && todo.TargetType == "issue" {
				issueID = todo.TargetID
			}
			sid = uuid.New().String()
		}

		if ctx.Err() != nil {
			if activeTodo != nil {
				releaseTodo(rc, activeTodo.ID, agentID, sid, false)
			}
			return nil
		}

		// Save state for crash recovery.
		os.WriteFile(stateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)

		prevSID := ""
		if resumed {
			prevSID = sid
			sid = uuid.New().String()
			os.WriteFile(stateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)
			log("forking session: %s → %s", prevSID, sid)
		}

		// Srcless: clone (idempotent) + create per-session worktree, chdir
		// in so runSession launches Claude rooted at the project. Any error
		// here releases the claim and continues — better to skip a bad
		// project than to wedge the loop.
		var srclessProjectPath, srclessWorktreePath, srclessBranch string
		if srcless && activeTodo != nil && activeTodo.ProjectSlug != "" {
			pp, err := ensureProjectClone(workDir, activeTodo.ProjectSlug, rc.url)
			if err != nil {
				log("srcless: clone %s failed: %v", activeTodo.ProjectSlug, err)
				releaseTodo(rc, activeTodo.ID, agentID, sid, false)
				os.Remove(stateFile)
				continue
			}
			srclessProjectPath = pp
			if ierr := ensureProjectInit(pp, activeTodo.ProjectSlug, rc.url, rc.key, selfPath); ierr != nil {
				log("srcless: init %s failed: %v", activeTodo.ProjectSlug, ierr)
				releaseTodo(rc, activeTodo.ID, agentID, sid, false)
				os.Remove(stateFile)
				continue
			}
			wt, br, err := createSessionWorktree(pp, workDir, activeTodo.ProjectSlug, sid)
			if err != nil {
				log("srcless: worktree for %s failed: %v", activeTodo.ProjectSlug, err)
				releaseTodo(rc, activeTodo.ID, agentID, sid, false)
				os.Remove(stateFile)
				continue
			}
			srclessWorktreePath = wt
			srclessBranch = br
			if err := os.Chdir(wt); err != nil {
				log("srcless: chdir %s failed: %v", wt, err)
				_ = removeSessionWorktree(pp, wt, br)
				releaseTodo(rc, activeTodo.ID, agentID, sid, false)
				os.Remove(stateFile)
				continue
			}
			log("srcless: project=%s worktree=%s branch=%s", activeTodo.ProjectSlug, wt, br)
		}

		log("──────────────────────────────────────────────")
		log("SESSION START  session=%s  issue=%s  resumed=%v", sid, issueID, resumed)
		log("──────────────────────────────────────────────")
		startTime := time.Now()

		// Start a lease renewal goroutine for the active todo.
		var leaseCancel context.CancelFunc
		if activeTodo != nil {
			var leaseCtx context.Context
			leaseCtx, leaseCancel = context.WithCancel(ctx)
			go func(todoID int32, renewMin int32) {
				interval := time.Duration(renewMin/2) * time.Minute
				if interval < time.Minute {
					interval = time.Minute
				}
				ticker := time.NewTicker(interval)
				defer ticker.Stop()
				for {
					select {
					case <-leaseCtx.Done():
						return
					case <-ticker.C:
						renewTodoLease(rc, todoID, agentID, renewMin)
					}
				}
			}(activeTodo.ID, int32(agentCfg.LeaseMinutes))
		}

		resolvedModel := sel.resolve(rc, sessionIdx)
		sessionIdx++
		if resolvedModel != "" {
			log("model: %s", resolvedModel)
		}
		var todoID int32
		if activeTodo != nil {
			todoID = activeTodo.ID
		}
		sessionErr := runSession(ctx, rc, sid, issueID, alias, chrome, prevSID, resumed, resolvedModel, todoID, activeTodo)

		// ── Stall recovery: transparently restart the session ────────
		if errors.Is(sessionErr, ErrSessionStalled) && ctx.Err() == nil {
			stalledSID := sid
			log("session stalled, attempting resume...")

			// Attempt 1: resume the stalled session via --resume.
			resumeSID := uuid.New().String()
			os.WriteFile(stateFile, []byte(issueID+"\n"+resumeSID+"\n"), 0o644)
			log("forking stalled session: %s → %s", stalledSID, resumeSID)

			resumeStart := time.Now()
			resumeErr := runSession(ctx, rc, resumeSID, issueID, alias, chrome, stalledSID, true, resolvedModel, todoID, activeTodo)

			if resumeErr != nil && time.Since(resumeStart) < 60*time.Second {
				// Resume failed fast — likely a context/compaction issue.
				// Fall back to a fresh session seeded with a transcript summary.
				log("resume failed quickly (%v), starting fresh session with transcript summary", resumeErr)

				projDir := claudeProjectDir()
				summary := SummarizeTranscript(
					filepath.Join(projDir, stalledSID+".jsonl"),
					filepath.Join(projDir, stalledSID, "subagents"),
					30, 40,
				)

				freshSID := uuid.New().String()
				os.WriteFile(stateFile, []byte(issueID+"\n"+freshSID+"\n"), 0o644)
				log("fresh session with summary: %s (issue=%s)", freshSID, issueID)

				sessionErr = runSessionWithSummary(ctx, rc, freshSID, issueID, alias, chrome, resolvedModel, todoID, summary, activeTodo)
				sid = freshSID
			} else {
				sessionErr = resumeErr
				sid = resumeSID
			}
		}

		// Stop lease renewal.
		if leaseCancel != nil {
			leaseCancel()
		}

		if sessionErr != nil {
			log("session error: %v", sessionErr)
		}

		elapsed := time.Since(startTime)
		log("──────────────────────────────────────────────")
		log("SESSION END  session=%s  duration=%s", sid, elapsed.Truncate(time.Second))
		log("──────────────────────────────────────────────")

		// Release or resolve the claimed todo. Server checks the session for
		// recorded revisions when resolve=true; sessions with zero mutations
		// are silently downgraded to a plain release to prevent churn.
		if activeTodo != nil {
			success := sessionErr == nil
			downgraded := releaseTodo(rc, activeTodo.ID, agentID, sid, success)
			switch {
			case !success:
				log("todo %d released (session failed)", activeTodo.ID)
				consecutiveChurns = 0
				lastChurnTodoID = 0
			case downgraded:
				// Only count as churn if the same todo keeps coming back.
				// Different todos churning is normal (e.g. comment-only work
				// that the server's mutation check doesn't recognize).
				if activeTodo.ID == lastChurnTodoID {
					consecutiveChurns++
				} else {
					consecutiveChurns = 1
					lastChurnTodoID = activeTodo.ID
				}
				log("todo %d released (session made no mutations — churn guard, streak %d)", activeTodo.ID, consecutiveChurns)
			default:
				log("todo %d resolved", activeTodo.ID)
				consecutiveChurns = 0
				lastChurnTodoID = 0
			}
		}

		os.Remove(stateFile)

		// Srcless cleanup: push the session branch (success only) and tear
		// down the worktree before chdir'ing back. We always restore homeCwd
		// so the next iteration's stateFile / GC reads are stable.
		if srclessWorktreePath != "" {
			if sessionErr == nil {
				skipped, perr := pushSessionBranch(srclessWorktreePath, srclessBranch)
				switch {
				case perr != nil:
					log("srcless: push %s failed: %v", srclessBranch, perr)
				case skipped:
					log("srcless: %s had no commits to push", srclessBranch)
				default:
					log("srcless: pushed %s", srclessBranch)
				}
			}
			if rerr := removeSessionWorktree(srclessProjectPath, srclessWorktreePath, srclessBranch); rerr != nil {
				log("srcless: worktree teardown: %v", rerr)
			}
			if homeCwd != "" {
				_ = os.Chdir(homeCwd)
			}
		}

		// Exponential backoff when the same todo keeps getting churn-guarded.
		if consecutiveChurns >= 3 {
			backoff := time.Duration(1<<min(consecutiveChurns-3, 6)) * time.Minute // 1m, 2m, 4m … 64m cap
			log("churn backoff: todo %d churned %d times; sleeping %s", lastChurnTodoID, consecutiveChurns, backoff.Truncate(time.Second))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
		}
	}
}

// runSession launches a single Claude CLI session and drives its lifecycle
// through the provider-agnostic RunLifecycle runner. Event tailing, WS
// streaming, and close are all owned by the shared runner — this wrapper
// only constructs a claudeAdapter and prints the post-session token summary.
func runSession(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, prevSID string, resumed bool, model string, todoID int32, todo *claimedTodo) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	prompt := ""
	if issueID != "" {
		prompt = "/work " + issueID
	} else if todo != nil {
		// Non-issue todo (maturity nudge, stale comment, etc.) — pass the
		// todo text directly so the skill can act on it without re-claiming.
		prompt = fmt.Sprintf("/work\n\nClaimed todo %d [%s] target=%s:%s\n%s",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)
	}

	adapter := &claudeAdapter{
		projDir: projDir,
		chrome:  chrome,
		prevSID: prevSID,
		resumed: resumed,
		alias:   alias,
		model:   model,
		prompt:  prompt,
		exited:  make(chan struct{}),
	}

	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "claude-cli", todoID)

	// Print token usage summary from the on-disk transcripts regardless of
	// whether the lifecycle runner reached the server; useful in dev.
	printTokenSummary(
		filepath.Join(projDir, sid+".jsonl"),
		filepath.Join(projDir, sid, "subagents"),
	)
	return err
}

// runSessionWithSummary starts a fresh claude session whose prompt includes
// a transcript summary from a previous stalled session so the agent can
// continue the same work without --resume.
func runSessionWithSummary(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, model string, todoID int32, summary string, todo *claimedTodo) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	workCmd := "/work"
	if issueID != "" {
		workCmd = "/work " + issueID
	} else if todo != nil {
		workCmd = fmt.Sprintf("/work\n\nClaimed todo %d [%s] target=%s:%s\n%s",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)
	}
	prompt := fmt.Sprintf("%s\n\nThis session is a continuation of a stalled session. The previous session was automatically terminated because it stopped producing output (likely a stuck tool call). Below is a summary of what it accomplished. Continue the work from where it left off — do NOT repeat already-completed steps.\n\n%s", workCmd, summary)

	adapter := &claudeAdapter{
		projDir: projDir,
		chrome:  chrome,
		alias:   alias,
		model:   model,
		prompt:  prompt,
		exited:  make(chan struct{}),
	}

	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "claude-cli", todoID)
	printTokenSummary(
		filepath.Join(projDir, sid+".jsonl"),
		filepath.Join(projDir, sid, "subagents"),
	)
	return err
}

// ── Claude AgentAdapter ──────────────────────────────────────────────────

// claudeAdapter implements AgentAdapter against the real `claude` CLI. It
// launches the process with ZDX-aware environment vars and returns the
// transcript path that Claude writes its JSONL session to.
type claudeAdapter struct {
	projDir string
	chrome  bool
	prevSID string
	resumed bool
	alias   string
	model   string
	prompt  string // custom prompt; empty = "/work"

	proc       *exec.Cmd
	exited     chan struct{}
	exitedOnce sync.Once

	toolNamesMu sync.Mutex
	toolNames   map[string]string
}

func (a *claudeAdapter) Provider() string { return "claude" }

func (a *claudeAdapter) Start(ctx context.Context, sid, _, _ string) (string, error) {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found in PATH")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "--dangerously-skip-permissions")
	if a.chrome {
		cmdArgs = append(cmdArgs, "--chrome")
	}
	if a.model != "" {
		cmdArgs = append(cmdArgs, "--model", a.model)
	}
	prompt := a.prompt
	if prompt == "" {
		prompt = "/work"
	}
	if a.resumed && a.prevSID != "" {
		cmdArgs = append(cmdArgs, "--resume", a.prevSID, "--fork-session", "--session-id", sid, "-p", prompt)
	} else {
		cmdArgs = append(cmdArgs, "--session-id", sid, "-p", prompt)
	}

	a.proc = exec.Command(claudePath, cmdArgs...)
	a.proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	a.proc.Stdin = os.Stdin
	a.proc.Stdout = os.Stdout
	a.proc.Stderr = os.Stderr
	a.proc.Env = append(os.Environ(),
		"ZDX_SESSION_ID="+sid,
		"ZDX_AGENT_ID="+a.alias,
		"DX_AUTHOR_ALIAS="+a.alias,
	)

	if err := a.proc.Start(); err != nil {
		return "", err
	}

	// When the shared loop context cancels (e.g. SIGINT on the parent),
	// send SIGTERM to claude so the session wraps up cleanly; escalate to
	// SIGKILL after a grace window if it hasn't exited. Wait() observes the
	// termination and returns the real exit code, so SESSION END gets logged.
	go func() {
		<-ctx.Done()
		p := a.proc.Process
		if p == nil {
			return
		}
		// Signal the process group (negative PID) so claude and any
		// children it spawned all receive SIGTERM.
		_ = syscall.Kill(-p.Pid, syscall.SIGTERM)
		select {
		case <-a.exited:
			return
		case <-time.After(10 * time.Second):
			_ = syscall.Kill(-p.Pid, syscall.SIGKILL)
		}
	}()

	return filepath.Join(a.projDir, sid+".jsonl"), nil
}

func (a *claudeAdapter) Wait() (int, error) {
	if a.proc == nil {
		return 1, fmt.Errorf("claudeAdapter.Wait: process not started")
	}
	err := a.proc.Wait()
	a.exitedOnce.Do(func() { close(a.exited) })
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return 1, err
	}
	return a.proc.ProcessState.ExitCode(), nil
}

func (a *claudeAdapter) SubagentDir(sid string) string {
	return filepath.Join(a.projDir, sid, "subagents")
}

func (a *claudeAdapter) ParseLine(line []byte, agentID string) (AgentEvent, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return AgentEvent{}, fmt.Errorf("empty line")
	}
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &peek); err != nil {
		return AgentEvent{}, err
	}
	return AgentEvent{
		EventType: peek.Type,
		EventJSON: json.RawMessage(append([]byte(nil), trimmed...)),
		AgentID:   agentID,
	}, nil
}

func (a *claudeAdapter) RenderEvent(eventJSON []byte) string {
	a.toolNamesMu.Lock()
	if a.toolNames == nil {
		a.toolNames = map[string]string{}
	}
	defer a.toolNamesMu.Unlock()
	return renderSessionEvent(eventJSON, a.toolNames)
}

// ANSI color codes. Emitted only when colorsEnabled is true (TTY + no NO_COLOR).
const (
	ansiReset   = "\033[0m"
	ansiDim     = "\033[2m"
	ansiBold    = "\033[1m"
	ansiCyan    = "\033[36m"
	ansiGreen   = "\033[32m"
	ansiRed     = "\033[31m"
	ansiMagenta = "\033[35m"
	ansiBlue    = "\033[34m"
)

var colorsEnabled = computeColorsEnabled()

func computeColorsEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func colorize(codes, s string) string {
	if !colorsEnabled || codes == "" {
		return s
	}
	return codes + s + ansiReset
}

// renderSessionEvent turns a single JSONL line from a Claude transcript into
// one or more human-readable lines (newline-separated) for stdout + agent.log.
// Returns "" for events that should not be rendered (queue-operation,
// attachment, non-tool-result user messages, malformed JSON).
//
// Output shape matches the retired bin/claude-work-render:
//
//	[Tool] detail            cyan bold (+ dim detail)
//	text                     plain
//	⟡ thinking               blue dim
//	● title                  magenta bold
//	✓ tool: content snippet  green dim (tool_result ok)
//	✗ tool: content snippet  red (tool_result err)
//
// toolNames accumulates tool_use id → name so later tool_result lines can
// label which tool returned.
func renderSessionEvent(line []byte, toolNames map[string]string) string {
	var evt struct {
		Type    string          `json:"type"`
		Title   string          `json:"title"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return ""
	}
	switch evt.Type {
	case "queue-operation", "attachment":
		return ""
	case "ai-title":
		t := strings.TrimSpace(evt.Title)
		if t == "" {
			return ""
		}
		return colorize(ansiBold+ansiMagenta, "● "+t)
	case "assistant":
		var m struct {
			Content []struct {
				Type     string          `json:"type"`
				Text     string          `json:"text"`
				Thinking string          `json:"thinking"`
				Name     string          `json:"name"`
				ID       string          `json:"id"`
				Input    json.RawMessage `json:"input"`
			} `json:"content"`
		}
		if err := json.Unmarshal(evt.Message, &m); err != nil {
			return ""
		}
		var lines []string
		for _, c := range m.Content {
			switch c.Type {
			case "tool_use":
				if c.ID != "" && c.Name != "" {
					toolNames[c.ID] = c.Name
				}
				lines = append(lines, renderToolUse(c.Name, c.Input))
			case "text":
				s := flattenTrunc(c.Text, 200)
				if s == "" {
					continue
				}
				lines = append(lines, s)
			case "thinking":
				s := flattenTrunc(c.Thinking, 100)
				if s == "" {
					continue
				}
				lines = append(lines, colorize(ansiDim+ansiBlue, "⟡ "+s))
			}
		}
		return strings.Join(lines, "\n")
	case "user":
		var m struct {
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(evt.Message, &m); err != nil {
			return ""
		}
		raw := bytes.TrimSpace(m.Content)
		if len(raw) == 0 || raw[0] != '[' {
			return ""
		}
		var arr []struct {
			Type      string          `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			IsError   bool            `json:"is_error"`
			Content   json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(raw, &arr); err != nil {
			return ""
		}
		var lines []string
		for _, c := range arr {
			if c.Type != "tool_result" {
				continue
			}
			name := toolNames[c.ToolUseID]
			if name == "" {
				name = "tool"
			}
			snippet := toolResultSnippet(c.Content, 120)
			if c.IsError {
				label := "✗ " + name
				if snippet != "" {
					label += ": " + snippet
				}
				lines = append(lines, colorize(ansiRed, label))
			} else {
				label := "✓ " + name
				if snippet != "" {
					label += ": " + snippet
				}
				lines = append(lines, colorize(ansiDim+ansiGreen, label))
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// flattenTrunc collapses newlines to spaces, trims, and truncates to n runes
// with a trailing ellipsis. Returns "" if the input is blank.
func flattenTrunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// toolResultSnippet extracts a short text snippet from tool_result content,
// which can be either a JSON string or an array of content blocks (text /
// image / etc). Returns "" when no usable text is present.
func toolResultSnippet(content json.RawMessage, n int) string {
	raw := bytes.TrimSpace(content)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return ""
		}
		return flattenTrunc(s, n)
	case '[':
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &blocks) != nil {
			return ""
		}
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return flattenTrunc(strings.Join(parts, " "), n)
	}
	return ""
}

func renderToolUse(name string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	summary := ""
	switch name {
	case "Bash":
		if s, ok := in["command"].(string); ok {
			summary = flattenTrunc(s, 80)
		}
	case "Read", "Edit", "Write", "NotebookEdit":
		if s, ok := in["file_path"].(string); ok {
			summary = s
		}
	case "Grep", "Glob":
		if s, ok := in["pattern"].(string); ok {
			summary = s
		}
	case "Agent":
		if s, ok := in["description"].(string); ok && s != "" {
			summary = s
		} else if s, ok := in["subagent_type"].(string); ok {
			summary = s
		}
	case "WebFetch", "WebSearch":
		if s, ok := in["url"].(string); ok {
			summary = s
		} else if s, ok := in["query"].(string); ok {
			summary = s
		}
	}
	if name == "" {
		name = "tool"
	}
	label := colorize(ansiBold+ansiCyan, "["+name+"]")
	if summary == "" {
		return label
	}
	return label + " " + colorize(ansiDim, summary)
}

func parseSubagentMeta(jsonlPath string) (id, agentType, desc string) {
	base := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
	id = base

	metaPath := strings.TrimSuffix(jsonlPath, ".jsonl") + ".meta.json"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		// Meta file might not exist yet, wait a bit
		for i := 0; i < 5; i++ {
			time.Sleep(500 * time.Millisecond)
			data, err = os.ReadFile(metaPath)
			if err == nil {
				break
			}
		}
	}
	if err == nil {
		var meta struct {
			AgentType   string `json:"agentType"`
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &meta) == nil {
			agentType = meta.AgentType
			desc = meta.Description
		}
	}
	return
}

type tokenUsage struct {
	Input       int64
	Output      int64
	CacheRead   int64
	CacheCreate int64
}

func printTokenSummary(sfile, subagentDir string) {
	total := parseTokenUsage(sfile)

	matches, _ := filepath.Glob(filepath.Join(subagentDir, "agent-*.jsonl"))
	for _, m := range matches {
		sub := parseTokenUsage(m)
		total.Input += sub.Input
		total.Output += sub.Output
		total.CacheRead += sub.CacheRead
		total.CacheCreate += sub.CacheCreate
	}

	if total.Input+total.Output > 0 {
		fmt.Printf("tokens: input=%d output=%d cache_read=%d cache_create=%d\n",
			total.Input, total.Output, total.CacheRead, total.CacheCreate)
	}
}

func parseTokenUsage(path string) tokenUsage {
	var t tokenUsage
	f, err := os.Open(path)
	if err != nil {
		return t
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens              int64 `json:"input_tokens"`
					OutputTokens             int64 `json:"output_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &ev) == nil && ev.Type == "assistant" {
			t.Input += ev.Message.Usage.InputTokens
			t.Output += ev.Message.Usage.OutputTokens
			t.CacheRead += ev.Message.Usage.CacheReadInputTokens
			t.CacheCreate += ev.Message.Usage.CacheCreationInputTokens
		}
	}
	return t
}

func runDxTodoSolo(issue string) (string, error) {
	var args []string
	args = append(args, "todo", "solo")
	if issue != "" {
		args = append(args, "--issue="+issue)
	}
	dxPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(dxPath, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func issueStatus(issueID string) string {
	dxPath, _ := os.Executable()
	out, err := exec.Command(dxPath, "todo", "show", issueID).CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Status:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func extractIssueID(text string) string {
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, "IS-") {
			return word
		}
	}
	return ""
}

// ── Todo claiming helpers ─────────────────────────────────────────────────

type claimedTodo struct {
	ID          int32  `json:"id"`
	Text        string `json:"text"`
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	TargetType  string `json:"target_type"`
	TargetID    string `json:"target_id"`
	IssueRef    string `json:"issue_ref"`
	Priority    int32  `json:"priority"`
	ClaimedBy   string `json:"claimed_by"`
	ProjectSlug string `json:"project_slug,omitempty"`
}

func claimNextTodo(rc remoteConfig, agentID string, leaseMinutes int32) (*claimedTodo, error) {
	body, _ := json.Marshal(map[string]any{
		"slug":          rc.slug,
		"agent_id":      agentID,
		"lease_minutes": leaseMinutes,
		"mode":          "autonomous",
	})
	req, _ := http.NewRequest("POST", rc.url+"/api/dx/solo/claim", bytes.NewReader(body))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("claim HTTP %d", resp.StatusCode)
	}
	var todo claimedTodo
	if err := json.NewDecoder(resp.Body).Decode(&todo); err != nil {
		return nil, err
	}
	return &todo, nil
}

func renewTodoLease(rc remoteConfig, todoID int32, agentID string, leaseMinutes int32) {
	body, _ := json.Marshal(map[string]any{
		"id":            todoID,
		"agent_id":      agentID,
		"lease_minutes": leaseMinutes,
	})
	req, _ := http.NewRequest("POST", rc.url+"/api/dx/solo/renew", bytes.NewReader(body))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err == nil {
		resp.Body.Close()
	}
}

// releaseTodo posts the release/resolve call and returns whether the server
// downgraded a resolve to a plain release because the session recorded no
// mutations (churn guard — see handlers_solo.go).
func releaseTodo(rc remoteConfig, todoID int32, agentID, sessionID string, resolve bool) bool {
	body, _ := json.Marshal(map[string]any{
		"id":         todoID,
		"agent_id":   agentID,
		"resolve":    resolve,
		"session_id": sessionID,
	})
	req, _ := http.NewRequest("POST", rc.url+"/api/dx/solo/release", bytes.NewReader(body))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil || resp == nil {
		return false
	}
	defer resp.Body.Close()
	var r struct {
		ChurnDowngraded bool `json:"churn_downgraded"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return r.ChurnDowngraded
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func shortHash(s string) string {
	if len(s) < 8 {
		return s
	}
	return s[:8]
}

// selfReexec replaces the current process image with a fresh exec of the
// binary. syscall.Exec does not return on success, so any return is an error
// and the caller should keep the old binary running until the next tick.
func selfReexec(path string, args []string) error {
	return syscall.Exec(path, args, os.Environ())
}
