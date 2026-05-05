package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/config"
)

// ErrNoWork is returned by Take when no claimable todo is available.
var ErrNoWork = errors.New("no claimable work")

// TakeConfig holds everything Take needs to execute one work item.
type TakeConfig struct {
	RC       remoteConfig
	AgentID  string
	Alias    string
	Chrome   bool
	Srcless  bool
	WorkDir  string // srcless work directory
	HomeCwd  string // original cwd to restore after srcless
	AgentCfg config.AgentConfig
	ModelSel modelSelector
	// SessionIdx is the model-rotation counter; caller increments after each Take.
	SessionIdx int
	SelfPath   string // for ensureProjectInit (srcless)

	// Resume fields: if ResumeIssueID is set, Take skips claiming and resumes
	// this specific session instead.
	ResumeIssueID string
	ResumeSID     string

	// StateFile path for crash-recovery breadcrumb.
	StateFile string

	// Holder, if non-nil, receives Set/Clear around the active session so
	// the daemon's pause hold-loop can read the live claim and the
	// LeaseRenewer can renew it. Nil-safe: skipped when the caller hasn't
	// wired a daemon (e.g. unit tests, future no-network mode).
	Holder *agentdaemon.LoopTaskHolder

	LogFn func(string, ...any)
}

// TakeResult is what Take returns to the supervisor loop.
type TakeResult struct {
	TodoKey         string
	Success         bool
	CycleDetected   bool
	ChurnDowngraded bool
	Err             error
}

// Take executes the full lifecycle for a single work item: claim → setup →
// run session → release → verify → cleanup. The caller (loop supervisor)
// handles cross-iteration concerns like churn tracking and backoff.
func Take(ctx context.Context, cfg TakeConfig) TakeResult {
	log := cfg.LogFn
	if log == nil {
		log = func(string, ...any) {}
	}

	var issueID, sid string
	var activeTodo *claimedTodo
	resumed := false

	if cfg.ResumeIssueID != "" {
		// Resume mode: skip claiming, use the provided issue/session.
		issueID = cfg.ResumeIssueID
		sid = cfg.ResumeSID
		resumed = true
		log("resuming interrupted session: issue=%s sid=%s", issueID, sid)
	} else {
		// Claim the next available todo via the API.
		todo, err := claimNextTodo(cfg.RC, cfg.AgentID, int32(cfg.AgentCfg.LeaseMinutes))
		if err != nil || todo == nil {
			return TakeResult{Err: ErrNoWork}
		}
		activeTodo = todo
		log("claimed todo %d [%s]: %s", todo.ID, todo.Kind, todo.Text)

		issueID = todo.IssueRef
		if issueID == "" && todo.TargetType == "issue" {
			issueID = todo.TargetID
		}
		sid = uuid.New().String()
	}

	if ctx.Err() != nil {
		if activeTodo != nil {
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
		}
		return TakeResult{Err: ctx.Err()}
	}

	// Save state for crash recovery.
	os.WriteFile(cfg.StateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)

	prevSID := ""
	if resumed {
		prevSID = sid
		sid = uuid.New().String()
		os.WriteFile(cfg.StateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)
		log("forking session: %s → %s", prevSID, sid)
	}

	// ── Srcless: clone + worktree + chdir ──────────────────────────────
	var srclessProjectPath, srclessWorktreePath, srclessBranch string
	if cfg.Srcless && activeTodo != nil && activeTodo.ProjectSlug != "" {
		pp, err := ensureProjectClone(cfg.WorkDir, activeTodo.ProjectSlug, cfg.RC.url)
		if err != nil {
			log("srcless: clone %s failed: %v", activeTodo.ProjectSlug, err)
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless clone: %w", err)}
		}
		srclessProjectPath = pp
		if ierr := ensureProjectInit(pp, activeTodo.ProjectSlug, cfg.RC.url, cfg.RC.key, cfg.SelfPath); ierr != nil {
			log("srcless: init %s failed: %v", activeTodo.ProjectSlug, ierr)
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless init: %w", ierr)}
		}
		wt, br, err := createSessionWorktree(pp, cfg.WorkDir, activeTodo.ProjectSlug, sid, activeTodo.TargetBranch)
		if err != nil {
			log("srcless: worktree for %s failed: %v", activeTodo.ProjectSlug, err)
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless worktree: %w", err)}
		}
		srclessWorktreePath = wt
		srclessBranch = br
		if err := os.Chdir(wt); err != nil {
			log("srcless: chdir %s failed: %v", wt, err)
			_ = removeSessionWorktree(pp, wt, br)
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless chdir: %w", err)}
		}
		log("srcless: project=%s worktree=%s branch=%s", activeTodo.ProjectSlug, wt, br)
	}

	// Deferred cleanup for srcless worktree and cwd restoration.
	defer func() {
		if srclessWorktreePath != "" {
			if rerr := removeSessionWorktree(srclessProjectPath, srclessWorktreePath, srclessBranch); rerr != nil {
				log("srcless: worktree teardown: %v", rerr)
			}
			if cfg.HomeCwd != "" {
				_ = os.Chdir(cfg.HomeCwd)
			}
		}
	}()

	log("──────────────────────────────────────────────")
	log("SESSION START  session=%s  issue=%s  resumed=%v", sid, issueID, resumed)
	log("──────────────────────────────────────────────")
	startTime := time.Now()

	// ── Daemon holder + lease renewal ─────────────────────────────────
	// Hand the daemon a live snapshot of the claim. Renewal happens in
	// two places that share the same closure: this Take's internal
	// ticker goroutine (every leaseMin/2) and — while the operator has
	// the agent paused — the daemon's pause hold-loop, which calls
	// holder.Renew to keep the claim alive without spawning new turns.
	var renewClosure func()
	if activeTodo != nil {
		renewMin := int32(cfg.AgentCfg.LeaseMinutes)
		todoID := activeTodo.ID
		renewClosure = func() { renewTodoLease(cfg.RC, todoID, cfg.AgentID, renewMin) }
		if cfg.Holder != nil {
			cfg.Holder.Set(agentdaemon.RunningTask{
				ID:        fmt.Sprintf("%d", todoID),
				SessionID: sid,
				IssueID:   issueID,
				Started:   time.Now(),
			}, renewClosure)
		}
	}
	defer func() {
		if cfg.Holder != nil {
			cfg.Holder.Clear()
		}
	}()

	var leaseCancel context.CancelFunc
	if activeTodo != nil {
		var leaseCtx context.Context
		leaseCtx, leaseCancel = context.WithCancel(ctx)
		go func(renewMin int32) {
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
					if renewClosure != nil {
						renewClosure()
					}
				}
			}
		}(int32(cfg.AgentCfg.LeaseMinutes))
	}
	defer func() {
		if leaseCancel != nil {
			leaseCancel()
		}
	}()

	// ── Session execution ──────────────────────────────────────────────
	resolvedModel := cfg.ModelSel.resolve(cfg.RC, cfg.SessionIdx)
	if resolvedModel != "" {
		log("model: %s", resolvedModel)
	}
	var todoID int32
	if activeTodo != nil {
		todoID = activeTodo.ID
	}
	sessionErr := runSession(ctx, cfg.RC, sid, issueID, cfg.Alias, cfg.Chrome, prevSID, resumed, resolvedModel, todoID, activeTodo, cfg.Srcless)

	// ── Stall recovery ─────────────────────────────────────────────────
	if errors.Is(sessionErr, ErrSessionStalled) && ctx.Err() == nil {
		stalledSID := sid
		log("session stalled, attempting resume...")

		resumeSID := uuid.New().String()
		os.WriteFile(cfg.StateFile, []byte(issueID+"\n"+resumeSID+"\n"), 0o644)
		log("forking stalled session: %s → %s", stalledSID, resumeSID)

		resumeStart := time.Now()
		resumeErr := runSession(ctx, cfg.RC, resumeSID, issueID, cfg.Alias, cfg.Chrome, stalledSID, true, resolvedModel, todoID, activeTodo, cfg.Srcless)

		if resumeErr != nil && time.Since(resumeStart) < 60*time.Second {
			log("resume failed quickly (%v), starting fresh session with transcript summary", resumeErr)

			projDir := claudeProjectDir()
			summary := SummarizeTranscript(
				filepath.Join(projDir, stalledSID+".jsonl"),
				filepath.Join(projDir, stalledSID, "subagents"),
				30, 40,
			)

			freshSID := uuid.New().String()
			os.WriteFile(cfg.StateFile, []byte(issueID+"\n"+freshSID+"\n"), 0o644)
			log("fresh session with summary: %s (issue=%s)", freshSID, issueID)

			sessionErr = runSessionWithSummary(ctx, cfg.RC, freshSID, issueID, cfg.Alias, cfg.Chrome, resolvedModel, todoID, summary, activeTodo, cfg.Srcless)
			sid = freshSID
		} else {
			sessionErr = resumeErr
			sid = resumeSID
		}
	}

	if sessionErr != nil {
		log("session error: %v", sessionErr)
	}

	elapsed := time.Since(startTime)
	log("──────────────────────────────────────────────")
	log("SESSION END  session=%s  duration=%s", sid, elapsed.Truncate(time.Second))
	log("──────────────────────────────────────────────")

	// ── Fallback incomplete-report ─────────────────────────────────────
	// If the session failed without the agent filing a done/incomplete signal,
	// emit a fallback report so the tracker always has signal on error paths.
	if activeTodo != nil && sessionErr != nil {
		slug := activeTodo.ProjectSlug
		if slug == "" {
			slug = cfg.RC.slug
		}
		emitFallbackIncompleteReport(cfg.RC, slug, activeTodo.Key, cfg.AgentID, log)
	}

	// ── Release / resolve ──────────────────────────────────────────────
	result := TakeResult{Success: sessionErr == nil}
	if activeTodo != nil {
		result.TodoKey = activeTodo.Key
		releaseRes := releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, result.Success)
		result.CycleDetected = releaseRes.CycleDetected
		result.ChurnDowngraded = releaseRes.ChurnDowngraded

		switch {
		case !result.Success:
			log("todo %d released (session failed)", activeTodo.ID)
		case result.CycleDetected:
			log("todo %d [%s] CYCLE DETECTED: resolved but would regenerate immediately — auto-blocked by server", activeTodo.ID, activeTodo.Key)
		case result.ChurnDowngraded:
			log("todo %d [%s] released (churn — no mutations detected)", activeTodo.ID, activeTodo.Key)
		default:
			log("todo %d resolved", activeTodo.ID)
		}
	}

	os.Remove(cfg.StateFile)

	// ── Srcless: push session branch on success ────────────────────────
	if srclessWorktreePath != "" && sessionErr == nil {
		targetBranch := ""
		if activeTodo != nil {
			targetBranch = activeTodo.TargetBranch
		}
		skipped, perr := pushSessionBranch(srclessWorktreePath, srclessBranch, targetBranch)
		switch {
		case perr != nil:
			log("srcless: push %s failed: %v", srclessBranch, perr)
		case skipped:
			log("srcless: %s had no commits to push", srclessBranch)
		default:
			log("srcless: pushed %s", srclessBranch)
		}
	}

	if sessionErr != nil {
		result.Err = sessionErr
	}
	return result
}
