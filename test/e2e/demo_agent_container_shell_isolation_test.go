package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_AgentContainerShellIsolation is the demo for spec 119: shell
// access inside an agent container must see only the worktree at /workspace —
// no host filesystem path outside that bind mount is reachable. Enforcement is
// pure Docker volume isolation: buildContainerArgs adds `-v cwd:/workspace -w
// /workspace` and no further `-v` flags, so the container's mount namespace
// excludes everything else on the host.
//
// The test mirrors that exact flag set against an alpine:3.20 image and
// verifies isolation at two complementary points:
//
//  1. Inside-out: a sentinel file written to a host dir outside the worktree
//     is NOT visible from inside the container (cat returns no-such-file).
//  2. Bind-mount sanity: a marker file inside the worktree IS visible at
//     /workspace, proving the mount itself works and the negative result above
//     is real isolation rather than a broken setup.
func TestDemoCLI_AgentContainerShellIsolation(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not available: %v", err)
	}

	rec := newDockerRecorder(t)
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_agent_container_shell_isolation_test.go", Note: "agent container shell-isolation demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/container.go", LineStart: 129, LineEnd: 129, Note: "buildContainerArgs: -v cwd:/workspace -w /workspace (only mount; host paths invisible)"})
	t.Cleanup(rec.Save)

	const image = "alpine:3.20"

	// Worktree stand-in: the only path the container is allowed to see.
	worktree := t.TempDir()
	const markerName = "workspace-marker.txt"
	const markerBody = "spec-119-worktree-mount-ok"
	if err := os.WriteFile(filepath.Join(worktree, markerName), []byte(markerBody), 0644); err != nil {
		t.Fatalf("write worktree marker: %v", err)
	}

	// Host sentinel: outside the worktree, on the host fs. The container must
	// NOT be able to read this. Using a sibling temp dir guarantees the path
	// exists on the host but is not part of the bind mount.
	hostDir := t.TempDir()
	const sentinelBody = "host-only-secret"
	hostSentinel := filepath.Join(hostDir, "host-secret.txt")
	if err := os.WriteFile(hostSentinel, []byte(sentinelBody), 0644); err != nil {
		t.Fatalf("write host sentinel: %v", err)
	}

	name := fmt.Sprintf("zdx-spec119-demo-%d", time.Now().UnixNano())

	// Mirror the bind-mount + workdir flags from buildContainerArgs. No other
	// -v flags — that is precisely the property under test.
	createArgs := []string{
		"create",
		"--name", name,
		"-v", worktree + ":/workspace",
		"-w", "/workspace",
		image,
		"sleep", "30",
	}
	if out, err := rec.Run(createArgs...); err != nil {
		t.Fatalf("docker create: %s: %v", strings.TrimSpace(string(out)), err)
	}
	defer func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	}()

	if out, err := rec.Run("start", name); err != nil {
		t.Fatalf("docker start: %s: %v", strings.TrimSpace(string(out)), err)
	}

	// 1) Bind-mount sanity: the worktree marker is visible at /workspace.
	markerOut, err := rec.Run("exec", name, "cat", "/workspace/"+markerName)
	if err != nil {
		t.Fatalf("docker exec cat marker: %s: %v", strings.TrimSpace(string(markerOut)), err)
	}
	if got := strings.TrimSpace(string(markerOut)); got != markerBody {
		t.Errorf("worktree marker content = %q, want %q", got, markerBody)
	}

	// 2) Isolation: the host sentinel is not reachable at its host path.
	// `cat` should fail (exit non-zero) and stderr should mention no such file.
	sentOut, err := rec.Run("exec", name, "cat", hostSentinel)
	if err == nil {
		t.Errorf("cat %s inside container succeeded (output=%q); host path must be invisible", hostSentinel, string(sentOut))
	}
	// Even if some path collision returned content, it must not be the host secret.
	if strings.Contains(string(sentOut), sentinelBody) {
		t.Errorf("host sentinel body leaked into container: output=%q", string(sentOut))
	}

	// 3) Isolation, common host paths: the host's /etc/hostname is unrelated
	// to the container's own /etc/hostname. We don't assert on that (every
	// container has its own /etc), but we do assert that arbitrary host
	// directories outside the bind mount — e.g. /tmp paths from the host —
	// are not reachable. The hostDir path is a t.TempDir() under /tmp, which
	// definitely exists on the host; from inside the container that exact
	// path resolves into the container's own (empty) filesystem.
	lsOut, _ := rec.Run("exec", name, "ls", hostDir)
	// We don't fail on err here — `ls` may or may not exit non-zero depending
	// on whether the path exists in the container's namespace. What matters
	// is that the host file is not present.
	if strings.Contains(string(lsOut), "host-secret.txt") {
		t.Errorf("host dir contents leaked into container: ls %s = %q", hostDir, string(lsOut))
	}
}
