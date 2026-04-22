// Package scripts holds tests for the bash scripts under bin/.
//
// These are standalone tests: no testserver, no database. They exec bin/ship
// in a temp git repo so they can assert the dirty-tree guard (spec 57)
// without contaminating the real working tree.
package scripts

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const refusalMarker = "[ship] REFUSING TO SHIP"

func TestShipRefusesDirtyTree(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.dirty(t)

	out, err := repo.runShip(t)
	if err == nil {
		t.Fatalf("bin/ship exited 0 with dirty tree; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, refusalMarker) {
		t.Fatalf("output missing %q.\noutput:\n%s", refusalMarker, out)
	}
}

func TestShipAllowDirtyBypassesGate(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.dirty(t)

	out, _ := repo.runShip(t, "--allow-dirty")
	if strings.Contains(out, refusalMarker) {
		t.Fatalf("--allow-dirty should skip the gate, but %q appeared.\noutput:\n%s", refusalMarker, out)
	}
}

func TestShipCleanTreePasses(t *testing.T) {
	repo := newTempShipRepo(t)

	out, _ := repo.runShip(t)
	if strings.Contains(out, refusalMarker) {
		t.Fatalf("clean tree should not refuse; %q appeared.\noutput:\n%s", refusalMarker, out)
	}
}

type shipRepo struct {
	root   string
	ship   string
	env    []string
	source string
}

func newTempShipRepo(t *testing.T) *shipRepo {
	t.Helper()

	sourceShip := filepath.Join(findRepoRoot(t), "bin", "ship")
	if _, err := os.Stat(sourceShip); err != nil {
		t.Fatalf("source bin/ship not found: %v", err)
	}

	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	shipDst := filepath.Join(binDir, "ship")
	copyFile(t, sourceShip, shipDst, 0o755)

	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=ship-test",
		"GIT_AUTHOR_EMAIL=ship-test@example.invalid",
		"GIT_COMMITTER_NAME=ship-test",
		"GIT_COMMITTER_EMAIL=ship-test@example.invalid",
	)

	repo := &shipRepo{root: root, ship: shipDst, env: env, source: sourceShip}
	repo.runGit(t, "init", "-q", "-b", "main")
	repo.runGit(t, "config", "commit.gpgsign", "false")

	seed := filepath.Join(root, "tracked.txt")
	if err := os.WriteFile(seed, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	repo.runGit(t, "add", "tracked.txt")
	repo.runGit(t, "commit", "-q", "-m", "seed")

	return repo
}

func (r *shipRepo) dirty(t *testing.T) {
	t.Helper()
	seed := filepath.Join(r.root, "tracked.txt")
	if err := os.WriteFile(seed, []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("dirty tracked file: %v", err)
	}
}

func (r *shipRepo) runShip(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{r.ship}, args...)...)
	cmd.Dir = r.root
	cmd.Env = r.env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func (r *shipRepo) runGit(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.root
	cmd.Env = r.env
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
}

func copyFile(t *testing.T, src, dst string, mode os.FileMode) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, mode); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}
