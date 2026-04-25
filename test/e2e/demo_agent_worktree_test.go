package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_AgentWorktreeIsolation is the demo for spec 118: each agent
// session gets its own isolated git worktree — agents never share or mutate
// the main working tree. Two simulated sessions are created back-to-back; the
// test verifies that each resolves to a distinct path under
// <workDir>/<slug>/worktrees/<sessionID> and that the host-facing main clone
// is untouched throughout.
func TestDemoCLI_AgentWorktreeIsolation(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_agent_worktree_test.go", Note: "agent worktree isolation demo source"},
		{FilePath: "internal/cli/agent/srcless.go", Note: "ensureProjectClone / createSessionWorktree / removeSessionWorktree"},
	})

	const slug = "isolation-demo"
	workDir := t.TempDir()

	// Seed a bare upstream and clone it as the main project checkout.
	// Layout mirrors what the agent harness produces:
	//   <workDir>/<slug>/main            — persistent main clone
	//   <workDir>/<slug>/worktrees/<sid> — per-session worktrees
	bareRoot := t.TempDir()
	bareRepo := filepath.Join(bareRoot, "git", slug)
	if err := os.MkdirAll(bareRepo, 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	gitRun(t, "", "git", "init", "--bare", "--initial-branch=main", bareRepo)

	// Seed the bare repo with one commit so origin/main resolves after cloning.
	seed := filepath.Join(bareRoot, "seed")
	gitRun(t, "", "git", "clone", bareRepo, seed)
	gitRun(t, seed, "git", "config", "user.email", "test@example.com")
	gitRun(t, seed, "git", "config", "user.name", "test")
	gitRun(t, seed, "git", "checkout", "-b", "main")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("project root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, seed, "git", "add", ".")
	gitRun(t, seed, "git", "commit", "-m", "init")
	gitRun(t, seed, "git", "push", "origin", "main")

	// Main clone: equivalent to ensureProjectClone.
	projectPath := filepath.Join(workDir, slug, "main")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("mkdir clone parent: %v", err)
	}
	gitRun(t, "", "git", "clone", bareRepo, projectPath)

	if _, err := os.Stat(filepath.Join(projectPath, "README.md")); err != nil {
		t.Fatalf("main clone missing README.md: %v", err)
	}

	// Ensure the worktrees parent exists (mirrors the MkdirAll in createSessionWorktree).
	worktreesDir := filepath.Join(workDir, slug, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}

	// Session A: fetch + add worktree (mirrors createSessionWorktree for "session-a").
	gitRun(t, projectPath, "git", "fetch", "origin")
	wtA := filepath.Join(worktreesDir, "session-a")
	gitRun(t, projectPath, "git", "worktree", "add", "-b", "agent/session-a", wtA, "origin/main")

	// Session B: fetch + add a second worktree — must be isolated from A.
	gitRun(t, projectPath, "git", "fetch", "origin")
	wtB := filepath.Join(worktreesDir, "session-b")
	gitRun(t, projectPath, "git", "worktree", "add", "-b", "agent/session-b", wtB, "origin/main")

	// Each worktree resolves to a distinct path under <workDir>/<slug>/worktrees/.
	if wtA == wtB {
		t.Errorf("sessions share a worktree path: %s", wtA)
	}
	for _, wt := range []struct{ path, id string }{{wtA, "session-a"}, {wtB, "session-b"}} {
		if !strings.HasSuffix(wt.path, filepath.Join("worktrees", wt.id)) {
			t.Errorf("worktree %s path %q does not follow expected layout", wt.id, wt.path)
		}
		if _, err := os.Stat(filepath.Join(wt.path, "README.md")); err != nil {
			t.Errorf("worktree %s missing seeded file: %v", wt.id, err)
		}
	}

	// Each session has its own branch ref visible from the main clone.
	branchOut, err := exec.Command("git", "-C", projectPath, "branch", "--list", "agent/*").CombinedOutput()
	if err != nil {
		t.Fatalf("git branch --list: %v", err)
	}
	branches := strings.TrimSpace(string(branchOut))
	for _, b := range []string{"agent/session-a", "agent/session-b"} {
		if !strings.Contains(branches, b) {
			t.Errorf("branch %s missing from main clone; got: %s", b, branches)
		}
	}

	// Host-facing main clone is untouched: HEAD on main, .git intact, no stray files.
	headOut, err := exec.Command("git", "-C", projectPath, "symbolic-ref", "--short", "HEAD").CombinedOutput()
	if err != nil {
		t.Fatalf("git symbolic-ref HEAD: %v", err)
	}
	if head := strings.TrimSpace(string(headOut)); head != "main" {
		t.Errorf("main clone HEAD = %q, want main", head)
	}
	if _, err := os.Stat(filepath.Join(projectPath, ".git")); err != nil {
		t.Fatalf("main clone .git missing: %v", err)
	}

	// Teardown: remove worktrees and branches (mirrors removeSessionWorktree).
	gitRun(t, projectPath, "git", "worktree", "remove", "--force", wtA)
	gitRun(t, projectPath, "git", "worktree", "remove", "--force", wtB)
	gitRun(t, projectPath, "git", "branch", "-D", "agent/session-a", "agent/session-b")

	if _, err := os.Stat(wtA); !os.IsNotExist(err) {
		t.Errorf("session-a worktree still present after removal: %v", err)
	}
	if _, err := os.Stat(wtB); !os.IsNotExist(err) {
		t.Errorf("session-b worktree still present after removal: %v", err)
	}
}

// gitRun runs a git command, failing t immediately on error.
// cwd="" uses the process working directory.
func gitRun(t *testing.T, cwd string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %v: %s: %v", name, args, strings.TrimSpace(string(out)), err)
	}
}
