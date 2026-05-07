package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMergeTrain_SemanticConflictHalts is the negative-case proof for IS-648's
// forward-only rebase contract. Two branches edit the same line of the same
// query file. Branch A lands on dev first; running merge-train on branch B
// must surface the conflict, exit nonzero, and leave the repo in a state a
// human can pick up — no half-applied rebase, no force-merge, no orphan refs.
func TestMergeTrain_SemanticConflictHalts(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}
	dxBin := filepath.Join(root, "bin/dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not found at %q — run: make build", dxBin)
	}

	repo := t.TempDir()

	// Initialize repo with `dev` as the default branch (merge-train rebases onto dev).
	gitRun(t, "", "git", "init", "--initial-branch=dev", repo)
	gitRun(t, repo, "git", "config", "user.email", "test@example.com")
	gitRun(t, repo, "git", "config", "user.name", "test")
	gitRun(t, repo, "git", "config", "commit.gpgsign", "false")

	// Seed: queries/x.sql + README on dev.
	if err := os.MkdirAll(filepath.Join(repo, "queries"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("seed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "queries", "x.sql"),
		[]byte("-- name: GetX :one\nSELECT id FROM x;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "git", "add", ".")
	gitRun(t, repo, "git", "commit", "-m", "seed")
	devBaseSHA := gitOut(t, repo, "git", "rev-parse", "HEAD")

	// Branch A: rewrite the same SELECT line, then ff-merge into dev directly.
	// (We don't drive A through merge-train here — the negative case never
	// exercises the regen pipeline, so a real merge-train run for A would just
	// require the full zdx-go layout for no test gain. The conflict semantics
	// are identical regardless of how A's commit reached dev.)
	gitRun(t, repo, "git", "checkout", "-b", "agent/branch-a")
	if err := os.WriteFile(filepath.Join(repo, "queries", "x.sql"),
		[]byte("-- name: GetX :one\nSELECT id, name FROM x;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "git", "commit", "-am", "feat: A select id,name")
	gitRun(t, repo, "git", "checkout", "dev")
	gitRun(t, repo, "git", "merge", "--ff-only", "agent/branch-a")
	devTipAfterA := gitOut(t, repo, "git", "rev-parse", "HEAD")

	// Branch B: forks from the pre-A dev tip, edits the SAME line a different way.
	gitRun(t, repo, "git", "checkout", "-b", "agent/branch-b", devBaseSHA)
	if err := os.WriteFile(filepath.Join(repo, "queries", "x.sql"),
		[]byte("-- name: GetX :one\nSELECT id, age FROM x;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "git", "commit", "-am", "feat: B select id,age")
	bTipBefore := gitOut(t, repo, "git", "rev-parse", "HEAD")

	// Run merge-train on B. Rebase must conflict on queries/x.sql, abort, and
	// surface the conflict. Pass --dry-run=false explicitly so behavior is
	// independent of any future flag-default change.
	cmd := exec.Command(dxBin, "merge-train", "run", "agent/branch-b", "--dry-run=false")
	cmd.Dir = repo
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout := outBuf.String()
	stderr := errBuf.String()
	combined := stdout + "\n" + stderr

	if runErr == nil {
		t.Fatalf("expected merge-train to exit nonzero on semantic conflict; stdout=%q stderr=%q", stdout, stderr)
	}

	// Conflict file must be named in the output.
	if !strings.Contains(combined, "queries/x.sql") {
		t.Errorf("expected output to name conflicted file queries/x.sql\n--- stdout ---\n%s\n--- stderr ---\n%s", stdout, stderr)
	}
	// Branch must be named in the output.
	if !strings.Contains(combined, "agent/branch-b") {
		t.Errorf("expected output to name branch agent/branch-b\n--- stdout ---\n%s\n--- stderr ---\n%s", stdout, stderr)
	}
	// "forward-only" wording from the contract.
	if !strings.Contains(combined, "forward-only") {
		t.Errorf("expected forward-only contract wording in output\n--- stdout ---\n%s\n--- stderr ---\n%s", stdout, stderr)
	}

	// dev tip is unchanged from after A's merge — no partial regen, no force-merge.
	if got := gitOut(t, repo, "git", "rev-parse", "dev"); got != devTipAfterA {
		t.Errorf("dev tip changed: was %s, now %s", devTipAfterA, got)
	}

	// agent/branch-b tip preserved at its pre-rebase commit (rebase --abort).
	if got := gitOut(t, repo, "git", "rev-parse", "agent/branch-b"); got != bTipBefore {
		t.Errorf("agent/branch-b tip changed: was %s, now %s", bTipBefore, got)
	}

	// No half-applied rebase state in .git/.
	for _, name := range []string{"rebase-merge", "rebase-apply"} {
		if _, err := os.Stat(filepath.Join(repo, ".git", name)); !os.IsNotExist(err) {
			t.Errorf(".git/%s still present — half-applied rebase not cleaned up", name)
		}
	}

	// No regen commit landed on dev for branch-b.
	devLog := gitOut(t, repo, "git", "log", "--format=%s", "dev")
	if strings.Contains(devLog, "regen: agent/branch-b") {
		t.Errorf("unexpected 'regen: agent/branch-b' commit on dev:\n%s", devLog)
	}

	// No orphan refs littering the repo (only dev + the two agent branches).
	branchesOut := gitOut(t, repo, "git", "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	wantBranches := map[string]bool{"dev": false, "agent/branch-a": false, "agent/branch-b": false}
	for _, b := range strings.Split(branchesOut, "\n") {
		b = strings.TrimSpace(b)
		if b == "" {
			continue
		}
		if _, ok := wantBranches[b]; !ok {
			t.Errorf("unexpected branch ref left behind: %q", b)
		}
		wantBranches[b] = true
	}
	for b, seen := range wantBranches {
		if !seen {
			t.Errorf("expected branch ref missing: %q", b)
		}
	}
}

// gitOut runs git, fails on error, and returns trimmed stdout.
func gitOut(t *testing.T, cwd string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s %v: %v", name, args, err)
	}
	return strings.TrimSpace(string(out))
}
