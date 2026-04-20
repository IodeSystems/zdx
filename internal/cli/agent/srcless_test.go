package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRemote initializes a bare repo with one commit on main and returns its
// path. The clone URL the helpers expect is "<remoteBase>/git/<slug>", so the
// caller composes their own remoteURL after pointing /git/<slug> at this dir
// (or sets remoteBase such that ${remoteBase}/git/${slug} resolves to the
// bare repo path on disk — file:// URLs work because git treats the suffix
// path as the actual path).
func fakeRemote(t *testing.T, slug string) (remoteBase string, bareRepo string) {
	t.Helper()
	root := t.TempDir()

	// Layout: <root>/git/<slug> is the bare repo. Pointing remoteBase at
	// "file://<root>" means ensureProjectClone clones from
	// "file://<root>/git/<slug>".
	bareRepo = filepath.Join(root, "git", slug)
	if err := os.MkdirAll(bareRepo, 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	run(t, "", "git", "init", "--bare", "--initial-branch=main", bareRepo)

	// Seed the bare repo via a throwaway working clone so origin/main resolves
	// after subsequent clones.
	seed := filepath.Join(root, "seed")
	run(t, "", "git", "clone", bareRepo, seed)
	run(t, seed, "git", "config", "user.email", "test@example.com")
	run(t, seed, "git", "config", "user.name", "test")
	run(t, seed, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, seed, "git", "add", ".")
	run(t, seed, "git", "commit", "-m", "init")
	run(t, seed, "git", "push", "origin", "main")

	return "file://" + root, bareRepo
}

func run(t *testing.T, cwd string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %s: %v", name, args, strings.TrimSpace(string(out)), err)
	}
}

func TestEnsureProjectCloneIdempotent(t *testing.T) {
	remoteBase, _ := fakeRemote(t, "p1")
	workDir := t.TempDir()

	pp1, err := ensureProjectClone(workDir, "p1", remoteBase)
	if err != nil {
		t.Fatalf("first clone: %v", err)
	}
	if !strings.HasSuffix(pp1, filepath.Join("p1", "main")) {
		t.Fatalf("unexpected project path: %s", pp1)
	}
	if _, err := os.Stat(filepath.Join(pp1, ".git")); err != nil {
		t.Fatalf("clone missing .git: %v", err)
	}

	pp2, err := ensureProjectClone(workDir, "p1", remoteBase)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if pp1 != pp2 {
		t.Fatalf("path changed across calls: %s vs %s", pp1, pp2)
	}
}

func TestCreateSessionWorktree(t *testing.T) {
	remoteBase, _ := fakeRemote(t, "p2")
	workDir := t.TempDir()

	pp, err := ensureProjectClone(workDir, "p2", remoteBase)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	wt, branch, err := createSessionWorktree(pp, workDir, "p2", "abc123")
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if branch != "agent/abc123" {
		t.Fatalf("branch = %q, want agent/abc123", branch)
	}
	if !strings.HasSuffix(wt, filepath.Join("worktrees", "abc123")) {
		t.Fatalf("worktree path: %s", wt)
	}
	if _, err := os.Stat(filepath.Join(wt, "README.md")); err != nil {
		t.Fatalf("worktree missing seeded file: %v", err)
	}

	// Removal should drop both the worktree dir and the branch ref.
	if err := removeSessionWorktree(pp, wt, branch); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(wt); !os.IsNotExist(err) {
		t.Fatalf("worktree still present: %v", err)
	}
	out, _ := exec.Command("git", "-C", pp, "branch", "--list", branch).CombinedOutput()
	if strings.TrimSpace(string(out)) != "" {
		t.Fatalf("branch %s still present: %s", branch, out)
	}
}

func TestGcStaleWorktrees(t *testing.T) {
	workDir := t.TempDir()
	fresh := filepath.Join(workDir, "p", "worktrees", "fresh")
	stale := filepath.Join(workDir, "p", "worktrees", "stale")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	// Backdate the stale entry.
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	removed := gcStaleWorktrees(workDir, time.Hour, nil)
	if len(removed) != 1 {
		t.Fatalf("removed = %d, want 1: %v", len(removed), removed)
	}
	if removed[0] != stale {
		t.Fatalf("removed[0] = %s, want %s", removed[0], stale)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Fatalf("fresh worktree gone: %v", err)
	}
}
