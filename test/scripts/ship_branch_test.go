package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Pre-TK-1805, bin/ship enforced "operator's current branch == release_branch"
// as a refusal gate. TK-1805 replaced that check with slot provisioning: ship
// always builds from origin/<release_branch>, so the operator's branch is
// irrelevant. Tests below cover the residual semantics — when no release
// branch is configured, the slot block is skipped and ship proceeds in-place.

func TestShipNoBranchConfigured(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	// No home/deploy.secret.properties — slot is opt-in via release_branch.

	out, _ := repo.runShip(t)
	const pastGateMarker = "[ship] Running lint"
	if !strings.Contains(out, pastGateMarker) {
		t.Fatalf("expected script to proceed past gate (look for %q).\noutput:\n%s", pastGateMarker, out)
	}
	if strings.Contains(out, "Provisioning slot worktree") {
		t.Fatalf("no release_branch should skip slot provisioning.\noutput:\n%s", out)
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
	const pastGateMarker = "[ship] Running lint"
	if !strings.Contains(out, pastGateMarker) {
		t.Fatalf("expected script to proceed past gate (look for %q).\noutput:\n%s", pastGateMarker, out)
	}
	if strings.Contains(out, "Provisioning slot worktree") {
		t.Fatalf("absent release_branch key should skip slot provisioning.\noutput:\n%s", out)
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
