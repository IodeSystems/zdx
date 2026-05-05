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

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
)

func agentClaudeCmd() *cobra.Command {
	var loop bool
	var alias string
	var issue string
	var chrome bool
	var model string
	var complexity string
	var container bool
	var keepContainer bool
	var maxWorktrees int
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Run Claude agent sessions with zdx integration",
		Long:  "Launch Claude CLI sessions with automatic session streaming, subagent discovery, and token usage tracking.",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalized, err := NormalizeComplexity(complexity)
			if err != nil {
				return err
			}
			complexity = normalized
			global, _ := cmd.Flags().GetBool("global")
			global = global || config.IsGlobalMode()
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
				return runContainerLoop(rc, alias, agentCfg, keepContainer)
			}

			if err := enforceContainerExecution(container); err != nil {
				return err
			}

			sel := modelSelector{modelFlag: model, complexity: complexity, agentCfg: agentCfg}

			if loop {
				return runLoop(rc, alias, chrome, sel, srcless, workDir)
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			installReleaseOnSignal(rc, alias, "", nil, cancel)
			sid := uuid.New().String()
			resolved := sel.resolve(rc, 0)
			return runSession(ctx, rc, sid, issue, alias, chrome, "", false, resolved, 0, nil, srcless)
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().BoolVar(&chrome, "chrome", true, "pass --chrome to claude CLI")
	cmd.Flags().StringVar(&model, "model", "", "claude model name (passes through as --model to claude CLI; wins over --complexity)")
	cmd.Flags().StringVar(&complexity, "complexity", DefaultComplexity, "model tier: low|medium|high (resolved against admin /llm-config; falls back to sensible defaults)")
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

// fetchProjectVision returns a one-line vision string for the project, or ""
// if unavailable. Best-effort — never blocks the session on failure.
func fetchProjectVision(rc remoteConfig) string {
	if !rc.valid() {
		return ""
	}
	req, err := http.NewRequest("GET", rc.url+"/api/projects", nil)
	if err != nil {
		return ""
	}
	req.Header.Set("X-Api-Key", rc.key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()
	var body struct {
		Projects []struct {
			Slug        string  `json:"slug"`
			Title       *string `json:"title"`
			Description *string `json:"description"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return ""
	}
	for _, p := range body.Projects {
		if p.Slug == rc.slug {
			parts := []string{}
			if p.Title != nil && *p.Title != "" {
				parts = append(parts, *p.Title)
			}
			if p.Description != nil && *p.Description != "" {
				parts = append(parts, *p.Description)
			}
			return strings.Join(parts, " — ")
		}
	}
	return ""
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
	lastChurnTodoKey := ""
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

		// Build TakeConfig for this iteration.
		takeCfg := TakeConfig{
			RC:         rc,
			AgentID:    agentID,
			Alias:      alias,
			Chrome:     chrome,
			Srcless:    srcless,
			WorkDir:    workDir,
			HomeCwd:    homeCwd,
			AgentCfg:   agentCfg,
			ModelSel:   sel,
			SessionIdx: sessionIdx,
			SelfPath:   selfPath,
			StateFile:  stateFile,
			LogFn:      log,
		}

		// Check for crash-recovery state from a previous interrupted session.
		if data, err := os.ReadFile(stateFile); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 && lines[0] != "" {
				savedIssue, savedSID := lines[0], lines[1]
				status := issueStatus(savedIssue)
				if status == "open" || status == "wip" {
					takeCfg.ResumeIssueID = savedIssue
					takeCfg.ResumeSID = savedSID
				} else {
					log("stale state: %s is %s, clearing", savedIssue, status)
					os.Remove(stateFile)
				}
			}
		}

		result := Take(ctx, takeCfg)
		sessionIdx++

		// Handle idle (no work available).
		if errors.Is(result.Err, ErrNoWork) {
			log("idle (no claimable todos); sleeping 60s")
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(60 * time.Second):
			}
			continue
		}

		// Track churn across iterations by todo key.
		switch {
		case result.CycleDetected:
			consecutiveChurns = 0
			lastChurnTodoKey = ""
		case result.ChurnDowngraded:
			if result.TodoKey == lastChurnTodoKey {
				consecutiveChurns++
			} else {
				consecutiveChurns = 1
				lastChurnTodoKey = result.TodoKey
			}
		default:
			consecutiveChurns = 0
			lastChurnTodoKey = ""
		}

		// Exponential backoff when the same todo keeps getting churn-guarded.
		if consecutiveChurns >= 3 {
			backoff := time.Duration(1<<min(consecutiveChurns-3, 6)) * time.Minute // 1m, 2m, 4m … 64m cap
			log("churn backoff: key %s churned %d times; sleeping %s", lastChurnTodoKey, consecutiveChurns, backoff.Truncate(time.Second))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
		}
	}
}

// buildSessionPrompt assembles the -p prompt for a Claude session from the
// project vision, optional issue ID, and optional claimed todo.
func buildSessionPrompt(vision, issueID string, todo *claimedTodo) string {
	prompt := ""
	if vision != "" {
		prompt = "Project vision: " + vision + "\n\n"
	}
	if todo != nil {
		// Pass the todo text directly as the prompt — the work instructions
		// are already embedded in the todo by the queue generator (IS-382).
		// This must take precedence over the issueID-only fallback so that
		// kind-specific playbooks (triage, decompose, etc.) reach the agent.
		prompt += fmt.Sprintf("Claimed todo %d [%s] target=%s:%s\n\n%s",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)
		prompt += "\n\nCOMMIT RULE: Only commit intent files (migrations, queries/*.sql, handler source). Do not commit internal/db/*.sql.go, internal/dxclient/models.gen.go, ui/src/api.gen.ts, schema/shipped.sql. Use dx commit --intent."
		prompt += fmt.Sprintf(`

---
INCOMPLETE-REPORT PROTOCOL

If you cannot complete this task, you MUST run:

  dx todo incomplete %s --reason=<reason> --explanation="<what happened>" [--suggested-next="<hint>"]

before ending the session. Silent exit without either "dx todo dev done" or "dx todo incomplete" is a protocol violation.

Reason taxonomy:
  capability_gap          — needs a tool or ability the agent doesn't have
  ambiguous_spec          — spec is unclear or contradictory; needs clarification
  external_dep            — blocked on work in another issue/PR
  needs_decision          — requires a human or architectural decision before proceeding
  permission_denied       — lacks access to a resource
  environment_broken      — local environment/infra is broken
  preexisting_test_failure — test was already failing before this session
  flaky_test              — non-deterministic test failure unrelated to this change

Example --suggested-next values:
  "block on IS-N"
  "ask user: what should X do when Y?"
  "file capability request: need ability to read network responses"
---`, todo.TargetID)
	} else if issueID != "" {
		prompt += fmt.Sprintf("Work on issue %s. Use ./bin/dx CLI commands (issue show, comment add, todo dev done) to interact with the project tracker.", issueID)
	}
	return prompt
}

// runSession launches a single Claude CLI session and drives its lifecycle
// through the provider-agnostic RunLifecycle runner. Event tailing, WS
// streaming, and close are all owned by the shared runner — this wrapper
// only constructs a claudeAdapter and prints the post-session token summary.
func runSession(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, prevSID string, resumed bool, model string, todoID int32, todo *claimedTodo, srcless bool) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	// Fetch project vision to provide context for the session.
	vision := fetchProjectVision(rc)
	prompt := buildSessionPrompt(vision, issueID, todo)

	adapter := &claudeAdapter{
		rc:      rc,
		projDir: projDir,
		chrome:  chrome,
		prevSID: prevSID,
		resumed: resumed,
		alias:   alias,
		model:   model,
		prompt:  prompt,
		srcless: srcless,
		exited:  make(chan struct{}),
	}

	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "claude-cli", todoID)

	// Print token usage summary from the on-disk transcripts regardless of
	// whether the lifecycle runner reached the server; useful in dev.
	transcriptPath := filepath.Join(projDir, sid+".jsonl")
	printTokenSummary(transcriptPath, filepath.Join(projDir, sid, "subagents"))
	printPatternAnalysis(rc, transcriptPath, issueID, todo)
	return err
}

// runSessionWithSummary starts a fresh claude session whose prompt includes
// a transcript summary from a previous stalled session so the agent can
// continue the same work without --resume.
func runSessionWithSummary(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, model string, todoID int32, summary string, todo *claimedTodo, srcless bool) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	taskPrompt := ""
	if todo != nil {
		taskPrompt = fmt.Sprintf("Claimed todo %d [%s] target=%s:%s\n\n%s",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)
	} else if issueID != "" {
		taskPrompt = fmt.Sprintf("Work on issue %s. Use ./bin/dx CLI commands (issue show, comment add, todo dev done) to interact with the project tracker.", issueID)
	}
	prompt := fmt.Sprintf("%s\n\nThis session is a continuation of a stalled session. The previous session was automatically terminated because it stopped producing output (likely a stuck tool call). Below is a summary of what it accomplished. Continue the work from where it left off — do NOT repeat already-completed steps.\n\n%s", taskPrompt, summary)

	adapter := &claudeAdapter{
		rc:      rc,
		projDir: projDir,
		chrome:  chrome,
		alias:   alias,
		model:   model,
		prompt:  prompt,
		srcless: srcless,
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

func init() {
	RegisterProvider("claude", func(opts ProviderOpts) (AgentProvider, error) {
		projDir := claudeProjectDir()
		_ = os.MkdirAll(projDir, 0o755)
		vision := fetchProjectVision(opts.RC)
		prompt := buildSessionPrompt(vision, opts.IssueID, nil)
		// Default chrome=true to match the legacy claude command's flag default.
		chrome := opts.Chrome
		return &claudeAdapter{
			rc:       opts.RC,
			agentCfg: opts.AgentCfg,
			projDir:  projDir,
			chrome:   chrome,
			alias:    opts.Alias,
			model:    opts.Model,
			prompt:   prompt,
			srcless:  opts.Srcless,
			exited:   make(chan struct{}),
		}, nil
	})
}

// claudeAdapter implements AgentAdapter against the real `claude` CLI. It
// launches the process with ZDX-aware environment vars and returns the
// transcript path that Claude writes its JSONL session to.
type claudeAdapter struct {
	rc       remoteConfig
	agentCfg config.AgentConfig // honored by ResolveModel for the medium-tier ClaudeModel fallback
	projDir  string
	chrome   bool
	prevSID  string
	resumed  bool
	alias    string
	model    string
	prompt   string // custom prompt; empty = "/work"
	srcless  bool   // when true, inject DX_GLOBAL=1 into the subprocess env

	scopedTokenID int32

	proc       *exec.Cmd
	exited     chan struct{}
	exitedOnce sync.Once

	toolNamesMu sync.Mutex
	toolNames   map[string]string
}

// ResolveModel maps a complexity tier to a concrete claude model identifier.
// Honors a.rc + a.agentCfg for the existing admin/llm-config + ClaudeModel
// fallback chain. Empty complexity is treated as DefaultComplexity.
func (a *claudeAdapter) ResolveModel(_ context.Context, complexity string) (string, error) {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return "", err
	}
	return resolveComplexityModel(a.rc, tier, a.agentCfg), nil
}

// buildClaudeEnv builds the environment passed to the spawned claude CLI.
// Extracted from Start() so the env wiring (including DX_GLOBAL in srcless
// mode) is unit-testable without spawning a subprocess.
// scopedToken replaces any DX_REMOTE_API_KEY in base; pass "" to skip injection.
func buildClaudeEnv(base []string, sid, alias string, srcless bool, scopedToken string) []string {
	filtered := make([]string, 0, len(base))
	for _, kv := range base {
		if strings.HasPrefix(kv, "DX_REMOTE_API_KEY=") {
			continue
		}
		filtered = append(filtered, kv)
	}
	if scopedToken != "" {
		filtered = append(filtered, "DX_REMOTE_API_KEY="+scopedToken)
	}
	env := append(filtered,
		"ZDX_SESSION_ID="+sid,
		"ZDX_AGENT_ID="+alias,
		"DX_AUTHOR_ALIAS="+alias,
	)
	if srcless {
		env = append(env, "DX_GLOBAL=1")
	}
	return env
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

	scopedToken, tokenID, err := mintScopedToken(ctx, a.rc, "agent-claude-"+a.alias+"-"+sid[:8])
	if err != nil {
		return "", fmt.Errorf("mint scoped token: %w", err)
	}
	a.scopedTokenID = tokenID

	a.proc = exec.Command(claudePath, cmdArgs...)
	a.proc.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	a.proc.Stdin = os.Stdin
	a.proc.Stdout = os.Stdout
	a.proc.Stderr = os.Stderr
	a.proc.Env = buildClaudeEnv(os.Environ(), sid, a.alias, a.srcless, scopedToken)

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
	if a.scopedTokenID != 0 {
		revokeScopedToken(context.Background(), a.rc, a.scopedTokenID)
	}
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

// ExtractAuditEvents implements AuditExtractor. It parses tool_use blocks from
// assistant events and returns synthetic audit AgentEvents with event_type
// tool_call / file_edit / shell_cmd for the HTTP ingestion fallback path.
func (a *claudeAdapter) ExtractAuditEvents(line []byte, agentID string) []AgentEvent {
	var evt struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type  string          `json:"type"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &evt) != nil || evt.Type != "assistant" {
		return nil
	}
	var events []AgentEvent
	for _, c := range evt.Message.Content {
		if c.Type != "tool_use" {
			continue
		}
		payload, err := json.Marshal(map[string]any{
			"tool":  c.Name,
			"input": c.Input,
		})
		if err != nil {
			continue
		}
		events = append(events, AgentEvent{
			EventType: claudeClassifyToolUse(c.Name),
			EventJSON: json.RawMessage(payload),
			AgentID:   agentID,
		})
	}
	return events
}

func claudeClassifyToolUse(name string) string {
	switch name {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		return "file_edit"
	case "Bash":
		return "shell_cmd"
	default:
		return "tool_call"
	}
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

// extractTranscriptTitle reads a Claude JSONL transcript and returns the first
// ai-title event's title. Returns "" if none found.
func extractTranscriptTitle(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var ev struct {
			Type  string `json:"type"`
			Title string `json:"title"`
		}
		if json.Unmarshal(scanner.Bytes(), &ev) == nil && ev.Type == "ai-title" && ev.Title != "" {
			return ev.Title
		}
	}
	return ""
}

// patternNameFromText generates a kebab-case pattern name suggestion from
// the first few significant words of text.
func patternNameFromText(text string) string {
	skip := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"for": true, "to": true, "in": true, "of": true, "on": true,
		"is": true, "be": true, "use": true, "with": true,
	}
	var parts []string
	for _, w := range strings.Fields(text) {
		w = strings.ToLower(w)
		clean := strings.Map(func(r rune) rune {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return '-'
		}, w)
		clean = strings.Trim(clean, "-")
		if clean == "" || skip[clean] {
			continue
		}
		parts = append(parts, clean)
		if len(parts) >= 4 {
			break
		}
	}
	if len(parts) == 0 {
		return "new-pattern"
	}
	return strings.Join(parts, "-")
}

// printPatternAnalysis compares the completed session's work against the
// pattern library and prints a "Pattern recommendations" section so the
// operator can evolve the library without manual tracking.
//
// Three cases:
//   - Missing pattern (low similarity): suggest dx pattern add
//   - Incomplete pattern (medium similarity): suggest dx pattern refine PT-N
//   - Good coverage (high similarity): list matched patterns, prompt misalignment check
func printPatternAnalysis(rc remoteConfig, transcriptPath, issueID string, todo *claimedTodo) {
	if !rc.valid() {
		return
	}

	var textParts []string
	if issueID != "" {
		textParts = append(textParts, issueID)
	}
	if todo != nil && todo.Text != "" {
		textParts = append(textParts, todo.Text)
	}
	if title := extractTranscriptTitle(transcriptPath); title != "" {
		textParts = append(textParts, title)
	}
	if len(textParts) == 0 {
		return
	}
	searchText := strings.Join(textParts, " ")

	body, _ := json.Marshal(map[string]any{
		"slug": rc.slug,
		"text": searchText,
		"n":    5,
	})
	req, _ := http.NewRequest("POST", rc.url+"/api/dx/patterns/similar", bytes.NewReader(body))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/json")
	httpResp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return
	}
	defer httpResp.Body.Close()

	var result struct {
		Patterns []struct {
			Pattern struct {
				ID   int32  `json:"id"`
				Name string `json:"name"`
			} `json:"pattern"`
			Score float64 `json:"score"`
		} `json:"patterns"`
	}
	if json.NewDecoder(httpResp.Body).Decode(&result) != nil {
		return
	}

	fmt.Println("\nPattern recommendations:")

	if len(result.Patterns) == 0 {
		// Pattern library is empty.
		fmt.Printf("  No patterns in library — consider: dx pattern add %q --desc=\"<approach used>\"\n",
			patternNameFromText(searchText))
		return
	}

	top := result.Patterns[0]
	switch {
	case top.Score < 0.40:
		fmt.Printf("  Missing pattern (best match %.0f%%) — this work isn't captured yet.\n", top.Score*100)
		fmt.Printf("  Suggest: dx pattern add %q --desc=\"<describe the approach used>\"\n",
			patternNameFromText(searchText))
	case top.Score < 0.65:
		fmt.Printf("  Incomplete pattern: PT-%d %s (%.0f%% match)\n", top.Pattern.ID, top.Pattern.Name, top.Score*100)
		fmt.Printf("  Suggest: dx pattern refine PT-%d --add=\"<detail about this specific case>\"\n", top.Pattern.ID)
	default:
		var matched []string
		for _, p := range result.Patterns {
			if p.Score >= 0.65 {
				matched = append(matched, fmt.Sprintf("PT-%d %s (%.0f%%)", p.Pattern.ID, p.Pattern.Name, p.Score*100))
			}
		}
		fmt.Printf("  Matched: %s\n", strings.Join(matched, "  "))
		fmt.Printf("  Review: did the session follow these patterns? If it deviated: dx pattern refine PT-N --add=\"<correction>\"\n")
	}
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
	ID              int32  `json:"id"`
	Text            string `json:"text"`
	Key             string `json:"key"`
	Kind            string `json:"kind"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	IssueRef        string `json:"issue_ref"`
	TargetBranch    string `json:"target_branch,omitempty"`
	Priority        int32  `json:"priority"`
	ClaimedBy       string `json:"claimed_by"`
	ProjectSlug     string `json:"project_slug,omitempty"`
	ClaimBaseSha    string `json:"claim_base_sha,omitempty"`
	ClaimBaseBranch string `json:"claim_base_branch,omitempty"`
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

type releaseResult struct {
	ChurnDowngraded bool
	CycleDetected   bool
}

// releaseTodo posts the release/resolve call. The server may report:
//   - churn_downgraded: session had no mutations, resolve was downgraded to release
//   - cycle_detected: the resolved todo would be immediately regenerated by the queue,
//     indicating the agent cannot actually fix the underlying condition. Server auto-blocks.
func releaseTodo(rc remoteConfig, todoID int32, agentID, sessionID, claimBaseSHA, todoKind, claimBaseBranch string, resolve bool) releaseResult {
	body := map[string]any{
		"id":         todoID,
		"agent_id":   agentID,
		"resolve":    resolve,
		"session_id": sessionID,
	}
	if bs := cli.CollectBranchState(claimBaseSHA); bs != nil {
		body["branch_state"] = bs
		if resolve {
			if err := cli.ValidateBranchState(todoKind, bs, claimBaseSHA, claimBaseBranch); err != nil {
				fmt.Fprintf(os.Stderr, "branch contract violation: %v\ndowngrading resolve to release\n", err)
				body["resolve"] = false
			}
		}
	}
	encoded, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", rc.url+"/api/dx/solo/release", bytes.NewReader(encoded))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil || resp == nil {
		return releaseResult{}
	}
	defer resp.Body.Close()
	var r struct {
		ChurnDowngraded bool `json:"churn_downgraded"`
		CycleDetected   bool `json:"cycle_detected"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return releaseResult{ChurnDowngraded: r.ChurnDowngraded, CycleDetected: r.CycleDetected}
}

// emitFallbackIncompleteReport checks whether a todo already has an
// incomplete-report and, if not, posts a generic one. Best-effort: errors are
// logged but never block the release flow.
func emitFallbackIncompleteReport(rc remoteConfig, slug, key, agentID string, log func(string, ...any)) {
	if rc.url == "" || rc.key == "" || slug == "" || key == "" {
		return
	}
	client := &http.Client{Timeout: 10 * time.Second}
	reportsURL := fmt.Sprintf("%s/api/dx/projects/%s/todos/%s/incomplete-reports",
		rc.url, url.PathEscape(slug), url.PathEscape(key))

	req, err := http.NewRequest("GET", reportsURL, nil)
	if err != nil {
		log("fallback-report: build req: %v", err)
		return
	}
	req.Header.Set("X-Api-Key", rc.key)
	resp, err := client.Do(req)
	if err != nil {
		log("fallback-report: list: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log("fallback-report: list HTTP %d", resp.StatusCode)
		return
	}
	var items []json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		log("fallback-report: decode: %v", err)
		return
	}
	if len(items) > 0 {
		return // already has a report — skip duplicate
	}

	payload, _ := json.Marshal(map[string]any{
		"reason":         "needs_decision",
		"explanation":    "Session ended without emitting an incomplete-report or a done signal. Manual triage required.",
		"agent_id":       agentID,
		"suggested_next": "Investigate session transcript and re-claim or reassign.",
	})
	postReq, err := http.NewRequest("POST", reportsURL, bytes.NewReader(payload))
	if err != nil {
		log("fallback-report: build post req: %v", err)
		return
	}
	postReq.Header.Set("X-Api-Key", rc.key)
	postReq.Header.Set("Content-Type", "application/json")
	postResp, err := client.Do(postReq)
	if err != nil {
		log("fallback-report: post: %v", err)
		return
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusCreated {
		log("fallback-report: post HTTP %d", postResp.StatusCode)
		return
	}
	log("fallback-report: emitted for %s/%s", slug, key)
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
