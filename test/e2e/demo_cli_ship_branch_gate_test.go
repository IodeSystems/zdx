package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_ShipBranchGate is the demo for spec 146:
// Given bin/ship, when deploying to an environment, then it validates that
// the current git branch matches the environment's configured release branch.
//
// Three cases:
//  1. Branch mismatch — ship refused, exit non-zero, error message emitted.
//  2. Branch match   — gate passes silently, deploy proceeds.
//  3. No config      — gate is opt-in and skipped when no properties file exists.
func TestDemoCLI_ShipBranchGate(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_branch_gate_test.go", Note: "spec 146 demo source"},
		{FilePath: "bin/ship", LineStart: 95, LineEnd: 112, Note: "release-branch gate"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}

	const refusedMarker = "does not match deploy.release_branch"

	// ── Case 1: feature branch vs main release branch → refused ───────────────
	repo1 := newShipRepo(t, root)
	repo1.git(t, "checkout", "-q", "-b", "feature/my-work")
	writeReleaseBranchProps(t, repo1.root, "main")

	out1, err1 := repo1.runShip(t)
	if err1 == nil {
		t.Fatalf("bin/ship exited 0 with branch mismatch; want non-zero.\noutput:\n%s", out1)
	}
	if !strings.Contains(out1, refusedMarker) {
		t.Fatalf("expected %q in output:\n%s", refusedMarker, out1)
	}

	// ── Case 2: on the configured release branch → gate passes ────────────────
	repo2 := newShipRepo(t, root)
	// newShipRepo initialises on main; configure release_branch=main.
	writeReleaseBranchProps(t, repo2.root, "main")

	out2, _ := repo2.runShip(t)
	if strings.Contains(out2, refusedMarker) {
		t.Fatalf("matching branch must not trip gate; %q appeared.\noutput:\n%s", refusedMarker, out2)
	}

	// ── Case 3: no deploy.secret.properties → gate skipped (opt-in) ──────────
	repo3 := newShipRepo(t, root)
	repo3.git(t, "checkout", "-q", "-b", "any-branch")
	// No properties file written; gate must be a no-op.

	out3, _ := repo3.runShip(t)
	if strings.Contains(out3, refusedMarker) {
		t.Fatalf("missing config must skip gate; %q appeared.\noutput:\n%s", refusedMarker, out3)
	}
}

func writeReleaseBranchProps(t *testing.T, repoRoot, branch string) {
	t.Helper()
	homeDir := filepath.Join(repoRoot, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	props := filepath.Join(homeDir, "deploy.secret.properties")
	if err := os.WriteFile(props, []byte("deploy.release_branch="+branch+"\n"), 0o644); err != nil {
		t.Fatalf("write deploy.secret.properties: %v", err)
	}
}
