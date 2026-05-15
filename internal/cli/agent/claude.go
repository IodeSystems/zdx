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

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/config"
)

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
func runLoop(rc remoteConfig, alias string, chrome bool, sel modelSelector, srcless bool, workDir string, agentCfg config.AgentConfig, persona string, scopeIssueID string) error {
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

	agentID := alias
	if agentID == "" {
		agentID = "agent-" + shortID()
	}

	// Loop-scope tracelog (IS-1033 quick wire-up): one logger for this
	// runLoop run, alias-tagged so every event from this process is
	// filterable as a chain via `dx log tail --tag alias=<X>`. claude's
	// existing `log` closure stays for human-readable stderr; emit
	// duplicates structured tags into tracelog.
	loopLog, loopSink := setupLoopTracelog(rc, "claude", agentID)
	if loopSink != nil {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = loopSink.Close(closeCtx)
		}()
	}
	if loopLog != nil {
		defer loopLog.Close()
		if u := loopLog.FilteredURL(rc.url, rc.slug); u != "" {
			fmt.Printf("View loop events: %s\n", u)
		}
		loopLog.Info("loop.started", "lease_minutes", agentCfg.LeaseMinutes, "srcless", srcless)
		defer loopLog.Info("loop.exited")
	}
	emit := func(name string, kv ...any) {
		if loopLog != nil {
			loopLog.Info(name, kv...)
		}
	}

	// Remote-control bridge (IS-1032): same wiring as RunManagedLoop. The
	// daemon's ControlCh consumer toggles holder pause/drain state which
	// the for-loop checks each iteration. Best-effort dial; failure falls
	// back to file-only operation.
	holder := agentdaemon.NewLoopTaskHolder()
	startDaemon(ctx, "claude", ProviderOpts{
		RC:       rc,
		AgentCfg: agentCfg,
		Alias:    agentID,
		WorkDir:  workDir,
	}, holder, loopLog)

	selfPath, _ := os.Executable()
	selfHash := fileHash(selfPath)

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
	tightLoop := newTightLoopDetector(5, 60*time.Second)
	for {
		if ctx.Err() != nil {
			return nil
		}

		// Self-update detection.
		if h := fileHash(selfPath); h != "" && selfHash != "" && h != selfHash {
			log("self-update: %s → %s, re-execing", shortHash(selfHash), shortHash(h))
			emit("self_update.detected", "old", shortHash(selfHash), "new", shortHash(h))
			if err := selfReexec(selfPath, os.Args); err != nil {
				log("re-exec failed: %v", err)
				emit("self_update.reexec_failed", "err", err.Error())
			}
		}

		// Pause / drain gates from the daemon's ControlCh consumer.
		if err := holder.WaitWhilePaused(ctx); err != nil {
			return nil
		}
		if holder.DrainSignaled() {
			log("drain signaled, exiting loop")
			emit("drain.exited")
			return nil
		}

		// Build TakeConfig for this iteration. iterationID is minted below
		// before take.started so both the boundary events and Take's
		// internal events (srcless, stall recovery, model.balanced) share
		// the same iteration_id tag.
		iterationID := uuid.New().String()
		takeCfg := TakeConfig{
			RC:           rc,
			AgentID:      agentID,
			Alias:        alias,
			Chrome:       chrome,
			Srcless:      srcless,
			WorkDir:      workDir,
			HomeCwd:      homeCwd,
			AgentCfg:     agentCfg,
			ModelSel:     sel,
			Persona:      persona,
			ScopeIssueID: scopeIssueID,
			SessionIdx:   sessionIdx,
			SelfPath:     selfPath,
			StateFile:    stateFile,
			Holder:       holder,
			LogFn:        log,
			LoopLog:      loopLog,
			IterationID:  iterationID,
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

		emit("take.started",
			"iteration_id", iterationID,
			"session_idx", sessionIdx,
			"resume_issue_id", takeCfg.ResumeIssueID,
			"resume_sid", takeCfg.ResumeSID)

		iterationStart := time.Now()
		result := Take(ctx, takeCfg)
		sessionIdx++

		// Defense-in-depth tight-loop guard (TK-1836 / IS-1234): a real
		// session resets the window; otherwise feed the iteration start
		// timestamp so we can detect the loop spinning faster than its
		// declared idle interval and back off exponentially, regardless
		// of which downstream code path is mis-pacing it.
		if result.Success || result.TodoKey != "" {
			tightLoop.reset()
		} else if backoff := tightLoop.record(iterationStart); backoff > 0 {
			log("tight-loop detected: %d iterations in <%s; backing off %s",
				tightLoop.capacity,
				tightLoop.span.Truncate(time.Second),
				backoff.Truncate(time.Second))
			emit("loop.tight_loop_detected",
				"iteration_id", iterationID,
				"iterations", tightLoop.capacity,
				"span_seconds", int(tightLoop.span.Seconds()),
				"backoff_seconds", int(backoff.Seconds()))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
		}

		// Handle idle (no work available).
		if errors.Is(result.Err, ErrNoWork) {
			emit("claim.idle", "iteration_id", iterationID)
			log("idle (no claimable todos); sleeping 60s")
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(60 * time.Second):
			}
			continue
		}

		emit("take.ended",
			"iteration_id", iterationID,
			"todo_key", result.TodoKey,
			"success", result.Success,
			"churn_downgraded", result.ChurnDowngraded,
			"cycle_detected", result.CycleDetected,
			"err", errString(result.Err))

		// Track churn across iterations by todo key. Both ChurnDowngraded
		// (server downgraded resolve→release because the session made no
		// mutations) and CycleDetected (queue would immediately regenerate
		// the resolved todo) signal the same operator-visible behavior:
		// this todo isn't making progress. Treat them identically for
		// backoff so the loop doesn't spin (IS-1039).
		switch {
		case result.CycleDetected || result.ChurnDowngraded:
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
			emit("churn.backoff", "key", lastChurnTodoKey, "count", consecutiveChurns, "backoff_seconds", int(backoff.Seconds()))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
		}
	}
}

// buildSessionPrompt assembles the -p prompt for a Claude session from the
// project vision, optional issue ID, and optional claimed todo. An empty
// persona falls back to DefaultPersona (dev) via NormalizePersona.
func buildSessionPrompt(vision, issueID, persona string, todo *claimedTodo) string {
	prompt := ""
	if block, err := PersonaPrompt(persona); err == nil && block != "" {
		prompt = block + "\n\n"
	}
	if vision != "" {
		prompt += "Project vision: " + vision + "\n\n"
	}
	if todo != nil {
		// Pass the todo text directly as the prompt — the work instructions
		// are already embedded in the todo by the queue generator (IS-382).
		// This must take precedence over the issueID-only fallback so that
		// kind-specific playbooks (triage, decompose, etc.) reach the agent.
		prompt += fmt.Sprintf("Claimed todo %d [%s] target=%s:%s\n\n%s",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)
		prompt += "\n\nCOMMIT RULE: Only commit intent files (migrations, queries/*.sql, handler source). Do not commit internal/db/*.sql.go, internal/dxclient/models.gen.go, ui/src/api.gen.ts, schema/shipped.sql. Use dx commit --intent."
		prompt += `

---
TEST-FAILURE PROTOCOL

Before committing, always run:

  dx test --classify-preexisting --escalate

If this detects preexisting test failures (tests already failing on the base
branch), the --escalate flag will auto-file a deduplicated blocker issue.
Do NOT attempt to fix preexisting failures — they are not your problem.
The escalation system handles filing and dedup automatically.

If the test run shows REGRESSION FAILURES (caused by your diff), you must
fix them before resolving the todo.
---
`
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
---`, todo.Key)
	} else if issueID != "" {
		prompt += fmt.Sprintf("Work on issue %s. Use ./bin/dx CLI commands (issue show, comment add, todo dev done) to interact with the project tracker.", issueID)
	}
	return prompt
}

// runSession launches a single Claude CLI session and drives its lifecycle
// through the provider-agnostic RunLifecycle runner. Event tailing, WS
// streaming, and close are all owned by the shared runner — this wrapper
// only constructs a claudeAdapter and prints the post-session token summary.
func runSession(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, prevSID string, resumed bool, model, persona string, todoID int32, todo *claimedTodo, srcless bool) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	// Fetch project vision to provide context for the session.
	vision := fetchProjectVision(rc)
	prompt := buildSessionPrompt(vision, issueID, persona, todo)

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
		issueID: issueID,
		exited:  make(chan struct{}),
	}

	tlog, tsink := setupSessionTracelog(rc, "claude", model, sid, issueID, alias)
	if tsink != nil {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tsink.Close(closeCtx)
		}()
	}
	if tlog != nil {
		defer tlog.Close()
		if u := tlog.FilteredURL(rc.url, rc.slug); u != "" {
			fmt.Printf("View live logs: %s\n", u)
		}
		tlog.Info("session.start",
			"sid", sid,
			"issue_id", issueID,
			"alias", alias,
			"model", model,
			"resumed", resumed,
			"prev_sid", prevSID)
	}

	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "claude-cli", todoID)
	if tlog != nil {
		status := "ok"
		errStr := ""
		if err != nil {
			status = "error"
			errStr = err.Error()
		}
		tlog.Info("session.end", "status", status, "err", errStr)
	}

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
func runSessionWithSummary(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, model, persona string, todoID int32, summary string, todo *claimedTodo, srcless bool) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	taskPrompt := ""
	if block, err := PersonaPrompt(persona); err == nil && block != "" {
		taskPrompt = block + "\n\n"
	}
	if todo != nil {
		taskPrompt += fmt.Sprintf("Claimed todo %d [%s] target=%s:%s\n\n%s",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)
	} else if issueID != "" {
		taskPrompt += fmt.Sprintf("Work on issue %s. Use ./bin/dx CLI commands (issue show, comment add, todo dev done) to interact with the project tracker.", issueID)
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
		issueID: issueID,
		exited:  make(chan struct{}),
	}

	tlog, tsink := setupSessionTracelog(rc, "claude", model, sid, issueID, alias)
	if tsink != nil {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tsink.Close(closeCtx)
		}()
	}
	if tlog != nil {
		defer tlog.Close()
		if u := tlog.FilteredURL(rc.url, rc.slug); u != "" {
			fmt.Printf("View live logs: %s\n", u)
		}
		tlog.Info("session.start",
			"sid", sid,
			"issue_id", issueID,
			"alias", alias,
			"model", model,
			"resumed_from_summary", true)
	}

	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "claude-cli", todoID)
	if tlog != nil {
		status := "ok"
		errStr := ""
		if err != nil {
			status = "error"
			errStr = err.Error()
		}
		tlog.Info("session.end", "status", status, "err", errStr)
	}
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
		prompt := buildSessionPrompt(vision, opts.IssueID, opts.Persona, nil)
		// Default chrome=true to match the legacy claude command's flag default.
		chrome := opts.Chrome
		return &claudeAdapter{
			rc:         opts.RC,
			agentCfg:   opts.AgentCfg,
			projDir:    projDir,
			chrome:     chrome,
			alias:      opts.Alias,
			model:      opts.Model,
			prompt:     prompt,
			srcless:    opts.Srcless,
			mcpCommand: opts.MCPCommand,
			traceID:    opts.TraceID,
			exited:     make(chan struct{}),
		}, nil
	})
}

// RunLoop implements LoopProvider — claude's loop runs the rich Take-based
// orchestration (worktree-per-session, stall recovery, churn detection,
// session-balanced model picking) rather than the universal RunManagedLoop.
// Manager dispatches here automatically when --provider=claude in loop mode.
func (a *claudeAdapter) RunLoop(_ context.Context, opts ProviderOpts) error {
	sel := modelSelector{modelFlag: opts.Model, complexity: opts.Complexity, agentCfg: opts.AgentCfg}
	return runLoop(opts.RC, opts.Alias, opts.Chrome, sel, opts.Srcless, opts.WorkDir, opts.AgentCfg, opts.Persona, opts.ScopeIssueID)
}

// claudeAdapter implements AgentAdapter against the real `claude` CLI. It
// launches the process with ZDX-aware environment vars and returns the
// transcript path that Claude writes its JSONL session to.
type claudeAdapter struct {
	rc         remoteConfig
	agentCfg   config.AgentConfig // honored by ResolveModel for the medium-tier ClaudeModel fallback
	projDir    string
	chrome     bool
	prevSID    string
	resumed    bool
	alias      string
	model      string
	prompt     string   // custom prompt; empty = "/work"
	srcless    bool     // when true, inject DX_GLOBAL=1 into the subprocess env
	issueID    string   // claimed issue ID; exported as DX_AGENT_ISSUE for escalation
	mcpCommand []string // when non-empty, claude is launched with --mcp-config dispatching tools through this argv (dev-container mode)
	traceID    string   // session trace_id; exported as ZDX_TRACE_ID and injected into docker exec env so server-side mutations correlate

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

// buildClaudeMCPConfig serializes argv into the inline JSON shape claude's
// --mcp-config flag accepts: {"mcpServers": {"<name>": {"command": ..., "args": [...]}}}.
// Returns the JSON string suitable for passing as --mcp-config <string>.
// argv must be non-empty and split argv0 (command) from argv[1:] (args).
func buildClaudeMCPConfig(argv []string) (string, error) {
	if len(argv) == 0 {
		return "", fmt.Errorf("mcp command must not be empty")
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"dx-tools": map[string]any{
				"command": argv[0],
				"args":    argv[1:],
			},
		},
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// buildClaudeEnv builds the environment passed to the spawned claude CLI.
// Extracted from Start() so the env wiring (including DX_GLOBAL in srcless
// mode) is unit-testable without spawning a subprocess.
// scopedToken replaces any DX_REMOTE_API_KEY in base; pass "" to skip injection.
// traceID, when non-empty, is exported as ZDX_TRACE_ID so dx CLI calls
// from claude (or its tools) stamp X-ZDX-Trace-Id on outbound requests.
func buildClaudeEnv(base []string, sid, alias, traceID string, srcless bool, scopedToken string, issueID string) []string {
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
	if issueID != "" {
		env = append(env, "DX_AGENT_ISSUE="+issueID)
	}
	if traceID != "" {
		env = append(env, "ZDX_TRACE_ID="+traceID)
	}
	if srcless {
		env = append(env, "DX_GLOBAL=1")
	}
	return env
}

// mcpDockerExecEnv returns the env-var map injected into the docker
// exec argv so the in-slot MCP server (and any tool subprocesses it
// spawns) inherit the agent's correlation IDs. Mirrors what
// buildClaudeEnv exports for the host claude subprocess — same keys,
// same values — so the host and slot share one correlation namespace.
func mcpDockerExecEnv(sid, alias, traceID string, srcless bool) map[string]string {
	kv := make(map[string]string, 4)
	if sid != "" {
		kv["ZDX_SESSION_ID"] = sid
	}
	if alias != "" {
		kv["ZDX_AGENT_ID"] = alias
		kv["DX_AUTHOR_ALIAS"] = alias
	}
	if traceID != "" {
		kv["ZDX_TRACE_ID"] = traceID
	}
	if srcless {
		kv["DX_GLOBAL"] = "1"
	}
	return kv
}

// injectMCPDockerExecEnv injects `-e KEY=VALUE` flags into a `docker exec
// ...` argv so processes spawned inside the slot inherit the variable.
// `docker exec` does NOT propagate host env into the container by default;
// for in-slot dx CLI calls (Bash tool → `dx ...`) to stamp X-ZDX-Trace-Id
// headers, ZDX_TRACE_ID must reach the slot via this per-exec env. Returns
// argv unchanged when it doesn't look like `docker exec`.
//
// Insertion point: between `docker exec` and the first non-flag arg
// (typically the container name). All `-e` flags must precede the
// container name.
func injectMCPDockerExecEnv(argv []string, kv map[string]string) []string {
	if len(argv) < 2 || filepath.Base(argv[0]) != "docker" || argv[1] != "exec" {
		return argv
	}
	if len(kv) == 0 {
		return argv
	}
	// Find insertion index: right after `docker exec` and any existing
	// flag tokens (e.g. `-i`, `-t`). The first non-flag arg is the
	// container name.
	insert := 2
	for insert < len(argv) && strings.HasPrefix(argv[insert], "-") {
		// `-e VAL` consumes two tokens; account for it.
		if argv[insert] == "-e" {
			insert += 2
			continue
		}
		insert++
	}
	out := make([]string, 0, len(argv)+2*len(kv))
	out = append(out, argv[:insert]...)
	for k, v := range kv {
		out = append(out, "-e", k+"="+v)
	}
	out = append(out, argv[insert:]...)
	return out
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
	// Dev-container mode: claude's built-in tools are disabled and tool calls
	// dispatch through an MCP server spawned by mcpCommand (typically
	// `docker exec -i <slot> dx-agent --mcp-stdio`). --strict-mcp-config
	// ensures global ~/.claude/.mcp.json or repo-level .mcp.json don't sneak
	// in.
	if len(a.mcpCommand) > 0 {
		// Inject ZDX correlation env into the docker exec argv so in-slot
		// dx CLI calls (Bash tool → `dx ...`) inherit them. Without this,
		// the host's claude env stops at the slot boundary and server-side
		// mutations triggered from the slot lose trace_id correlation.
		mcpArgv := injectMCPDockerExecEnv(a.mcpCommand, mcpDockerExecEnv(sid, a.alias, a.traceID, a.srcless))
		mcpJSON, err := buildClaudeMCPConfig(mcpArgv)
		if err != nil {
			return "", fmt.Errorf("build claude --mcp-config: %w", err)
		}
		cmdArgs = append(cmdArgs,
			"--mcp-config", mcpJSON,
			"--strict-mcp-config",
			"--tools", "",
		)
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
	a.proc.Env = buildClaudeEnv(os.Environ(), sid, a.alias, a.traceID, a.srcless, scopedToken, a.issueID)

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

// ── Todo claiming helpers ─────────────────────────────────────────────────

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
