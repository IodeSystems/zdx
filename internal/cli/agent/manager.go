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
