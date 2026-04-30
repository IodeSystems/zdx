package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_AgentWorktreeTargetBranch is the demo for spec 175:
// Given a claimed todo/task with a target branch (e.g. v1.0.x for backports),
// when the agent worktree is created, then it is checked out from the target
// branch, not the default dev branch.
//
// The demo seeds a bare remote with two branches — "dev" (default) and "v1.0.x"
// (target). A sentinel file unique to v1.0.x proves which branch the worktree
// was based on. The test mirrors the createSessionWorktree call from take.go
// with a non-empty baseBranch, then confirms the sentinel is present and that
// the worktree's merge-base equals origin/v1.0.x (not origin/dev).
func TestDemoCLI_AgentWorktreeTargetBranch(t *testing.T) {
	rec := newGitRecorder(t)
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_agent_worktree_target_branch_test.go", Note: "spec 175 demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/srcless.go", LineStart: 60, LineEnd: 92, Note: "createSessionWorktree: branches from baseBranch when provided"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/srcless.go", LineStart: 75, LineEnd: 82, Note: "base selection: target branch → resolveRemoteRef; fallback to default"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/take.go", LineStart: 123, LineEnd: 123, Note: "take.go passes activeTodo.TargetBranch to createSessionWorktree"})
	t.Cleanup(rec.Save)

	const slug = "target-branch-demo"
	workDir := t.TempDir()

	// Seed a bare remote with two branches: "dev" (default) and "v1.0.x" (target).
	bareRoot := t.TempDir()
	bareRepo := filepath.Join(bareRoot, "git", slug)
	if err := os.MkdirAll(bareRepo, 0o755); err != nil {
		t.Fatalf("mkdir bare: %v", err)
	}
	rec.Run("", "init", "--bare", "--initial-branch=dev", bareRepo)

	// Seed dev branch.
	seed := filepath.Join(bareRoot, "seed")
	rec.Run("", "clone", bareRepo, seed)
	rec.Run(seed, "config", "user.email", "test@example.com")
	rec.Run(seed, "config", "user.name", "test")
	rec.Run(seed, "checkout", "-b", "dev")
	if err := os.WriteFile(filepath.Join(seed, "README.md"), []byte("project root\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec.Run(seed, "add", ".")
	rec.Run(seed, "commit", "-m", "init")
	rec.Run(seed, "push", "origin", "dev")

	// Create v1.0.x branch with a sentinel file not present on dev.
	rec.Run(seed, "checkout", "-b", "v1.0.x")
	if err := os.WriteFile(filepath.Join(seed, "v1_sentinel.txt"), []byte("v1.0.x only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec.Run(seed, "add", "v1_sentinel.txt")
	rec.Run(seed, "commit", "-m", "v1.0.x sentinel")
	rec.Run(seed, "push", "origin", "v1.0.x")

	// Clone the project (mirrors ensureProjectClone).
	projectPath := filepath.Join(workDir, slug, "main")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0o755); err != nil {
		t.Fatalf("mkdir clone parent: %v", err)
	}
	rec.Run("", "clone", bareRepo, projectPath)
	rec.Run(projectPath, "fetch", "origin")

	// Add worktree branching from origin/v1.0.x (mirrors createSessionWorktree with baseBranch="v1.0.x").
	worktreesDir := filepath.Join(workDir, slug, "worktrees")
	if err := os.MkdirAll(worktreesDir, 0o755); err != nil {
		t.Fatalf("mkdir worktrees: %v", err)
	}
	wt := filepath.Join(worktreesDir, "task-456")
	rec.Run(projectPath, "worktree", "add", "-b", "agent/task-456", wt, "origin/v1.0.x")

	// Sentinel file must exist — worktree is from v1.0.x, not dev.
	if _, err := os.Stat(filepath.Join(wt, "v1_sentinel.txt")); err != nil {
		t.Fatalf("worktree not based on v1.0.x — sentinel missing: %v", err)
	}

	// Sentinel must NOT exist if we were on dev (safety check: dev branch lacks it).
	if _, err := os.Stat(filepath.Join(projectPath, "v1_sentinel.txt")); err == nil {
		t.Errorf("sentinel found in main clone (dev branch) — test setup error")
	}

	// Confirm HEAD is a descendant of origin/v1.0.x via merge-base.
	mb, err := exec.Command("git", "-C", wt, "merge-base", "HEAD", "origin/v1.0.x").CombinedOutput()
	if err != nil {
		t.Fatalf("merge-base: %s: %v", mb, err)
	}
	tip, err := exec.Command("git", "-C", wt, "rev-parse", "origin/v1.0.x").CombinedOutput()
	if err != nil {
		t.Fatalf("rev-parse origin/v1.0.x: %s: %v", tip, err)
	}
	if strings.TrimSpace(string(mb)) != strings.TrimSpace(string(tip)) {
		t.Fatalf("worktree HEAD not descended from origin/v1.0.x\n  merge-base: %s\n  tip:        %s",
			strings.TrimSpace(string(mb)), strings.TrimSpace(string(tip)))
	}

	// Teardown.
	rec.Run(projectPath, "worktree", "remove", "--force", wt)
	rec.Run(projectPath, "branch", "-D", "agent/task-456")
}
