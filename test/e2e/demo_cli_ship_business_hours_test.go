package e2e

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_ShipBlocksNonCompatMigrationDuringBusinessHours is the demo for
// spec 58: given incompatible migrations during business hours, when bin/ship
// runs in production, then it blocks the deploy until off-hours.
//
// The demo uses an isolated temp repo (clean git tree, stubbed bin/lint) to
// exercise bin/ship without touching infrastructure. SHIP_BETA=false activates
// the gate; NOW_DOW and NOW_HOUR override the clock.
func TestDemoCLI_ShipBlocksNonCompatMigrationDuringBusinessHours(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_business_hours_test.go", Note: "demo source"},
		{FilePath: "bin/ship", LineStart: 67, LineEnd: 74, Note: "business-hours gate"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}

	const blockedMarker = "blocked during business hours"

	// ── Case 1: weekday business hours → gate fires, exit non-zero ────────────
	repo := newShipRepo(t, root)
	repo.env = append(repo.env, "SHIP_BETA=false", "NOW_DOW=3", "NOW_HOUR=10")
	out, err := repo.runShip(t, "--non-compatible-migration")
	if err == nil {
		t.Fatalf("bin/ship exited 0 during business hours; want non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, blockedMarker) {
		t.Fatalf("expected %q in output:\n%s", blockedMarker, out)
	}

	// ── Case 2: weekday after hours → gate silent, deploy proceeds past gate ──
	repo2 := newShipRepo(t, root)
	repo2.env = append(repo2.env, "SHIP_BETA=false", "NOW_DOW=3", "NOW_HOUR=20")
	out2, _ := repo2.runShip(t, "--non-compatible-migration")
	if strings.Contains(out2, blockedMarker) {
		t.Fatalf("gate must not fire after hours; %q appeared:\n%s", blockedMarker, out2)
	}
}

// shipRepo is a minimal isolated git repo for exercising bin/ship without
// touching real infrastructure. bin/lint is stubbed to always pass.
type shipRepo struct {
	root string
	ship string
	env  []string
}

func newShipRepo(t *testing.T, repoRoot string) *shipRepo {
	t.Helper()

	sourceShip := filepath.Join(repoRoot, "bin", "ship")
	if _, err := os.Stat(sourceShip); err != nil {
		t.Fatalf("bin/ship not found: %v", err)
	}

	tmp := t.TempDir()
	binDir := filepath.Join(tmp, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	// Copy bin/ship into the isolated repo.
	shipDst := filepath.Join(binDir, "ship")
	shipSrc, err := os.ReadFile(sourceShip)
	if err != nil {
		t.Fatalf("read bin/ship: %v", err)
	}
	if err := os.WriteFile(shipDst, shipSrc, 0o755); err != nil {
		t.Fatalf("write ship: %v", err)
	}

	// Stub bin/lint so the lint gate passes in the isolated repo.
	if err := os.WriteFile(filepath.Join(binDir, "lint"), []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write lint stub: %v", err)
	}

	path, _ := exec.LookPath("bash")
	if path == "" {
		path = "/usr/bin:/bin"
	} else {
		path = filepath.Dir(path) + ":/usr/bin:/bin"
	}
	env := []string{
		"PATH=" + path + ":/usr/local/bin",
		"HOME=" + os.Getenv("HOME"),
		"GIT_AUTHOR_NAME=ship-test",
		"GIT_AUTHOR_EMAIL=ship-test@example.invalid",
		"GIT_COMMITTER_NAME=ship-test",
		"GIT_COMMITTER_EMAIL=ship-test@example.invalid",
	}

	r := &shipRepo{root: tmp, ship: shipDst, env: env}
	r.git(t, "init", "-q", "-b", "main")
	r.git(t, "config", "commit.gpgsign", "false")

	// Seed a clean committed file so the dirty-tree gate does not fire.
	seed := filepath.Join(tmp, "tracked.txt")
	if err := os.WriteFile(seed, []byte("seed\n"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	r.git(t, "add", "tracked.txt")
	r.git(t, "commit", "-q", "-m", "seed")

	return r
}

func (r *shipRepo) runShip(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(r.ship, args...)
	cmd.Dir = r.root
	cmd.Env = r.env
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}

func (r *shipRepo) git(t *testing.T, args ...string) {
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
