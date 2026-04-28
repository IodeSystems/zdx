package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_AgentWorktreeCleanup is the demo for spec 124: worktrees are
// cleaned up automatically after a session ends — no leaked branches or
// directories on the host. The harness has two cleanup paths and the test
// exercises both:
//
//  1. Session-end teardown — the deferred removeSessionWorktree call in
//     internal/cli/agent/take.go runs when a session returns, removing the
//     worktree directory and deleting agent/<sid>.
//  2. Stale-worktree GC — gcStaleWorktrees in internal/cli/agent/srcless.go
//     sweeps ${workDir}/*/worktrees/* for entries whose mtime is older than
//     2×lease, catching crashed/abandoned sessions that skipped (1).
//
// After both paths run, no agent branches and no worktree directories remain
// on the host.
func TestDemoCLI_AgentWorktreeCleanup(t *testing.T) {
	rec := newGitRecorder(t)
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_agent_worktree_cleanup_test.go", Note: "agent worktree cleanup demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/srcless.go", LineStart: 100, LineEnd: 119, Note: "removeSessionWorktree: per-session teardown (worktree dir + branch)"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/srcless.go", LineStart: 184, LineEnd: 220, Note: "gcStaleWorktrees: mtime-based sweep for crashed/abandoned sessions"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/take.go", LineStart: 142, LineEnd: 152, Note: "deferred cleanup: removeSessionWorktree runs when a session returns"})
	t.Cleanup(rec.Save)

	const slug = "cleanup-demo"
	workDir := t.TempDir()

	// Seed a bare upstream and clone it as the main project checkout. Layout
	// mirrors what the agent harness produces:
	//   <workDir>/<slug>/main            — persistent main clone
	//   <workDir>/<slug>/worktrees/<sid> — per-session worktrees
	bareRoot := t.TempDir()
	bareRepo := filepath.Join(bareRoot, "git", slug)
	if err := os.MkdirAll(bareRepo, 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	rec.Run("", "init", "--bare", "--initial-branch=main", bareRepo)

	seed := filepath.Join(bareRoot, "seed")
	rec.Run("", "clone", bareRepo, seed)
	rec.Run(seed, "config", "user.email", "test@example.com")
	rec.Run(seed, "config", "user.name", "test")
	rec.Run(seed, "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("project root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec.Run(seed, "add", ".")
	rec.Run(seed, "commit", "-m", "init")
	rec.Run(seed, "push", "origin", "main")

	projectPath := filepath.Join(workDir, slug, "main")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("mkdir clone parent: %v", err)
	}
	rec.Run("", "clone", bareRepo, projectPath)

	worktreesDir := filepath.Join(workDir, slug, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}

	// Create two session worktrees + branches. session-clean follows the happy
	// path (session ends → deferred cleanup runs). session-crashed simulates a
	// crash that skipped its defer — the GC pass must catch it.
	rec.Run(projectPath, "fetch", "origin")
	wtClean := filepath.Join(worktreesDir, "session-clean")
	rec.Run(projectPath, "worktree", "add", "-b", "agent/session-clean", wtClean, "origin/main")

	wtCrashed := filepath.Join(worktreesDir, "session-crashed")
	rec.Run(projectPath, "worktree", "add", "-b", "agent/session-crashed", wtCrashed, "origin/main")

	// Sanity: both worktrees and both branches exist before cleanup runs.
	for _, wt := range []string{wtClean, wtCrashed} {
		if _, err := os.Stat(wt); err != nil {
			t.Fatalf("pre-cleanup: worktree %s missing: %v", wt, err)
		}
	}
	for _, br := range []string{"agent/session-clean", "agent/session-crashed"} {
		if !branchExists(t, projectPath, br) {
			t.Fatalf("pre-cleanup: branch %s missing", br)
		}
	}

	// ── Path 1: session-end teardown ─────────────────────────────────────────
	// Mirror the deferred removeSessionWorktree call in take.go: remove the
	// worktree directory then delete its branch.
	rec.Run(projectPath, "worktree", "remove", "--force", wtClean)
	rec.Run(projectPath, "branch", "-D", "agent/session-clean")

	if _, err := os.Stat(wtClean); !os.IsNotExist(err) {
		t.Errorf("session-clean worktree dir still present after session-end cleanup: %v", err)
	}
	if branchExists(t, projectPath, "agent/session-clean") {
		t.Errorf("session-clean branch still present after session-end cleanup")
	}

	// ── Path 2: stale-worktree GC ────────────────────────────────────────────
	// Backdate session-crashed so it falls outside the GC age window, then
	// run the same sweep gcStaleWorktrees performs: glob worktree dirs older
	// than maxAge and remove them along with their agent/<sid> branches.
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(wtCrashed, old, old); err != nil {
		t.Fatalf("chtimes crashed worktree: %v", err)
	}

	removed := sweepStaleWorktrees(t, workDir, time.Hour)
	if len(removed) != 1 || removed[0] != wtCrashed {
		t.Fatalf("GC sweep removed = %v, want [%s]", removed, wtCrashed)
	}

	if _, err := os.Stat(wtCrashed); !os.IsNotExist(err) {
		t.Errorf("session-crashed worktree dir still present after GC: %v", err)
	}
	if branchExists(t, projectPath, "agent/session-crashed") {
		t.Errorf("session-crashed branch still present after GC")
	}

	// ── Final assertion: no agent/* branches and no worktree dirs remain.
	branchOut := rec.RunOutput(projectPath, "branch", "--list", "agent/*")
	if leftover := strings.TrimSpace(string(branchOut)); leftover != "" {
		t.Errorf("leaked agent branches after cleanup: %s", leftover)
	}

	entries, err := os.ReadDir(worktreesDir)
	if err != nil {
		t.Fatalf("readdir worktrees: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("leaked worktree directories after cleanup: %v", names)
	}
}

// branchExists returns true if branch is present in the project's local refs.
func branchExists(t *testing.T, projectPath, branch string) bool {
	t.Helper()
	out, _ := exec.Command("git", "-C", projectPath, "branch", "--list", branch).CombinedOutput()
	return strings.TrimSpace(string(out)) != ""
}

// sweepStaleWorktrees mirrors gcStaleWorktrees in srcless.go: walk
// ${workDir}/*/worktrees/* and remove entries whose mtime is older than maxAge
// along with their agent/<sid> branches in the sibling ./main clone.
func sweepStaleWorktrees(t *testing.T, workDir string, maxAge time.Duration) []string {
	t.Helper()
	matches, _ := filepath.Glob(filepath.Join(workDir, "*", "worktrees", "*"))
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
		projectPath := filepath.Join(filepath.Dir(filepath.Dir(wt)), "main")
		branch := "agent/" + filepath.Base(wt)
		_, _ = exec.Command("git", "-C", projectPath, "worktree", "remove", "--force", wt).CombinedOutput()
		_ = os.RemoveAll(wt)
		_, _ = exec.Command("git", "-C", projectPath, "branch", "-D", branch).CombinedOutput()
		removed = append(removed, wt)
	}
	return removed
}
