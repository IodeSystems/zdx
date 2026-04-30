package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_ShipRefusesDirtyTree is the demo for spec 57:
// Given a dirty git tree, when bin/ship runs without --allow-dirty, then it
// refuses to deploy and exits with an error.
//
// Three cases:
//  1. Untracked file   — unstaged change makes tree dirty → refused, exit non-zero.
//  2. Staged change    — cached change also counts as dirty → refused.
//  3. Clean tree       — gate passes silently, deploy proceeds.
func TestDemoCLI_ShipRefusesDirtyTree(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_dirty_tree_test.go", Note: "spec 57 demo source"},
		{FilePath: "bin/ship", LineStart: 78, LineEnd: 93, Note: "dirty-tree gate: refuses when git tree has uncommitted changes"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}

	// Use the exact message emitted by the dirty-tree gate (distinct from the
	// must-spec gate which prints "must-spec gate failed").
	const dirtyMarker = "working tree has uncommitted changes"

	// ── Case 1: unstaged change to tracked file → dirty tree → refused ──────────
	// newShipRepo commits "tracked.txt"; modifying it makes the tree dirty via
	// `git diff --quiet` (untracked files do not register with git diff).
	repo1 := newShipRepo(t, root)
	if err := os.WriteFile(filepath.Join(repo1.root, "tracked.txt"), []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("write modified tracked file: %v", err)
	}
	out1, err1 := repo1.runShip(t)
	if err1 == nil {
		t.Fatalf("bin/ship exited 0 with untracked file; want non-zero.\noutput:\n%s", out1)
	}
	if !strings.Contains(out1, dirtyMarker) {
		t.Fatalf("expected %q in output:\n%s", dirtyMarker, out1)
	}

	// ── Case 2: staged (cached) change → dirty tree → refused ─────────────────
	repo2 := newShipRepo(t, root)
	if err := os.WriteFile(filepath.Join(repo2.root, "staged.txt"), []byte("staged\n"), 0o644); err != nil {
		t.Fatalf("write staged file: %v", err)
	}
	repo2.git(t, "add", "staged.txt")
	out2, err2 := repo2.runShip(t)
	if err2 == nil {
		t.Fatalf("bin/ship exited 0 with staged change; want non-zero.\noutput:\n%s", out2)
	}
	if !strings.Contains(out2, dirtyMarker) {
		t.Fatalf("expected %q in staged output:\n%s", dirtyMarker, out2)
	}

	// ── Case 3: clean tree → dirty-tree gate does not fire ────────────────────
	repo3 := newShipRepo(t, root)
	out3, _ := repo3.runShip(t)
	if strings.Contains(out3, dirtyMarker) {
		t.Fatalf("clean tree must not trip dirty gate; %q appeared:\n%s", dirtyMarker, out3)
	}
}
