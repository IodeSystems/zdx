package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/agent/tracelog"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// wipSyncMu serializes wip→dev fast-forwards across slots in the same
// orchestrator. Two slots resolving simultaneously would race on the dev
// ref otherwise — git's update-ref is atomic per-call but the
// "rebase, then ff" sequence isn't. With the mutex, slot B always sees
// slot A's just-merged work and rebases its own branch onto it before
// advancing dev.
var wipSyncMu sync.Mutex

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// ErrNoWork is returned by Take when no claimable todo is available.
var ErrNoWork = errors.New("no claimable work")

// ErrScopeExhausted is returned by Take when a scoped claim
// (--scope-issue) reports a terminal state (the scope issue is closed, or
// every descendant is closed/blocked so the tracker is closable). Distinct
// from ErrNoWork because the supervisor loop should exit cleanly rather
// than continue polling — no amount of waiting will produce work in the
// subtree (TK-1835 / IS-1234).
var ErrScopeExhausted = errors.New("scope exhausted")

// ScopeExhaustedError carries the why-exhausted context (final scope state
// + reason) so the supervisor loop can log a meaningful exit line.
// errors.Is(err, ErrScopeExhausted) returns true.
type ScopeExhaustedError struct {
	IssueID string
	State   string
	Reason  string
}

func (e *ScopeExhaustedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("scope %s exhausted: %s (%s)", e.IssueID, e.State, e.Reason)
	}
	return fmt.Sprintf("scope %s exhausted: %s", e.IssueID, e.State)
}

func (e *ScopeExhaustedError) Is(target error) bool {
	return target == ErrScopeExhausted
}

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
	Persona  string // role tier (NormalizePersona-d); empty defaults to "dev"
	// ScopeIssueID, when non-empty, restricts the claim to todos whose
	// issue_ref is the issue OR a descendant of it (server-side descendant
	// walk per TK-1754). Wired from the loop's --scope-issue flag.
	ScopeIssueID string
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

	// LoopLog, when non-nil, receives structured events at Take's internal
	// boundaries (srcless clone/worktree, stall recovery, model selection)
	// so the loop's tracelog chain stays unbroken across a take. Nil-safe:
	// callers without tracelog wiring (resume runner, unit tests) get the
	// existing LogFn-only behavior.
	LoopLog *tracelog.Logger

	// IterationID correlates events emitted from within Take with the
	// take.started/take.ended boundary events the loop supervisor already
	// emits. Empty when LoopLog is nil; callers that wire LoopLog should
	// also pass the iteration_id they used for take.started.
	IterationID string
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
	emit := func(name string, kv ...any) {
		if cfg.LoopLog == nil {
			return
		}
		if cfg.IterationID != "" {
			kv = append([]any{"iteration_id", cfg.IterationID}, kv...)
		}
		cfg.LoopLog.Info(name, kv...)
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
		todo, scope, err := claimNextTodo(cfg.RC, cfg.AgentID, cfg.Persona, int32(cfg.AgentCfg.LeaseMinutes), cfg.ScopeIssueID)
		if err != nil || todo == nil {
			// Distinguish scope-exhausted (terminal: caller should exit
			// the loop) from transient idle (caller should sleep + retry).
			// Server signals exhaustion via scope.state=closed or
			// scope.state=stalled,reason=tracker-closable. Anything else
			// — including transient "all-blocked" stalls or transport
			// errors — falls through to ErrNoWork.
			if scope.terminal() {
				emit("claim.scope_exhausted",
					"scope_issue_id", scope.IssueID,
					"state", scope.State,
					"reason", scope.Reason)
				return TakeResult{Err: &ScopeExhaustedError{
					IssueID: scope.IssueID,
					State:   scope.State,
					Reason:  scope.Reason,
				}}
			}
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
			emit("srcless.clone_failed", "project_slug", activeTodo.ProjectSlug, "err", err.Error())
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless clone: %w", err)}
		}
		srclessProjectPath = pp
		emit("srcless.cloned", "project_slug", activeTodo.ProjectSlug, "path", pp)
		if ierr := ensureProjectInit(pp, activeTodo.ProjectSlug, cfg.RC.url, cfg.RC.key, cfg.SelfPath); ierr != nil {
			log("srcless: init %s failed: %v", activeTodo.ProjectSlug, ierr)
			emit("srcless.init_failed", "project_slug", activeTodo.ProjectSlug, "err", ierr.Error())
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless init: %w", ierr)}
		}
		wt, br, err := createSessionWorktree(pp, cfg.WorkDir, activeTodo.ProjectSlug, sid, activeTodo.TargetBranch)
		if err != nil {
			log("srcless: worktree for %s failed: %v", activeTodo.ProjectSlug, err)
			emit("srcless.worktree_failed", "project_slug", activeTodo.ProjectSlug, "err", err.Error())
			releaseTodo(cfg.RC, activeTodo.ID, cfg.AgentID, sid, activeTodo.ClaimBaseSha, activeTodo.Kind, activeTodo.ClaimBaseBranch, false)
			os.Remove(cfg.StateFile)
			return TakeResult{Err: fmt.Errorf("srcless worktree: %w", err)}
		}
		srclessWorktreePath = wt
		srclessBranch = br
		emit("srcless.worktree_created", "path", wt, "branch", br)
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
				emit("srcless.worktree_remove_failed", "path", srclessWorktreePath, "branch", srclessBranch, "err", rerr.Error())
			} else {
				emit("srcless.worktree_removed", "path", srclessWorktreePath, "branch", srclessBranch)
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
	// Emit model.balanced only when the selector took the alternation
	// path (no --model, no --complexity). Other paths are deterministic
	// from CLI flags, so the resolved name is recoverable from the
	// session.start tags already.
	if resolvedModel != "" && cfg.ModelSel.modelFlag == "" && cfg.ModelSel.complexity == "" {
		emit("model.balanced", "idx", cfg.SessionIdx, "picked", resolvedModel)
	}
	var todoID int32
	if activeTodo != nil {
		todoID = activeTodo.ID
	}
	sessionErr := runSession(ctx, cfg.RC, sid, issueID, cfg.Alias, cfg.Chrome, prevSID, resumed, resolvedModel, cfg.Persona, todoID, activeTodo, cfg.Srcless)

	// ── Stall recovery ─────────────────────────────────────────────────
	if errors.Is(sessionErr, ErrSessionStalled) && ctx.Err() == nil {
		stalledSID := sid
		log("session stalled, attempting resume...")
		emit("stall_recovery.attempted", "stalled_sid", stalledSID, "issue_id", issueID)

		resumeSID := uuid.New().String()
		os.WriteFile(cfg.StateFile, []byte(issueID+"\n"+resumeSID+"\n"), 0o644)
		log("forking stalled session: %s → %s", stalledSID, resumeSID)

		resumeStart := time.Now()
		resumeErr := runSession(ctx, cfg.RC, resumeSID, issueID, cfg.Alias, cfg.Chrome, stalledSID, true, resolvedModel, cfg.Persona, todoID, activeTodo, cfg.Srcless)

		if resumeErr != nil && time.Since(resumeStart) < 60*time.Second {
			log("resume failed quickly (%v), starting fresh session with transcript summary", resumeErr)

			projDir := claudeProjectDir()
			transcriptPath := filepath.Join(projDir, stalledSID+".jsonl")
			summary := SummarizeTranscript(
				transcriptPath,
				filepath.Join(projDir, stalledSID, "subagents"),
				30, 40,
			)

			freshSID := uuid.New().String()
			os.WriteFile(cfg.StateFile, []byte(issueID+"\n"+freshSID+"\n"), 0o644)
			log("fresh session with summary: %s (issue=%s)", freshSID, issueID)
			emit("stall_recovery.summarized",
				"stalled_sid", stalledSID,
				"fresh_sid", freshSID,
				"transcript_path", transcriptPath,
				"summary_lines", countLines(summary))

			sessionErr = runSessionWithSummary(ctx, cfg.RC, freshSID, issueID, cfg.Alias, cfg.Chrome, resolvedModel, cfg.Persona, todoID, summary, activeTodo, cfg.Srcless)
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

	// ── Post-session escalation: block on filed test-fix issues ───────
	// If the agent ran dx test --escalate (or it was run automatically),
	// check for escalation results and block the claimed issue on any
	// filed test-fix issues.
	if activeTodo != nil && issueID != "" {
		processPostSessionEscalation(ctx, cfg.RC, activeTodo, issueID, sid, log)
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
			// Container slots commit their work to wip/<alias> in an
			// isolated worktree. After a successful resolve we fast-forward
			// dev to absorb those commits so subsequent claims see the
			// up-to-date code and the operator doesn't have to integrate
			// by hand. No-op when there's no wip branch (host mode) or
			// when there are no new commits.
			if synced, syncErr := syncSlotWipToDev(cfg.AgentID, log); syncErr != nil {
				log("sync wip→dev: %v", syncErr)
			} else if synced != "" {
				log("sync: dev advanced to %s (%s)", synced, "wip/"+cfg.AgentID)
			}
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

// syncSlotWipToDev fast-forwards dev to absorb commits from the slot's
// wip/<agentID> branch, without disturbing the operator's working tree.
//
// Returns the new dev tip SHA (truncated) on success, or "" if there was
// nothing to sync (no wip branch, no new commits). Errors are surfaced;
// rebase conflicts are aborted cleanly and reported so the operator can
// integrate manually.
//
// Concurrency: serialized via package-level wipSyncMu so two slots can't
// race on advancing the dev ref. Within the lock the sequence is
// "rebase wip onto current dev → if clean, update-ref dev = wip-tip".
// The rebase happens in the slot's *own worktree* (`git worktree list`
// resolves where wip/<alias> is checked out), not in the operator's tree
// — so the operator's checkout stays untouched.
//
// Skips when wip/<agentID> doesn't exist (host mode never created one)
// or when wip is already an ancestor of dev (no work).
func syncSlotWipToDev(agentID string, log func(string, ...any)) (string, error) {
	if strings.TrimSpace(agentID) == "" {
		return "", nil
	}
	branch := "wip/" + agentID

	// Branch existence check — gentle no-op for host mode.
	if err := exec.Command("git", "rev-parse", "--verify", "refs/heads/"+branch).Run(); err != nil {
		return "", nil
	}

	wipSyncMu.Lock()
	defer wipSyncMu.Unlock()

	// If wip is already in dev (no commits past dev), nothing to do.
	if err := exec.Command("git", "merge-base", "--is-ancestor", "refs/heads/"+branch, "refs/heads/dev").Run(); err == nil {
		return "", nil
	}

	// If wip is a fast-forward of dev (dev is ancestor of wip), advance
	// dev directly via update-ref — no rebase, no checkout, operator's
	// working tree is untouched.
	if err := exec.Command("git", "merge-base", "--is-ancestor", "refs/heads/dev", "refs/heads/"+branch).Run(); err == nil {
		return advanceDevToBranchTip(branch, log)
	}

	// dev moved (a sibling slot merged first). Rebase wip onto dev in the
	// slot's worktree, then advance.
	wt, err := worktreePathFor(branch)
	if err != nil {
		return "", fmt.Errorf("locate worktree for %s: %w", branch, err)
	}
	if wt == "" {
		// Branch exists but isn't checked out anywhere — operator workflow
		// or post-cleanup state. Skip rebase, refuse to merge dirty
		// histories without a checkout.
		return "", fmt.Errorf("branch %s has no worktree to rebase in; skipping sync", branch)
	}
	if out, rerr := exec.Command("git", "-C", wt, "rebase", "refs/heads/dev").CombinedOutput(); rerr != nil {
		// Rebase conflicted (or other failure). Abort to leave the slot
		// in a clean state so the next session can claim work; surface
		// the original error so the operator notices.
		_ = exec.Command("git", "-C", wt, "rebase", "--abort").Run()
		return "", fmt.Errorf("rebase %s onto dev: %s", branch, strings.TrimSpace(string(out)))
	}
	return advanceDevToBranchTip(branch, log)
}

// advanceDevToBranchTip writes dev → <branch>'s current tip via
// update-ref. Atomic at the git layer; safe under wipSyncMu.
func advanceDevToBranchTip(branch string, log func(string, ...any)) (string, error) {
	tipBytes, err := exec.Command("git", "rev-parse", "refs/heads/"+branch).Output()
	if err != nil {
		return "", fmt.Errorf("rev-parse %s: %w", branch, err)
	}
	tip := strings.TrimSpace(string(tipBytes))
	if out, uerr := exec.Command("git", "update-ref", "refs/heads/dev", tip).CombinedOutput(); uerr != nil {
		return "", fmt.Errorf("update-ref dev → %s: %s", tip[:8], strings.TrimSpace(string(out)))
	}
	if len(tip) > 8 {
		return tip[:8], nil
	}
	return tip, nil
}

// worktreePathFor returns the worktree path where `branch` is currently
// checked out, or "" if the branch is not checked out anywhere. Parses
// `git worktree list --porcelain` output. We need this because rebase
// requires a checkout, and the slot's own worktree is the natural place
// — never the operator's.
func worktreePathFor(branch string) (string, error) {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").Output()
	if err != nil {
		return "", err
	}
	target := "branch refs/heads/" + branch
	var lastPath string
	for _, line := range strings.Split(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			lastPath = strings.TrimPrefix(line, "worktree ")
		case line == target:
			return lastPath, nil
		}
	}
	return "", nil
}

// processPostSessionEscalation checks for escalation results written by
// dx test --escalate (or by the agent running it) and, if any filed
// test-fix issues are found, blocks the claimed issue on them so the
// agent's parent work stays gated until the test failure is fixed.
func processPostSessionEscalation(ctx context.Context, rc remoteConfig, todo *claimedTodo, issueID, sessionID string, log func(string, ...any)) {
	// Look for escalation results in the session's agent directory.
	escPath := filepath.Join(".zdx", "agent", sessionID, "escalation.json")
	if _, err := os.Stat(escPath); os.IsNotExist(err) {
		// Also check the flat .zdx directory.
		escPath = filepath.Join(".zdx", "escalation-results.json")
		if _, err := os.Stat(escPath); os.IsNotExist(err) {
			return
		}
	}

	data, err := os.ReadFile(escPath)
	if err != nil {
		log("escalation: read %s: %v", escPath, err)
		return
	}

	var escResults []struct {
		TestName string `json:"test_name"`
		Action   string `json:"action"`
		IssueID  string `json:"issue_id"`
	}
	if err := json.Unmarshal(data, &escResults); err != nil {
		log("escalation: parse: %v", err)
		return
	}

	// Collect unique blocker issue IDs.
	seen := make(map[string]bool)
	var blockerIDs []string
	for _, r := range escResults {
		if r.IssueID != "" && !seen[r.IssueID] {
			seen[r.IssueID] = true
			blockerIDs = append(blockerIDs, r.IssueID)
		}
	}
	if len(blockerIDs) == 0 {
		return
	}

	// Block the claimed issue on each test-fix issue.
	for _, blockerID := range blockerIDs {
		if err := blockIssueOn(ctx, rc, issueID, blockerID); err != nil {
			log("escalation: block %s on %s: %v", issueID, blockerID, err)
			continue
		}
		log("escalation: blocked %s on %s (preexisting test failure)", issueID, blockerID)
	}
}

// blockIssueOn adds a block edge: issueID is blocked by blockerID.
func blockIssueOn(ctx context.Context, rc remoteConfig, issueID, blockerID string) error {
	c, err := cli.DefaultClient()
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	slug := c.Slug()
	if slug == "" {
		return fmt.Errorf("no project slug")
	}

	resp, err := c.IssueAddBlockWithResponse(ctx, dxclient.IssueAddBlockJSONRequestBody{
		Slug:      slug,
		Id:        extractIssueNumber(issueID),
		BlockedBy: blockerID,
		Kind:      ptrString("sequencing"),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() >= 400 {
		return fmt.Errorf("block: HTTP %d", resp.StatusCode())
	}
	return nil
}

func extractIssueNumber(id string) int32 {
	// Parse "IS-123" → 123.
	if strings.HasPrefix(id, "IS-") {
		n, _ := strconv.Atoi(strings.TrimPrefix(id, "IS-"))
		return int32(n)
	}
	n, _ := strconv.Atoi(id)
	return int32(n)
}

func ptrString(s string) *string {
	return &s
}
