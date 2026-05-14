package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spec 172 (must): bin/ship refuses deploy when current branch does not match
// the configured release_branch. Source of truth is the server when
// deploy.environment_name is set; otherwise falls back to deploy.release_branch
// in home/deploy.secret.properties (legacy path).

const branchRefusalMarker = "does not match release_branch"

func TestShipRefusesBranchMismatch(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	repo.writeReleaseBranch(t, "main")

	out, err := repo.runShip(t)
	if err == nil {
		t.Fatalf("bin/ship exited 0 with branch mismatch; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("output missing %q.\noutput:\n%s", branchRefusalMarker, out)
	}
}

func TestShipAllowsBranchMatch(t *testing.T) {
	repo := newTempShipRepo(t)
	// newTempShipRepo inits on main; configure release_branch=main.
	repo.writeReleaseBranch(t, "main")

	out, _ := repo.runShip(t)
	if strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("matching branch should not trip the gate; %q appeared.\noutput:\n%s", branchRefusalMarker, out)
	}
}

func TestShipNoBranchConfigured(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	// No home/deploy.secret.properties — gate is opt-in and must skip.

	out, _ := repo.runShip(t)
	if strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("missing config should skip the branch gate; %q appeared.\noutput:\n%s", branchRefusalMarker, out)
	}
}

func TestShipPropsMissingReleaseBranchKey(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	// Properties file exists but does not declare deploy.release_branch.
	homeDir := filepath.Join(repo.root, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	props := filepath.Join(homeDir, "deploy.secret.properties")
	if err := os.WriteFile(props, []byte("deploy.host=ubuntu@example\n"), 0o644); err != nil {
		t.Fatalf("write deploy.secret.properties: %v", err)
	}

	out, _ := repo.runShip(t)
	if strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("absent release_branch key should skip gate; %q appeared.\noutput:\n%s", branchRefusalMarker, out)
	}
	// Without `|| true`, set -o pipefail kills the script silently at the gate.
	// Assert it proceeded past the gate by checking for a marker from the next stage.
	const pastGateMarker = "[ship] Running lint"
	if !strings.Contains(out, pastGateMarker) {
		t.Fatalf("expected script to proceed past gate (look for %q).\noutput:\n%s", pastGateMarker, out)
	}
}

func (r *shipRepo) writeReleaseBranch(t *testing.T, branch string) {
	t.Helper()
	homeDir := filepath.Join(r.root, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	props := filepath.Join(homeDir, "deploy.secret.properties")
	if err := os.WriteFile(props, []byte("deploy.release_branch="+branch+"\n"), 0o644); err != nil {
		t.Fatalf("write deploy.secret.properties: %v", err)
	}
}
