package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_ShipBranchGate is the demo for spec 146 post-TK-1805.
//
// Pre-TK-1805 the gate was: "operator's current branch must equal the
// configured release_branch, or ship refuses." TK-1805 supersedes that with
// slot provisioning: ship always provisions a fresh worktree from
// origin/<release_branch> and re-execs there, so the operator's branch is
// no longer load-bearing — the canonical branch is what gets shipped by
// construction.
//
// Three cases:
//  1. Operator on a feature branch + release_branch configured → ship still
//     provisions the slot from origin/<release_branch>; slot HEAD matches the
//     canonical branch SHA, not the operator's local HEAD.
//  2. Operator on the configured release branch → same; slot still provisions
//     from origin/<release_branch>.
//  3. No release_branch configured → slot is opt-in via the env config; ship
//     proceeds in-place without provisioning.
func TestDemoCLI_ShipBranchGate(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_branch_gate_test.go", Note: "spec 146 demo source"},
		{FilePath: "bin/ship", LineStart: 161, LineEnd: 255, Note: "TK-1805 slot provisioning"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}

	const provisioningMarker = "[ship] Provisioning slot worktree"

	// ── Case 1: feature branch + release_branch=main → ships origin/main ─────
	repo1 := newShipRepo(t, root)
	repo1.addOrigin(t)
	repo1.git(t, "checkout", "-q", "-b", "feature/my-work")
	writeReleaseBranchProps(t, repo1.root, "main")

	out1, err := repo1.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("--print-slot-path failed (case 1): %v\noutput:\n%s", err, out1)
	}
	if !strings.Contains(out1, provisioningMarker) {
		t.Fatalf("expected slot provisioning on feature branch.\noutput:\n%s", out1)
	}
	slot1 := lastTrimmedLine(out1)
	originSHA := strings.TrimSpace(gitCapture(t, repo1.root, repo1.env, "rev-parse", "origin/main"))
	slotSHA := strings.TrimSpace(gitCapture(t, slot1, repo1.env, "rev-parse", "HEAD"))
	if slotSHA != originSHA {
		t.Fatalf("slot HEAD %q must match origin/main %q (operator branch is irrelevant post-TK-1805)", slotSHA, originSHA)
	}

	// ── Case 2: on release branch → same slot semantics ──────────────────────
	repo2 := newShipRepo(t, root)
	repo2.addOrigin(t)
	writeReleaseBranchProps(t, repo2.root, "main")

	out2, err := repo2.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("--print-slot-path failed (case 2): %v\noutput:\n%s", err, out2)
	}
	if !strings.Contains(out2, provisioningMarker) {
		t.Fatalf("expected slot provisioning on release branch.\noutput:\n%s", out2)
	}

	// ── Case 3: no deploy.secret.properties → slot is skipped ────────────────
	repo3 := newShipRepo(t, root)
	repo3.git(t, "checkout", "-q", "-b", "any-branch")
	// No properties file, no addOrigin — slot must not be provisioned.

	out3, _ := repo3.runShip(t)
	if strings.Contains(out3, provisioningMarker) {
		t.Fatalf("missing config must skip slot provisioning.\noutput:\n%s", out3)
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

func lastTrimmedLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" {
			return l
		}
	}
	return ""
}

func gitCapture(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, out.String())
	}
	return out.String()
}
