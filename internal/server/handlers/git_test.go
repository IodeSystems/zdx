package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@test.local",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@test.local",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestCommitDrift covers spec 156: drift query returns correct count and oldest
// commit age for a branch N commits behind dev, and returns zero for no drift.
func TestCommitDrift(t *testing.T) {
	dir := t.TempDir()

	gitCmd(t, dir, "init", "-b", "main")
	gitCmd(t, dir, "config", "user.email", "test@test.local")
	gitCmd(t, dir, "config", "user.name", "Test")

	// Initial commit shared by both branches.
	writeFile(t, dir, "base.txt", "base")
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "base")

	// release/staging stays at the base commit.
	gitCmd(t, dir, "branch", "release/staging")

	// Switch to dev and add 3 commits.
	gitCmd(t, dir, "checkout", "-b", "dev")
	beforeCommit := time.Now().Unix()
	for i := 0; i < 3; i++ {
		writeFile(t, dir, fmt.Sprintf("f%d.txt", i), "x")
		gitCmd(t, dir, "add", ".")
		gitCmd(t, dir, "commit", "-m", fmt.Sprintf("commit %d", i))
	}
	afterCommit := time.Now().Unix()

	t.Run("branch 3 commits behind dev", func(t *testing.T) {
		count, oldest, err := CommitDrift(dir, "release/staging", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 3 {
			t.Errorf("drift count: want 3, got %d", count)
		}
		if oldest == 0 {
			t.Error("oldest unix timestamp should be non-zero")
		}
		// Oldest commit must fall within our test window.
		if oldest < beforeCommit || oldest > afterCommit+1 {
			t.Errorf("oldest unix %d outside expected range [%d, %d]", oldest, beforeCommit, afterCommit+1)
		}
	})

	t.Run("zero drift — same branch", func(t *testing.T) {
		count, oldest, err := CommitDrift(dir, "dev", "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("zero-drift count: want 0, got %d", count)
		}
		if oldest != 0 {
			t.Errorf("zero-drift oldest: want 0, got %d", oldest)
		}
	})

	t.Run("missing branch returns error", func(t *testing.T) {
		_, _, err := CommitDrift(dir, "nonexistent", "dev")
		if err == nil {
			t.Error("expected error for missing branch, got nil")
		}
	})
}
