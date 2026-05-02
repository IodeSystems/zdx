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

func TestShipEnvLoadsNamedPropsFile(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	repo.writeNamedReleaseBranch(t, "staging", "staging-rel")

	out, err := repo.runShip(t, "--env", "staging")
	if err == nil {
		t.Fatalf("bin/ship exited 0 with branch mismatch; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("output missing %q.\noutput:\n%s", branchRefusalMarker, out)
	}
	if !strings.Contains(out, "staging-rel") {
		t.Fatalf("expected staging file's release_branch ('staging-rel') in output, proving named props file was loaded.\noutput:\n%s", out)
	}
}

func TestShipEnvBranchMismatchRefuses(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	repo.writeNamedReleaseBranch(t, "staging", "main")

	out, err := repo.runShip(t, "--env", "staging")
	if err == nil {
		t.Fatalf("bin/ship exited 0 with branch mismatch; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("output missing %q.\noutput:\n%s", branchRefusalMarker, out)
	}
}

func TestShipEnvDefaultUnchanged(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.runGit(t, "checkout", "-q", "-b", "dev")
	// Default props file declares main; staging declares dev.
	// Without --env we should hit the default's branch gate, not staging's.
	repo.writeReleaseBranch(t, "main")
	repo.writeNamedReleaseBranch(t, "staging", "dev")

	out, err := repo.runShip(t)
	if err == nil {
		t.Fatalf("bin/ship exited 0 with default branch mismatch; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, branchRefusalMarker) {
		t.Fatalf("output missing %q.\noutput:\n%s", branchRefusalMarker, out)
	}
	if !strings.Contains(out, "'main'") {
		t.Fatalf("expected default file's release_branch ('main') in output, proving default file was loaded.\noutput:\n%s", out)
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
