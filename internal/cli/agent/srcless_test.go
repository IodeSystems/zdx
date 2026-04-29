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
	wt, branch, err := createSessionWorktree(pp, workDir, "p2", "abc123", "")
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

func TestCreateSessionWorktree_TargetBranch(t *testing.T) {
	remoteBase, bareRepo := fakeRemote(t, "p2b")
	workDir := t.TempDir()

	// Push a v1.0.x branch carrying a sentinel file that does not exist on
	// main, so the worktree's contents prove which branch it was based on.
	side := filepath.Join(t.TempDir(), "side")
	run(t, "", "git", "clone", bareRepo, side)
	run(t, side, "git", "config", "user.email", "test@example.com")
	run(t, side, "git", "config", "user.name", "test")
	run(t, side, "git", "checkout", "-b", "v1.0.x")
	if err := os.WriteFile(filepath.Join(side, "v1_sentinel.txt"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, side, "git", "add", "v1_sentinel.txt")
	run(t, side, "git", "commit", "-m", "v1.0.x sentinel")
	run(t, side, "git", "push", "origin", "v1.0.x")

	pp, err := ensureProjectClone(workDir, "p2b", remoteBase)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}
	wt, branch, err := createSessionWorktree(pp, workDir, "p2b", "tgt123", "v1.0.x")
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	if branch != "agent/tgt123" {
		t.Fatalf("branch = %q, want agent/tgt123", branch)
	}
	if _, err := os.Stat(filepath.Join(wt, "v1_sentinel.txt")); err != nil {
		t.Fatalf("worktree not based on v1.0.x — sentinel missing: %v", err)
	}

	// Confirm HEAD is a descendant of origin/v1.0.x (merge-base equals it).
	mb, err := exec.Command("git", "-C", wt, "merge-base", "HEAD", "origin/v1.0.x").CombinedOutput()
	if err != nil {
		t.Fatalf("merge-base: %s: %v", mb, err)
	}
	tip, err := exec.Command("git", "-C", wt, "rev-parse", "origin/v1.0.x").CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse: %s: %v", tip, err)
	}
	if strings.TrimSpace(string(mb)) != strings.TrimSpace(string(tip)) {
		t.Fatalf("HEAD not descended from origin/v1.0.x: merge-base=%s tip=%s",
			strings.TrimSpace(string(mb)), strings.TrimSpace(string(tip)))
	}
}

func TestEnsureProjectInit_AlreadyConfigured(t *testing.T) {
	remoteBase, _ := fakeRemote(t, "p3")
	workDir := t.TempDir()
	pp, err := ensureProjectClone(workDir, "p3", remoteBase)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	// Write a config with a remote section so ensureProjectInit should skip.
	cfg := filepath.Join(pp, ".zdx", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("remote:\n  url: https://zdx.example.com\n  slug: p3\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// dxPath is irrelevant — should not be called.
	if err := ensureProjectInit(pp, "p3", "https://zdx.example.com", "key", "/nonexistent/dx"); err != nil {
		t.Fatalf("expected skip, got error: %v", err)
	}
}

func TestEnsureProjectInit_FreshClone(t *testing.T) {
	remoteBase, _ := fakeRemote(t, "p4")
	workDir := t.TempDir()
	pp, err := ensureProjectClone(workDir, "p4", remoteBase)
	if err != nil {
		t.Fatalf("clone: %v", err)
	}

	// Build a fake dx script that writes a minimal config with remote section.
	fakeDir := t.TempDir()
	fakeDx := filepath.Join(fakeDir, "dx")
	script := `#!/bin/sh
# fake dx init: parse --url and --slug, write .zdx/config.yaml
url=""
slug=""
for arg in "$@"; do
  case "$arg" in
    --url=*) url="${arg#--url=}" ;;
    --slug=*) slug="${arg#--slug=}" ;;
  esac
done
if [ "$1" = "init" ]; then
  mkdir -p .zdx
  printf "remote:\n  url: %s\n  slug: %s\n" "$url" "$slug" > .zdx/config.yaml
  touch CLAUDE.md
fi
`
	if err := os.WriteFile(fakeDx, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := ensureProjectInit(pp, "p4", "https://zdx.example.com", "testkey", fakeDx); err != nil {
		t.Fatalf("ensureProjectInit: %v", err)
	}

	// Verify config was written and committed to origin.
	cfgPath := filepath.Join(pp, ".zdx", "config.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config not written: %v", err)
	}
	if !strings.Contains(string(data), "remote:") {
		t.Fatalf("config missing remote section: %s", data)
	}

	// Verify the commit landed on origin (bare repo has the commit).
	bareRepo := strings.TrimPrefix(remoteBase, "file://") + "/git/p4"
	out, err := exec.Command("git", "-C", bareRepo, "log", "--oneline", "-1").CombinedOutput()
	if err != nil {
		t.Fatalf("git log on bare: %v", err)
	}
	if !strings.Contains(string(out), "scaffold zdx project") {
		t.Fatalf("scaffold commit not found on origin: %s", out)
	}

	// Second call should be a no-op (idempotent).
	if err := ensureProjectInit(pp, "p4", "https://zdx.example.com", "testkey", "/nonexistent/dx"); err != nil {
		t.Fatalf("second call should skip, got: %v", err)
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
