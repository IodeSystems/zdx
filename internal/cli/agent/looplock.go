package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// acquireLoopLock takes an exclusive flock on the repo's git common
// directory so only one `dx agent loop` runs against a given checkout
// at a time. Two concurrent loops on the same working tree would race
// each other on `git checkout` and HEAD-flip operations performed by
// the merge-train and the agent sessions; we observed this in the wild
// when an agent flipped the operator's HEAD from main → dev-TK-1794
// mid-session.
//
// The lock file lives at <git-common-dir>/zdx-agent-loop.lock. We use
// --git-common-dir rather than --git-dir so the lock is shared across
// linked worktrees from the same origin checkout — exactly the boundary
// we want to serialize at.
//
// Returns the held lock file. Callers MUST defer releaseLoopLock(f).
// On collision, returns an error including the holder's PID so the
// operator can `ps` it.
func acquireLoopLock() (*os.File, error) {
	dir, err := loopLockCommonDir()
	if err != nil {
		return nil, err
	}
	lockPath := filepath.Join(dir, "zdx-agent-loop.lock")
	f, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open loop lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		// Read existing pid for the diagnostic before closing our fd.
		pidBytes, _ := os.ReadFile(lockPath)
		holder := strings.TrimSpace(string(pidBytes))
		if holder == "" {
			holder = "unknown"
		}
		f.Close()
		return nil, fmt.Errorf("another `dx agent loop` is already running in this checkout (lock=%s, holder pid=%s); only one loop per repo is permitted to prevent agents from flipping HEAD on each other", lockPath, holder)
	}
	// Stamp our PID into the lock file so the next would-be acquirer can
	// report who's holding it. Best-effort: a write failure here doesn't
	// invalidate the lock we just took.
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return f, nil
}

// releaseLoopLock releases the flock and closes the file. Nil-safe so
// callers can `defer releaseLoopLock(lock)` after a failed acquire path
// (where lock will be nil).
//
// flock is automatically released on process exit too, so signal-handler
// teardown does not need to call this explicitly — the defer covers the
// graceful path and the kernel covers the abrupt one.
func releaseLoopLock(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}

// loopLockCommonDir returns the git common directory for the cwd's repo.
// For a non-worktree checkout, this is the same as .git. For a linked
// worktree, this is the original repo's .git (shared across all
// worktrees from the same origin), which is exactly the boundary we
// want the lock to operate at — slot worktrees and the parent loop
// share one lock so a separate loop in a sibling worktree still gets
// refused.
func loopLockCommonDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --git-common-dir: %w", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		return "", fmt.Errorf("git rev-parse --git-common-dir: empty output")
	}
	// Resolve to absolute path so a later os.Chdir doesn't relocate the
	// lock path under our feet.
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	return dir, nil
}
