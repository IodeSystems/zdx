package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

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

// DispatchLoop is the entry point for any loop-mode dispatch. If the named
// provider implements LoopProvider, its RunLoop owns the run (claude's
// Take-based orchestration). Otherwise the universal RunManagedLoop runs.
// dx agent loop and any future loop-driving code path through this function
// rather than picking a runtime themselves.
func DispatchLoop(ctx context.Context, providerName string, opts ProviderOpts) error {
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

// DispatchContainerLoop is the entry point for --container dispatch. Errors
// when the named provider doesn't implement ContainerProvider (only claude
// today). Bypasses the standard loop entirely — the per-slot containers
// each run their own `dx agent loop --provider=...` internally.
func DispatchContainerLoop(ctx context.Context, providerName string, opts ProviderOpts) error {
	ctor, err := LookupProvider(providerName)
	if err != nil {
		return err
	}
	provider, err := ctor(opts)
	if err != nil {
		return fmt.Errorf("construct %s provider: %w", providerName, err)
	}
	cp, ok := provider.(ContainerProvider)
	if !ok {
		return fmt.Errorf("--provider=%s does not support --container (only claude today)", providerName)
	}
	return cp.RunContainerLoop(ctx, opts)
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

	// Crash-recovery: if a previous run left a state file, the claim it
	// references is still held until the lease expires. Release it now so
	// the next claim picks up fresh work instead of duplicating effort.
	if data, err := os.ReadFile(stateFile); err == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if len(lines) >= 3 {
			if id, perr := parseInt32(lines[2]); perr == nil && id > 0 {
				fmt.Fprintf(os.Stderr, "[%s] crash-recovery: releasing orphaned claim %d (sid=%s)\n", providerName, id, lines[1])
				releaseTodo(opts.RC, id, opts.Alias, lines[1], "", "", "", false)
			}
		}
		_ = os.Remove(stateFile)
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	installReleaseOnSignal(opts.RC, opts.Alias, stateFile, nil, cancel)

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
			if err := selfReexec(selfPath, os.Args); err != nil {
				fmt.Fprintf(os.Stderr, "[%s] re-exec failed: %v\n", providerName, err)
			}
		}

		todo, err := claimNextTodo(opts.RC, opts.Alias, leaseMin)
		if err != nil || todo == nil {
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

		// Lease renewal: heartbeat every leaseMin/2 (min 1 minute) until the
		// session finishes or the loop context is cancelled.
		renewCtx, stopRenew := context.WithCancel(ctx)
		go func(todoID int32) {
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
					renewTodoLease(opts.RC, todoID, opts.Alias, leaseMin)
				}
			}
		}(todo.ID)

		seed := fmt.Sprintf("Claimed todo %d [%s] target=%s:%s\n\n%s\n\nWork this vertical: resolve the items for the referenced issue, then close it. Use dx tools for project ops and filesystem/shell tools for code changes. Stop when the issue is closed or blocked.",
			todo.ID, todo.Kind, todo.TargetType, todo.TargetID, todo.Text)

		sessOpts := opts
		sessOpts.SID = sid
		sessOpts.IssueID = issueID
		sessOpts.SeedPrompt = seed
		runErr := RunManagedSession(ctx, providerName, sessOpts)
		stopRenew()

		// Resolve the todo on success; release (without resolving) on error
		// so the queue can re-evaluate it. Branch contract validation runs
		// inside releaseTodo and may downgrade resolve→release on its own.
		release := releaseTodo(opts.RC, todo.ID, opts.Alias, sid, todo.ClaimBaseSha, todo.Kind, todo.ClaimBaseBranch, runErr == nil)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "[%s] session error: %v\n", providerName, runErr)
		}
		_ = os.Remove(stateFile)

		// Update churn tracking, then maybe back off before the next claim.
		switch {
		case release.CycleDetected:
			// Server auto-blocked the underlying issue; no point in
			// retrying immediately. Reset churn state.
			consecutiveChurns = 0
			lastChurnTodoKey = ""
		case release.ChurnDowngraded:
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
