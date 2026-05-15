package scripts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spec 174 (must): bin/ship --env <name> loads home/deploy.<name>.secret.properties
// instead of the default home/deploy.secret.properties. Enables targeting a
// second environment (e.g. staging).
//
// Pre-TK-1805 these tests asserted the loaded file via the operator-tree
// branch-mismatch refusal message. With slot provisioning that gate is gone,
// so we now verify by inspecting the slot provisioning line, which echoes
// `from origin/<release_branch>` — the release_branch comes from whichever
// props file was loaded.

const provisioningMarker = "[ship] Provisioning slot worktree"

func TestShipEnvLoadsNamedPropsFile(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	repo.writeNamedReleaseBranch(t, "staging", "main")

	out, _ := repo.runShip(t, "--env", "staging", "--print-slot-path")
	if !strings.Contains(out, provisioningMarker) {
		t.Fatalf("expected slot provisioning to run with --env=staging.\noutput:\n%s", out)
	}
	if !strings.Contains(out, "from origin/main") {
		t.Fatalf("expected slot to provision from origin/main (release_branch in staging props).\noutput:\n%s", out)
	}
}

func TestShipEnvDefaultUnchanged(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	// Default props declares main; staging would declare a different branch.
	// Without --env we should hit the default's props, slot from origin/main.
	repo.writeReleaseBranch(t, "main")
	repo.writeNamedReleaseBranch(t, "staging", "no-such-branch")

	out, _ := repo.runShip(t, "--print-slot-path")
	if !strings.Contains(out, provisioningMarker) {
		t.Fatalf("expected slot provisioning to run.\noutput:\n%s", out)
	}
	if !strings.Contains(out, "from origin/main") {
		t.Fatalf("expected default file's release_branch ('main') to drive slot provisioning.\noutput:\n%s", out)
	}
	if strings.Contains(out, "from origin/no-such-branch") {
		t.Fatalf("default invocation must not load staging props.\noutput:\n%s", out)
	}
}

func (r *shipRepo) writeNamedReleaseBranch(t *testing.T, env, branch string) {
	t.Helper()
	homeDir := filepath.Join(r.root, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	props := filepath.Join(homeDir, "deploy."+env+".secret.properties")
	if err := os.WriteFile(props, []byte("deploy.release_branch="+branch+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", props, err)
	}
}
