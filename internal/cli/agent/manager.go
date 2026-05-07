package agent

import (
	"context"
	"fmt"
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
	"github.com/iodesystems/zdx-go/internal/cli/agent/tracelog"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

// RunManagedSession is the unified entry point for a single agent session
// across all providers. It owns the scaffolding that used to be duplicated
// across runClaudeSession / runOpenCodeSession / runLocalSession:
//
//   - mints a project-scoped admin token, sets DX_REMOTE_API_KEY for the
//     subprocess, defers revocation + env restoration
//   - builds a tracelog.Logger with trace_id, agent/session/git tags, and a
//     zdxclient sink wired to the dual-auth /api/ingest/logs endpoint
//   - prints the "View live logs:" deep-link on session start
//   - emits session.start / session.end events around RunLifecycle
//
// The provider-specific bits (which AgentAdapter to construct, what Model
// to use) are supplied via ProviderOpts. The Model field is expected to be
// already resolved (post-AgentProvider.ResolveModel); the manager doesn't
// re-resolve.
//
// providerName is the registry key (e.g. "claude") used for tracelog tags
// and to look up the constructor.
func RunManagedSession(ctx context.Context, providerName string, opts ProviderOpts) error {
	if opts.RC.slug == "" {
		return fmt.Errorf("dx agent requires a project config with a remote slug")
	}
	ctor, err := LookupProvider(providerName)
	if err != nil {
		return err
	}

	if opts.SID == "" {
		opts.SID = uuid.New().String()
	}

	token, tokenID, err := mintScopedToken(ctx, opts.RC, fmt.Sprintf("agent-%s-%s-%s", providerName, opts.Alias, opts.SID[:8]))
	if err != nil {
		return fmt.Errorf("mint scoped token: %w", err)
	}

	prev, hadKey := os.LookupEnv("DX_REMOTE_API_KEY")
	os.Setenv("DX_REMOTE_API_KEY", token)
	defer func() {
		revokeScopedToken(context.Background(), opts.RC, tokenID)
		if hadKey {
			os.Setenv("DX_REMOTE_API_KEY", prev)
		} else {
			os.Unsetenv("DX_REMOTE_API_KEY")
		}
	}()

	tlog, tsink, tlogErr := newManagedTraceLogger(opts.RC, providerName, opts.Model, opts.SID, opts.IssueID, opts.Alias, token)
	if tlogErr != nil {
		fmt.Fprintf(os.Stderr, "tracelog: %v (continuing without structured logging)\n", tlogErr)
	}
	if tsink != nil {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = tsink.Close(closeCtx)
		}()
	}
	if tlog != nil {
		defer tlog.Close()
		if u := tlog.FilteredURL(opts.RC.url, opts.RC.slug); u != "" {
			fmt.Printf("View live logs: %s\n", u)
		}
		tlog.Info("session.start",
			"sid", opts.SID,
			"issue_id", opts.IssueID,
			"alias", opts.Alias,
			"model", opts.Model)
	}

	provider, err := ctor(opts)
	if err != nil {
		return fmt.Errorf("construct %s provider: %w", providerName, err)
	}

	_, runErr := RunLifecycle(ctx, provider, opts.RC, opts.SID, opts.IssueID, opts.Alias, providerName+"-cli", 0)
	if tlog != nil {
		status := "ok"
		errStr := ""
		if runErr != nil {
			status = "error"
			errStr = runErr.Error()
		}
		tlog.Info("session.end", "status", status, "err", errStr)
	}
	return runErr
}

// DispatchSingle runs ONE managed session via the given executor. The
// driver-vs-executor split: this function is the single-session driver,
// agnostic to host vs container; the Executor decides how the workspace
// is provisioned. Bare `dx agent` (with --container=docker or
// --container=local) routes here.
//
// SIGINT/SIGTERM cancels the session ctx so RunManagedSession returns;
// deferred ws.Cleanup() then tears the workspace down. No os.Exit
// dance — single session is sequential.
func DispatchSingle(parentCtx context.Context, providerName string, opts ProviderOpts, executor Executor) error {
	if executor == nil {
		executor = HostExecutor{}
	}
	if opts.Alias == "" {
		opts.Alias = providerName + "-" + uuid.New().String()[:8]
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	ws, err := executor.Provision(ctx, opts, 0)
	if err != nil {
		return err
	}
	defer ws.Cleanup()

	installSingleSessionSignalHandler(ctx, cancel)

	if executor.Name() != "host" {
		fmt.Printf("[%s] %s mode: workspace=%s\n",
			time.Now().Format(time.RFC3339), executor.Name(), ws.Name)
	}
	return RunManagedSession(ctx, providerName, ws.Apply(opts))
}

// DispatchLoop runs N parallel claim/work loops via the given executor —
// each loop in its own workspace. Concurrency=1 (default) is one loop in
// one workspace; concurrency=N is the explicit fan-out opt-in for
// `dx agent loop`. Each loop runs on its own goroutine, alias-tagged
// `<base>-<i>` for trace correlation; cluster_id stamps the orchestrator
// alias so the cluster is filterable as one chain in the UI.
//
// The driver picks LoopProvider.RunLoop (claude's bespoke loop body) when
// the provider implements it; otherwise the universal RunManagedLoop. The
// executor concern (host worktree vs slot container) is orthogonal.
func DispatchLoop(parentCtx context.Context, providerName string, opts ProviderOpts, executor Executor) error {
	if executor == nil {
		executor = HostExecutor{}
	}

	concurrency := opts.Concurrency
	if concurrency <= 0 {
		concurrency = 1
	}

	if opts.Alias == "" {
		opts.Alias = providerName + "-" + uuid.New().String()[:8]
	}
	clusterID := opts.Alias

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	// Provision N workspaces up front. Failure rolls back all created
	// workspaces before bailing — callers see a clean state on error.
	workspaces := make([]*Workspace, 0, concurrency)
	cleanupAll := func() {
		for _, w := range workspaces {
			w.Cleanup()
		}
	}
	for i := 0; i < concurrency; i++ {
		ws, err := executor.Provision(ctx, opts, i)
		if err != nil {
			cleanupAll()
			return fmt.Errorf("provision workspace %d: %w", i, err)
		}
		workspaces = append(workspaces, ws)
	}
	defer cleanupAll()

	// Loop-mode signal handler: drain in-flight slots, fire workspace
	// cleanups before os.Exit (defer skips os.Exit). os.Exit is what the
	// pre-refactor runMCPContainerLoop used for parallel-goroutine drain;
	// preserved here for the same reason.
	installLoopSignalHandler(ctx, cancel, cleanupAll)

	if concurrency == 1 {
		return runOneLoop(ctx, providerName, workspaces[0].Apply(opts))
	}

	var wg sync.WaitGroup
	for i, ws := range workspaces {
		wg.Add(1)
		go func(slot int, ws *Workspace) {
			defer wg.Done()
			slotOpts := ws.Apply(opts)
			slotOpts.Alias = fmt.Sprintf("%s-%d", opts.Alias, slot)
			slotOpts.ClusterID = clusterID
			if err := runOneLoop(ctx, providerName, slotOpts); err != nil && ctx.Err() == nil {
				fmt.Fprintf(os.Stderr, "slot %d (%s) loop error: %v\n", slot, ws.Name, err)
			}
		}(i, ws)
	}
	wg.Wait()
	return nil
}

// runOneLoop is the per-workspace loop body. Dispatches to LoopProvider's
// RunLoop when the provider has one (claude's Take-based orchestration);
// falls back to the universal RunManagedLoop otherwise.
func runOneLoop(ctx context.Context, providerName string, opts ProviderOpts) error {
	ctor, err := LookupProvider(providerName)
	if err != nil {
		return err
	}
	provider, err := ctor(opts)
	if err != nil {
		return fmt.Errorf("construct %s provider: %w", providerName, err)
	}
	if lp, ok := provider.(LoopProvider); ok {
		return lp.RunLoop(ctx, opts)
	}
	return RunManagedLoop(ctx, providerName, opts)
}

func installSingleSessionSignalHandler(ctx context.Context, cancel context.CancelFunc) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
			signal.Stop(sigCh)
		case <-sigCh:
			cancel()
			signal.Stop(sigCh)
		}
	}()
}

func installLoopSignalHandler(ctx context.Context, cancel context.CancelFunc, cleanup func()) {
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
			signal.Stop(sigCh)
			return
		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "[%s] received signal %s: cancelling loop and cleaning up\n",
				time.Now().Format(time.RFC3339), sig)
			cancel()
			// Give in-flight goroutines a chance to drain via ctx
			// cancellation; force cleanup + os.Exit if a second signal
			// arrives or 10s elapses (defer doesn't run after os.Exit).
			select {
			case <-time.After(10 * time.Second):
			case <-sigCh:
			}
			cleanup()
			os.Exit(130)
		}
	}()
}

// RunManagedLoop atomically claims work via /api/dx/solo/claim, runs a
// managed session per pick, renews the lease while the session runs, and
// releases on completion. The shared scaffolding (signal handling, state-
// file checkpoint, idle backoff, self-update re-exec) is owned here.
//
// Concurrency: claims are FOR UPDATE SKIP LOCKED at the DB layer, so two
// agents running the same loop never see the same todo. Lease minutes come
// from opts.AgentCfg.LeaseMinutes (default applied via ResolvedAgent).
//
// Crash recovery: on startup, if a state file from a previous run exists,
// the orphaned claim is released so it returns to the queue. The next
// iteration claims fresh work.
func RunManagedLoop(parentCtx context.Context, providerName string, opts ProviderOpts) error {
	if opts.RC.slug == "" {
		return fmt.Errorf("dx agent loop requires a project config with a remote slug")
	}
	if _, err := LookupProvider(providerName); err != nil {
		return err
	}
	if opts.Alias == "" {
		opts.Alias = providerName + "-" + uuid.New().String()[:8]
	}
	leaseMin := int32(opts.AgentCfg.LeaseMinutes)
	if leaseMin <= 0 {
		leaseMin = 30
	}

	stateFile := filepath.Join(".zdx", "cache", providerName+"-agent-state")
	_ = os.MkdirAll(filepath.Join(".zdx", "cache"), 0o755)

	// Loop-level tracelog: one logger for this entire loop run. Constant
	// tags (alias, provider, slug, branch) make every event from this run
	// filterable as a chain in the UI by alias=X. Per-iteration sub-tags
	// (iteration_id, todo_id) are added per take via loopLog.With.
	loopLog, loopSink := setupLoopTracelog(opts.RC, providerName, opts.Alias)
	if loopLog != nil && opts.ClusterID != "" {
		// Container-orchestrator path: stamp cluster_id so per-slot chains
		// correlate with the orchestrator's chain in the UI.
		loopLog = loopLog.With(map[string]string{"cluster_id": opts.ClusterID})
	}
	if loopSink != nil {
		defer func() {
			closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = loopSink.Close(closeCtx)
		}()
	}
	if loopLog != nil {
		defer loopLog.Close()
		if u := loopLog.FilteredURL(opts.RC.url, opts.RC.slug); u != "" {
			fmt.Printf("View loop events: %s\n", u)
		}
		loopLog.Info("loop.started", "lease_minutes", leaseMin)
		defer loopLog.Info("loop.exited")
	}
	emit := func(name string, kv ...any) {
		if loopLog != nil {
			loopLog.Info(name, kv...)
		}
	}

	// Crash-recovery: if a previous run left a state file, the claim it
	// references is still held until the lease expires. Release it now so
	// the next claim picks up fresh work instead of duplicating effort.
	if data, err := os.ReadFile(stateFile); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 3 {
			if id, perr := parseInt32(lines[2]); perr == nil && id > 0 {
				fmt.Fprintf(os.Stderr, "[%s] crash-recovery: releasing orphaned claim %d (sid=%s)\n", providerName, id, lines[1])
				emit("crash_recovery.released_orphan", "todo_id", id, "session_id", lines[1])
				releaseTodo(opts.RC, id, opts.Alias, lines[1], "", "", "", false)
			}
		}
		_ = os.Remove(stateFile)
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	installReleaseOnSignal(opts.RC, opts.Alias, stateFile, nil, cancel)

	// Remote-control bridge (IS-1032): open a persistent WS to /api/agents/
	// connect, expose a real TaskHolder reflecting the current claim, and
	// react to server-pushed pause/resume/drain control messages. Best-
	// effort: dial failures fall back to file-only operation so a working
	// unattended loop never depends on an unavailable server.
	holder := agentdaemon.NewLoopTaskHolder()
	startDaemon(ctx, providerName, opts, holder, loopLog)

	// Self-update detection: hash the running binary at startup, re-hash each
	// iteration, and re-exec if the hash changes. Long-running loops survive
	// `make build` mid-flight without manual restart. fileHash returns ""
	// on read error (e.g. binary deleted) which disables the check rather
	// than firing a false positive.
	selfPath, _ := os.Executable()
	selfHash := fileHash(selfPath)

	// Churn tracking: when the server reports ChurnDowngraded for the same
	// todo key on consecutive iterations, the agent is thrashing — backoff
	// exponentially (1m, 2m, 4m, ... 64m cap) so we don't spin. Different
	// todo key resets the counter.
	consecutiveChurns := 0
	lastChurnTodoKey := ""

	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if h := fileHash(selfPath); h != "" && selfHash != "" && h != selfHash {
			fmt.Fprintf(os.Stderr, "[%s] self-update: %s → %s, re-execing\n", providerName, shortHash(selfHash), shortHash(h))
			emit("self_update.detected", "old", shortHash(selfHash), "new", shortHash(h))
			if err := selfReexec(selfPath, os.Args); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] re-exec failed: %v\n", providerName, err)
				emit("self_update.reexec_failed", "err", err.Error())
			}
		}

		// Pause gate: server-pushed pause holds the loop here without
		// claiming. The daemon's hold-loop keeps any in-flight claim's
		// lease alive while we're parked. WaitWhilePaused returns
		// immediately when not paused.
		if err := holder.WaitWhilePaused(ctx); err != nil {
			return err
		}
		// Drain gate: server-pushed drain (or operator-issued via
		// `dx agent drain <id>`) means "no new claims." Exit the loop
		// cleanly so the next deploy / shutdown isn't waiting on us.
		if holder.DrainSignaled() {
			fmt.Fprintf(os.Stderr, "[%s] drain signaled, exiting loop\n", providerName)
			emit("drain.exited")
			return nil
		}

		todo, err := claimNextTodo(opts.RC, opts.Alias, leaseMin)
		if err != nil || todo == nil {
			emit("claim.idle", "err", errString(err))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(60 * time.Second):
				continue
			}
		}

		issueID := todo.IssueRef
		if issueID == "" && todo.TargetType == "issue" {
			issueID = todo.TargetID
		}
		sid := uuid.New().String()
		_ = os.WriteFile(stateFile, []byte(fmt.Sprintf("%s\n%s\n%d\n", issueID, sid, todo.ID)), 0o644)

		iterationID := uuid.New().String()
		emit("claim.acquired",
			"iteration_id", iterationID,
			"todo_id", todo.ID,
			"todo_key", todo.Key,
			"todo_kind", todo.Kind,
			"issue_id", issueID,
			"target_type", todo.TargetType,
			"target_id", todo.TargetID,
			"session_id", sid)

		// Hand the daemon a real TaskHolder snapshot for the duration of
		// this session. The daemon's pause hold-loop calls holder.Renew
		// while the operator has the agent paused; the renewer here is
		// the same closure as the in-loop heartbeat.
		renewClosure := func() { renewTodoLease(opts.RC, todo.ID, opts.Alias, leaseMin) }
		holder.Set(agentdaemon.RunningTask{
			ID:        fmt.Sprintf("%d", todo.ID),
			SessionID: sid,
			IssueID:   issueID,
			Started:   time.Now(),
		}, renewClosure)

		// Lease renewal: heartbeat every leaseMin/2 (min 1 minute) until the
		// session finishes or the loop context is cancelled.
		renewCtx, stopRenew := context.WithCancel(ctx)
		go func() {
			interval := time.Duration(leaseMin/2) * time.Minute
			if interval < time.Minute {
				interval = time.Minute
			}
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-renewCtx.Done():
					return
				case <-ticker.C:
					renewClosure()
				}
			}
		}()

		seed := fmt.Sprintf("Claimed todo %d [%s] target=%s:%s\n\n%s\n\nWork this vertical: resolve the items for the referenced issue, then close it. Use dx tools for project ops and filesystem/shell tools for code changes. Stop when the issue is closed or blocked.",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)

		sessOpts := opts
		sessOpts.SID = sid
		sessOpts.IssueID = issueID
		sessOpts.SeedPrompt = seed
		runErr := RunManagedSession(ctx, providerName, sessOpts)
		stopRenew()
		holder.Clear()

		// Resolve the todo on success; release (without resolving) on error
		// so the queue can re-evaluate it. Branch contract validation runs
		// inside releaseTodo and may downgrade resolve→release on its own.
		release := releaseTodo(opts.RC, todo.ID, opts.Alias, sid, todo.ClaimBaseSha, todo.Kind, todo.ClaimBaseBranch, runErr == nil)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "[%s] session error: %v\n", providerName, runErr)
		}
		emit("claim.released",
			"iteration_id", iterationID,
			"todo_id", todo.ID,
			"todo_key", todo.Key,
			"resolve", runErr == nil,
			"churn_downgraded", release.ChurnDowngraded,
			"cycle_detected", release.CycleDetected,
			"err", errString(runErr))
		_ = os.Remove(stateFile)

		// Update churn tracking, then maybe back off before the next
		// claim. ChurnDowngraded (server downgraded resolve→release
		// because the session made no mutations) and CycleDetected
		// (queue would immediately regenerate a resolved todo) are
		// the same signal — this todo isn't making progress — so
		// they share one backoff path. IS-1039: previously CycleDetected
		// reset the counter, which combined with a fast-failing session
		// meant the loop spun on the same todo at ~3 takes/sec.
		switch {
		case release.CycleDetected || release.ChurnDowngraded:
			if todo.Key == lastChurnTodoKey {
				consecutiveChurns++
			} else {
				consecutiveChurns = 1
				lastChurnTodoKey = todo.Key
			}
		default:
			consecutiveChurns = 0
			lastChurnTodoKey = ""
		}
		if consecutiveChurns >= 3 {
			shift := consecutiveChurns - 3
			if shift > 6 {
				shift = 6
			}
			backoff := time.Duration(1<<shift) * time.Minute
			fmt.Fprintf(os.Stderr, "[%s] churn backoff: key %s churned %d times; sleeping %s\n",
				providerName, lastChurnTodoKey, consecutiveChurns, backoff.Truncate(time.Second))
			emit("churn.backoff", "key", lastChurnTodoKey, "count", consecutiveChurns, "backoff_seconds", int(backoff.Seconds()))
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
}

// parseInt32 parses a base-10 int32 from s. Helper for state-file recovery
// where the todo ID is the third line.
func parseInt32(s string) (int32, error) {
	var n int32
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

// errString returns err.Error() or "" when err is nil. Convenience for
// emit("name", "err", errString(err)) so the tag is always present.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// setupSessionTracelog is the small-touch helper for callers that have
// already minted their own auth and just need a tracelog logger + sink for
// a session. Used by claude's runSession (which mints its own scoped token
// inside the adapter, so the parent process uses rc.key). Returns nil
// values silently when rc is missing fields — callers should null-check.
//
// For the unified manager path, prefer newManagedTraceLogger which threads
// the freshly-minted scoped token through.
func setupSessionTracelog(rc remoteConfig, providerName, model, sid, issueID, alias string) (*tracelog.Logger, *zdxclient.Client) {
	logger, client, err := newManagedTraceLogger(rc, providerName, model, sid, issueID, alias, rc.key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "tracelog: %v (continuing without structured logging)\n", err)
		return nil, nil
	}
	return logger, client
}

// setupLoopTracelog builds the loop-scoped logger used by RunManagedLoop.
// Constant tags (alias, provider, slug, branch, worktree, pid) make it
// possible to filter every event from one loop run with a single
// alias=X filter in the UI. Per-iteration sub-tags (todo_id, iteration_id)
// are added by callers via loopLog.With.
func setupLoopTracelog(rc remoteConfig, providerName, alias string) (*tracelog.Logger, *zdxclient.Client) {
	branch, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	worktree := "main"
	if out, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err == nil {
		if strings.Contains(string(out), "/worktrees/") {
			worktree = "worktree"
		}
	}
	tags := map[string]string{
		"agent":    providerName,
		"alias":    alias,
		"slug":     rc.slug,
		"branch":   strings.TrimSpace(string(branch)),
		"worktree": worktree,
		"pid":      fmt.Sprintf("%d", os.Getpid()),
		"scope":    "loop",
	}
	var sink tracelog.Sink
	var client *zdxclient.Client
	if rc.url != "" && rc.key != "" {
		c, err := zdxclient.New(zdxclient.Config{
			Endpoint:    rc.url,
			Token:       rc.key,
			AuthMode:    zdxclient.AuthApiKey,
			ProjectSlug: rc.slug,
			Component:   "agent-" + providerName,
			OnError: func(err error, n int) {
				fmt.Fprintf(os.Stderr, "tracelog ingest: %v (%d events)\n", err, n)
			},
		})
		if err == nil {
			client = c
			sink = c
		}
	}
	logger, err := tracelog.New(tracelog.Options{
		BaseTags:  tags,
		FilePath:  filepath.Join(".zdx", "logs", providerName+"-loop.jsonl"),
		Sink:      sink,
		Component: "agent-" + providerName,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "tracelog (loop): %v (continuing without loop event chain)\n", err)
		return nil, client
	}
	return logger, client
}

// newManagedTraceLogger builds the tracelog logger + zdxclient sink for a
// managed session. Mirrors what newOpenCodeTraceLogger did inline; lifted
// here so all providers share the wiring.
func newManagedTraceLogger(rc remoteConfig, providerName, model, sid, issueID, alias, scopedToken string) (*tracelog.Logger, *zdxclient.Client, error) {
	branch, _ := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	worktree := "main"
	if out, err := exec.Command("git", "rev-parse", "--git-dir").Output(); err == nil {
		if strings.Contains(string(out), "/worktrees/") {
			worktree = "worktree"
		}
	}
	tags := map[string]string{
		"agent":      providerName,
		"session_id": sid,
		"issue_id":   issueID,
		"alias":      alias,
		"slug":       rc.slug,
		"model":      model,
		"branch":     strings.TrimSpace(string(branch)),
		"worktree":   worktree,
		"pid":        fmt.Sprintf("%d", os.Getpid()),
	}

	var sink tracelog.Sink
	var client *zdxclient.Client
	if rc.url != "" && scopedToken != "" {
		c, err := zdxclient.New(zdxclient.Config{
			Endpoint:    rc.url,
			Token:       scopedToken,
			AuthMode:    zdxclient.AuthApiKey,
			ProjectSlug: rc.slug,
			Component:   "agent-" + providerName,
			OnError: func(err error, n int) {
				fmt.Fprintf(os.Stderr, "tracelog ingest: %v (%d events)\n", err, n)
			},
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "tracelog: zdxclient init: %v (continuing file-only)\n", err)
		} else {
			client = c
			sink = c
		}
	}

	logger, err := tracelog.New(tracelog.Options{
		BaseTags:  tags,
		FilePath:  filepath.Join(".zdx", "logs", providerName+"-"+sid[:8]+".jsonl"),
		Sink:      sink,
		Component: "agent-" + providerName,
	})
	if err != nil {
		if client != nil {
			closeCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = client.Close(closeCtx)
			cancel()
		}
		return nil, nil, err
	}
	return logger, client, nil
}

// startDaemon opens a persistent WebSocket to /api/agents/connect for the
// managed loop and spawns a goroutine that translates server→agent control
// messages into LoopTaskHolder state changes. Best-effort: dial failures
// fall back to file-only operation so a working unattended loop never
// depends on an unavailable server. ctx cancellation closes the daemon.
//
// loopLog, when non-nil, receives daemon lifecycle events
// (daemon.connected, daemon.disconnected, daemon.control) so the loop's
// trace surface is one event chain.
func startDaemon(ctx context.Context, providerName string, opts ProviderOpts, holder *agentdaemon.LoopTaskHolder, loopLog *tracelog.Logger) {
	if opts.RC.url == "" || opts.RC.key == "" {
		return // no server to connect to
	}
	hostname, _ := os.Hostname()
	ctrlCh := make(chan agentdaemon.ControlMsg, 16)

	emit := func(name string, kv ...any) {
		if loopLog != nil {
			loopLog.Info(name, kv...)
		}
	}
	// Project slug: empty for global-pool agents (registered without a
	// project binding); otherwise the configured project's slug so the
	// server attaches the agent record to the right project.
	projectSlug := opts.RC.slug
	if opts.Global {
		projectSlug = ""
	}
	d := &agentdaemon.Daemon{
		ServerURL:    opts.RC.url,
		AgentID:      opts.Alias,
		APIKey:       opts.RC.key,
		WorktreePath: opts.WorkDir, // empty in non-srcless mode → server treats as cwd
		Hostname:     hostname,
		Pid:          int32(os.Getpid()),
		Capabilities: []string{providerName},
		Holder:       holder,
		PauseRenewer: holder, // LoopTaskHolder satisfies LeaseRenewer
		ControlCh:    ctrlCh,
		EventLog:     emit,
		ProjectSlug:  projectSlug,
		Idle:         opts.Idle,
	}

	go func() {
		// RunForever retries with backoff so a server bounce doesn't kill
		// the loop's ability to react to control commands when the server
		// comes back. Returns nil on ctx cancellation.
		if err := d.RunForever(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "[%s] agent daemon: %v (continuing without remote control)\n", providerName, err)
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ctrlCh:
				switch msg.Type {
				case "pause":
					holder.SetPaused(true)
					fmt.Fprintf(os.Stderr, "[%s] paused (session=%s issue=%s)\n", providerName, msg.SessionID, msg.IssueID)
				case "resume":
					holder.SetPaused(false)
					fmt.Fprintf(os.Stderr, "[%s] resumed\n", providerName)
				case "drain":
					holder.SignalDrain()
					// Drain also unblocks any active pause so the loop
					// reaches its next-iteration check and exits.
					holder.SetPaused(false)
				}
			}
		}
	}()
}
