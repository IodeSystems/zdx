package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMergeTrain_CollidingMigrationsAutoRenumber proves the core merge-train
// contract for concurrent workers: when branch B adds a migration at the same
// NNN as branch A (which has already landed on dev), the merge-train detects
// the collision via `dx migrate renumber --auto`, renames B's files to the
// next free NNN, regenerates all artifacts (schema/shipped.sql, internal/db/,
// api.gen.ts), and fast-forward merges B cleanly.
//
// The test runs in the actual project repo. It creates two solo/* branches,
// runs merge-train twice, then resets dev to its pre-test tip in t.Cleanup.
// Expected runtime: ~10–20 min (two full build+regen+lint cycles).
func TestMergeTrain_CollidingMigrationsAutoRenumber(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}

	dxBin := filepath.Join(root, "bin/dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not found at %q — run: make build", dxBin)
	}
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not available: %v", err)
	}
	// sqlc is required by the merge-train regen step.
	home, _ := os.UserHomeDir()
	sqlcBin := filepath.Join(home, "go/bin/sqlc")
	if _, serr := os.Stat(sqlcBin); serr != nil {
		if _, perr := exec.LookPath("sqlc"); perr != nil {
			t.Skip("sqlc not found — install: go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0")
		}
	}

	// Find the current max migration NNN so we use the very next slot.
	migDir := filepath.Join(root, "internal/migrate/sql")
	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatalf("readdir %s: %v", migDir, err)
	}
	maxNNN := 0
	for _, e := range entries {
		var n int
		if _, err := fmt.Sscanf(e.Name(), "%d_", &n); err == nil && n > maxNNN {
			maxNNN = n
		}
	}
	testNNN := maxNNN + 1 // both branches use this NNN → collision

	// Unique suffix makes names collision-free across concurrent test runs.
	suffix := fmt.Sprintf("%d", time.Now().UnixNano()%1000000)
	branchA := "solo/coll-a-" + suffix
	branchB := "solo/coll-b-" + suffix
	// nameA < nameB alphabetically → A keeps testNNN, B gets renumbered to testNNN+1.
	nameA := "colla" + suffix
	nameB := "collb" + suffix
	tableA := "zdx_tc_" + suffix + "_a"
	tableB := "zdx_tc_" + suffix + "_b"
	queryA := "GetTc" + suffix + "A"
	queryB := "GetTc" + suffix + "B"
	queryFileA := filepath.Join(root, "queries", "test_"+suffix+"_a.sql")
	queryFileB := filepath.Join(root, "queries", "test_"+suffix+"_b.sql")

	// Require a clean working tree — the merge-train checks out branches and
	// git rebase refuses to run with uncommitted changes.
	if dirty := gitOut(t, root, "git", "status", "--porcelain"); dirty != "" {
		t.Skipf("working tree is dirty — commit or stash changes before running this test:\n%s", dirty)
	}

	// Save state for cleanup.
	currentBranch := gitOut(t, root, "git", "rev-parse", "--abbrev-ref", "HEAD")
	devTipBefore := gitOut(t, root, "git", "rev-parse", "dev")

	t.Cleanup(func() {
		exec.Command("git", "-C", root, "rebase", "--abort").Run()
		exec.Command("git", "-C", root, "checkout", "dev").Run()
		exec.Command("git", "-C", root, "reset", "--hard", devTipBefore).Run()
		exec.Command("git", "-C", root, "branch", "-D", branchA).Run()
		exec.Command("git", "-C", root, "branch", "-D", branchB).Run()
		if currentBranch != "HEAD" && currentBranch != "dev" {
			exec.Command("git", "-C", root, "checkout", currentBranch).Run()
		}
	})

	// ── Branch A: migration testNNN_nameA ─────────────────────────────────────
	t.Log("creating branch A with migration", fmt.Sprintf("%03d_%s", testNNN, nameA))
	gitRun(t, root, "git", "checkout", "-b", branchA, "dev")
	mtWriteFile(t, filepath.Join(migDir, fmt.Sprintf("%03d_%s.up.sql", testNNN, nameA)),
		fmt.Sprintf("CREATE TABLE %s (id bigserial primary key, label text not null);\n", tableA))
	mtWriteFile(t, filepath.Join(migDir, fmt.Sprintf("%03d_%s.down.sql", testNNN, nameA)),
		fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", tableA))
	mtWriteFile(t, queryFileA,
		fmt.Sprintf("-- name: %s :one\nSELECT id, label FROM %s WHERE id = $1;\n", queryA, tableA))
	gitRun(t, root, "git", "add",
		filepath.Join(migDir, fmt.Sprintf("%03d_%s.up.sql", testNNN, nameA)),
		filepath.Join(migDir, fmt.Sprintf("%03d_%s.down.sql", testNNN, nameA)),
		queryFileA,
	)
	gitRun(t, root, "git", "commit", "-m", "feat: add test migration "+nameA)
	gitRun(t, root, "git", "checkout", "dev")

	// ── Branch B: same NNN → collision with A ─────────────────────────────────
	t.Log("creating branch B with colliding migration", fmt.Sprintf("%03d_%s", testNNN, nameB))
	gitRun(t, root, "git", "checkout", "-b", branchB, "dev")
	mtWriteFile(t, filepath.Join(migDir, fmt.Sprintf("%03d_%s.up.sql", testNNN, nameB)),
		fmt.Sprintf("CREATE TABLE %s (id bigserial primary key, value text not null);\n", tableB))
	mtWriteFile(t, filepath.Join(migDir, fmt.Sprintf("%03d_%s.down.sql", testNNN, nameB)),
		fmt.Sprintf("DROP TABLE IF EXISTS %s;\n", tableB))
	mtWriteFile(t, queryFileB,
		fmt.Sprintf("-- name: %s :one\nSELECT id, value FROM %s WHERE id = $1;\n", queryB, tableB))
	gitRun(t, root, "git", "add",
		filepath.Join(migDir, fmt.Sprintf("%03d_%s.up.sql", testNNN, nameB)),
		filepath.Join(migDir, fmt.Sprintf("%03d_%s.down.sql", testNNN, nameB)),
		queryFileB,
	)
	gitRun(t, root, "git", "commit", "-m", "feat: add test migration "+nameB)
	gitRun(t, root, "git", "checkout", "dev")

	// ── Merge-train: branch A (no collision) ─────────────────────────────────
	t.Log("running merge-train on branch A…")
	mtRunMergeTrain(t, root, dxBin, branchA)

	devTipAfterA := gitOut(t, root, "git", "rev-parse", "dev")
	if devTipAfterA == devTipBefore {
		t.Fatal("dev tip did not advance after branch A merge")
	}

	// ── Merge-train: branch B (collision → renumber to testNNN+1) ────────────
	t.Log("running merge-train on branch B (collision expected)…")
	mtRunMergeTrain(t, root, dxBin, branchB)

	// ── Assertions ────────────────────────────────────────────────────────────

	// 1. Migration files: A keeps testNNN, B renumbered to testNNN+1.
	upA := fmt.Sprintf("%03d_%s.up.sql", testNNN, nameA)
	upB := fmt.Sprintf("%03d_%s.up.sql", testNNN+1, nameB)
	dnB := fmt.Sprintf("%03d_%s.down.sql", testNNN+1, nameB)
	origUpB := fmt.Sprintf("%03d_%s.up.sql", testNNN, nameB) // must NOT exist

	for _, want := range []string{upA, upB, dnB} {
		if _, err := os.Stat(filepath.Join(migDir, want)); err != nil {
			t.Errorf("expected migration file %s: %v", want, err)
		}
	}
	if _, err := os.Stat(filepath.Join(migDir, origUpB)); !os.IsNotExist(err) {
		t.Errorf("original B migration %s still exists — not renumbered", origUpB)
	}

	// 2. internal/db/*.sql.go contains both generated query functions.
	dbFiles, _ := filepath.Glob(filepath.Join(root, "internal", "db", "*.sql.go"))
	var dbBuf strings.Builder
	for _, f := range dbFiles {
		data, _ := os.ReadFile(f)
		dbBuf.Write(data)
	}
	allDB := dbBuf.String()
	if !strings.Contains(allDB, queryA) {
		t.Errorf("internal/db/*.sql.go missing %s", queryA)
	}
	if !strings.Contains(allDB, queryB) {
		t.Errorf("internal/db/*.sql.go missing %s", queryB)
	}

	// 3. schema/shipped.sql references both tables.
	shipped, err := os.ReadFile(filepath.Join(root, "schema", "shipped.sql"))
	if err != nil {
		t.Fatalf("read schema/shipped.sql: %v", err)
	}
	if !strings.Contains(string(shipped), tableA) {
		t.Errorf("schema/shipped.sql missing table %s", tableA)
	}
	if !strings.Contains(string(shipped), tableB) {
		t.Errorf("schema/shipped.sql missing table %s", tableB)
	}

	// 4. git log dev shows two regen: commits — one per branch.
	devLog := gitOut(t, root, "git", "log", "--format=%s", "dev")
	for _, branch := range []string{branchA, branchB} {
		if !strings.Contains(devLog, "regen: "+branch) {
			t.Errorf("dev log missing 'regen: %s'\n--- log ---\n%s", branch, devLog)
		}
	}
}

// mtRunMergeTrain invokes dx merge-train run for the given branch, logging
// all output and failing t on nonzero exit.
func mtRunMergeTrain(t *testing.T, root, dxBin, branch string) {
	t.Helper()
	cmd := exec.Command(dxBin, "merge-train", "run", branch, "--dry-run=false")
	cmd.Dir = root
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("merge-train run %s: %v\n--- stdout ---\n%s\n--- stderr ---\n%s",
			branch, err, outBuf.String(), errBuf.String())
	}
	t.Logf("merge-train %s stdout:\n%s", branch, outBuf.String())
}

// mtWriteFile writes content to path, failing t on error.
func mtWriteFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
