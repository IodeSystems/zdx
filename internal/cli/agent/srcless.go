package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// srclessProjectPath returns the persistent main clone directory for a project
// under workDir: ${workDir}/${slug}/main.
func srclessProjectPath(workDir, slug string) string {
	return filepath.Join(workDir, slug, "main")
}

// srclessWorktreePath returns the per-session worktree directory:
// ${workDir}/${slug}/worktrees/${sessionID}.
func srclessWorktreePath(workDir, slug, sessionID string) string {
	return filepath.Join(workDir, slug, "worktrees", sessionID)
}

// ensureProjectClone makes sure the main clone for slug exists at
// ${workDir}/${slug}/main. Returns the absolute clone path. Idempotent.
//
// remoteURL is the zdx server base (e.g. https://zdx.example.com); the proxy
// route is ${remoteURL}/git/${slug}. Authentication is left to git's
// configured credential helper (the dx credential-helper installed per
// IS-354). On a fresh clone we run `git fetch origin` to populate refs even
// if the upstream had no default branch yet.
func ensureProjectClone(workDir, slug, remoteURL string) (string, error) {
	if workDir == "" || slug == "" || remoteURL == "" {
		return "", fmt.Errorf("ensureProjectClone: empty workDir/slug/remoteURL")
	}
	projectPath := srclessProjectPath(workDir, slug)
	if st, err := os.Stat(filepath.Join(projectPath, ".git")); err == nil && st.IsDir() {
		return projectPath, nil
	}
	if _, err := os.Stat(projectPath); err == nil {
		return "", fmt.Errorf("ensureProjectClone: %s exists but is not a git repo", projectPath)
	}
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir parent: %w", err)
	}

	cloneURL := strings.TrimRight(remoteURL, "/") + "/git/" + slug
	out, err := exec.Command("git", "clone", cloneURL, projectPath).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone %s: %s: %w", cloneURL, strings.TrimSpace(string(out)), err)
	}
	return projectPath, nil
}

// createSessionWorktree fetches origin and creates a per-session worktree
// branched from origin/main (or origin/HEAD when main is missing — empty
// repos and non-main default branches).
//
// Returns the worktree path and branch name. Branch is `agent/${sessionID}`.
func createSessionWorktree(projectPath, workDir, slug, sessionID string) (string, string, error) {
	if projectPath == "" || sessionID == "" {
		return "", "", fmt.Errorf("createSessionWorktree: empty projectPath/sessionID")
	}

	if out, err := exec.Command("git", "-C", projectPath, "fetch", "origin").CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git fetch origin: %s: %w", strings.TrimSpace(string(out)), err)
	}

	worktreePath := srclessWorktreePath(workDir, slug, sessionID)
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir worktree parent: %w", err)
	}
	branch := "agent/" + sessionID

	base := resolveDefaultBase(projectPath)

	args := []string{"-C", projectPath, "worktree", "add", "-b", branch, worktreePath}
	if base != "" {
		args = append(args, base)
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return worktreePath, branch, nil
}

// resolveDefaultBase returns the upstream ref to branch from. Prefers
// origin/main, then origin/HEAD, then "" (let git pick HEAD — appropriate for
// a brand-new repo without any remote refs yet).
func resolveDefaultBase(projectPath string) string {
	for _, ref := range []string{"origin/main", "origin/HEAD"} {
		if err := exec.Command("git", "-C", projectPath, "rev-parse", "--verify", ref).Run(); err == nil {
			return ref
		}
	}
	return ""
}

// removeSessionWorktree tears down a session worktree and deletes its branch.
// Best-effort: errors are returned but not fatal — the GC pass cleans up
// stragglers. branch may be empty (skip branch delete).
func removeSessionWorktree(projectPath, worktreePath, branch string) error {
	var firstErr error
	if worktreePath != "" {
		if out, err := exec.Command("git", "-C", projectPath, "worktree", "remove", "--force", worktreePath).CombinedOutput(); err != nil {
			firstErr = fmt.Errorf("worktree remove: %s: %w", strings.TrimSpace(string(out)), err)
			// Fall back to filesystem rm so a corrupt worktree entry doesn't
			// permanently leak its directory.
			_ = os.RemoveAll(worktreePath)
		}
	}
	if branch != "" {
		if out, err := exec.Command("git", "-C", projectPath, "branch", "-D", branch).CombinedOutput(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("branch -D %s: %s: %w", branch, strings.TrimSpace(string(out)), err)
		}
	}
	return firstErr
}

// pushSessionBranch pushes the session branch to origin if it has commits
// ahead of origin/main. Returns nil + skipped=true when there is nothing to
// push (avoids creating empty remote refs for sessions that produced no work).
func pushSessionBranch(worktreePath, branch string) (skipped bool, err error) {
	if worktreePath == "" || branch == "" {
		return true, nil
	}
	// Count commits unique to this branch vs origin/main. If the base ref
	// doesn't exist (fresh repo), assume the branch has work.
	out, _ := exec.Command("git", "-C", worktreePath, "rev-list", "--count", "origin/main.."+branch).CombinedOutput()
	if n := strings.TrimSpace(string(out)); n == "0" {
		return true, nil
	}
	pushOut, pErr := exec.Command("git", "-C", worktreePath, "push", "origin", branch).CombinedOutput()
	if pErr != nil {
		return false, fmt.Errorf("git push origin %s: %s: %w", branch, strings.TrimSpace(string(pushOut)), pErr)
	}
	return false, nil
}

// gcStaleWorktrees walks ${workDir}/*/worktrees/* and removes entries whose
// mtime is older than maxAge. Returns the list of removed worktree paths so
// the caller can log them. Errors are logged via the logf callback (may be
// nil).
//
// This is a coarse pass — it does not consult the active claim/reservation
// table, only mtime. The lease lifecycle ensures live worktrees are touched
// (via the writes the agent makes inside them); a worktree older than
// `2 × lease_minutes` with no recent activity is by definition stale.
func gcStaleWorktrees(workDir string, maxAge time.Duration, logf func(string, ...any)) []string {
	if workDir == "" {
		return nil
	}
	matches, _ := filepath.Glob(filepath.Join(workDir, "*", "worktrees", "*"))
	if len(matches) == 0 {
		return nil
	}
	cutoff := time.Now().Add(-maxAge)
	var removed []string
	for _, wt := range matches {
		st, err := os.Stat(wt)
		if err != nil || !st.IsDir() {
			continue
		}
		if !st.ModTime().Before(cutoff) {
			continue
		}
		// projectPath = workDir/<slug>/main; wt = workDir/<slug>/worktrees/<sid>.
		projectPath := filepath.Join(filepath.Dir(filepath.Dir(wt)), "main")
		branch := "agent/" + filepath.Base(wt)
		if err := removeSessionWorktree(projectPath, wt, branch); err != nil && logf != nil {
			logf("gc: worktree %s: %v", wt, err)
		}
		removed = append(removed, wt)
	}
	return removed
}
