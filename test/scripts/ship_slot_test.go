package scripts

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TK-1805: bin/ship provisions a worktree at $SHIP_SLOT_BASE/<slug>/ship-slot/
// from origin/<release_branch> and re-execs into it. These tests use
// --print-slot-path which exits after provisioning so we can assert the slot
// path, its HEAD, and idempotent recreation without needing the full deploy
// stack inside the slot.

func TestShipPrintSlotPathProvisionsFromOrigin(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	repo.writeReleaseBranch(t, "main")

	out, err := repo.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("--print-slot-path failed: %v\noutput:\n%s", err, out)
	}

	expectedPrefix := filepath.Join(repo.slotBase) + string(filepath.Separator)
	slot := lastNonEmptyLine(out)
	if !strings.HasPrefix(slot, expectedPrefix) {
		t.Fatalf("slot path %q not under %q\noutput:\n%s", slot, expectedPrefix, out)
	}
	if !strings.HasSuffix(slot, string(filepath.Separator)+"ship-slot") {
		t.Fatalf("slot path %q does not end in /ship-slot\noutput:\n%s", slot, out)
	}
	if _, statErr := os.Stat(slot); statErr != nil {
		t.Fatalf("slot dir not created at %s: %v", slot, statErr)
	}

	// HEAD inside the slot must match origin/main from the operator's repo.
	want := strings.TrimSpace(runGitCapture(t, repo.root, repo.env, "rev-parse", "origin/main"))
	got := strings.TrimSpace(runGitCapture(t, slot, repo.env, "rev-parse", "HEAD"))
	if want != got {
		t.Fatalf("slot HEAD %q does not match origin/main %q", got, want)
	}
}

func TestShipSlotIsIdempotentAcrossRuns(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	repo.writeReleaseBranch(t, "main")

	out1, err := repo.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("first --print-slot-path failed: %v\noutput:\n%s", err, out1)
	}
	out2, err := repo.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("second --print-slot-path failed (slot should be recreated, not appended): %v\noutput:\n%s", err, out2)
	}
	// Both runs should print the same slot path — slug + base are stable.
	if lastNonEmptyLine(out1) != lastNonEmptyLine(out2) {
		t.Fatalf("slot path differed across runs:\n  first:  %s\n  second: %s", lastNonEmptyLine(out1), lastNonEmptyLine(out2))
	}
}

func TestShipSlotShipsOriginNotLocalHEAD(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	repo.writeReleaseBranch(t, "main")

	// Operator commits an extra change locally on main without pushing.
	// Slot must come from origin/main, NOT local HEAD.
	extra := filepath.Join(repo.root, "tracked.txt")
	if err := os.WriteFile(extra, []byte("local-only-change\n"), 0o644); err != nil {
		t.Fatalf("write extra: %v", err)
	}
	repo.runGit(t, "add", "tracked.txt")
	repo.runGit(t, "commit", "-q", "-m", "local-only")

	out, err := repo.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("--print-slot-path failed: %v\noutput:\n%s", err, out)
	}
	slot := lastNonEmptyLine(out)

	originSHA := strings.TrimSpace(runGitCapture(t, repo.root, repo.env, "rev-parse", "origin/main"))
	localSHA := strings.TrimSpace(runGitCapture(t, repo.root, repo.env, "rev-parse", "HEAD"))
	slotSHA := strings.TrimSpace(runGitCapture(t, slot, repo.env, "rev-parse", "HEAD"))

	if originSHA == localSHA {
		t.Fatalf("test setup error: local and origin should differ; both at %s", originSHA)
	}
	if slotSHA != originSHA {
		t.Fatalf("slot HEAD should match origin/main (%s), not local HEAD (%s); got %s", originSHA, localSHA, slotSHA)
	}
}

func TestShipSlotCopiesPropsFile(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	repo.writeReleaseBranch(t, "main")

	out, err := repo.runShip(t, "--print-slot-path")
	if err != nil {
		t.Fatalf("--print-slot-path failed: %v\noutput:\n%s", err, out)
	}
	slot := lastNonEmptyLine(out)

	// Properties file is gitignored, so the fresh worktree won't have it
	// from the checkout — bin/ship must copy it explicitly so config-driven
	// gates inside the slot still work.
	propsInSlot := filepath.Join(slot, "home", "deploy.secret.properties")
	if _, err := os.Stat(propsInSlot); err != nil {
		t.Fatalf("expected props file copied to slot at %s: %v", propsInSlot, err)
	}
}

func TestShipNoPackageRequiresExistingSlot(t *testing.T) {
	repo := newTempShipRepo(t)
	repo.addOrigin(t)
	repo.writeReleaseBranch(t, "main")

	// No prior --package run, so no slot exists. --no-package must refuse.
	out, err := repo.runShip(t, "--no-package")
	if err == nil {
		t.Fatalf("--no-package without prior slot should exit non-zero.\noutput:\n%s", out)
	}
	if !strings.Contains(out, "--no-package requires an existing slot") {
		t.Fatalf("expected explicit error about missing slot.\noutput:\n%s", out)
	}
}

// lastNonEmptyLine returns the last non-empty line of s. --print-slot-path
// prints the slot path on its own line, but bin/ship may emit log lines
// before it; tests grab the trailing line as the slot path.
func lastNonEmptyLine(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := strings.TrimSpace(lines[i])
		if l != "" {
			return l
		}
	}
	return ""
}

func runGitCapture(t *testing.T, dir string, env []string, args ...string) string {
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
