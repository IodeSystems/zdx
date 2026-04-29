package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spec 172 (must): bin/ship refuses deploy when current branch does not match
// deploy.release_branch in home/deploy.secret.properties.

const branchRefusalMarker = "does not match deploy.release_branch"

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
