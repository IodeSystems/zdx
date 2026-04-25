package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_AgentContainerNonRoot is the demo for spec 121: agent containers
// must not run as root and must drop all Linux capabilities to prevent
// privilege escalation. A real Docker container is launched with the same
// security flags that buildContainerArgs applies; the test verifies enforcement
// at two layers:
//
//  1. Daemon: `docker inspect` reports the security options the agent harness requests.
//  2. Kernel: /proc/1/status inside the running container shows UID ≠ 0, all
//     capability sets zeroed (CapEff = 0000000000000000), and NoNewPrivs = 1.
func TestDemoCLI_AgentContainerNonRoot(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not available: %v", err)
	}

	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_agent_nonroot_test.go", Note: "agent non-root + capability-drop demo source"},
		{FilePath: "internal/cli/agent/container.go", LineStart: 117, LineEnd: 120, Note: "buildContainerArgs: --user agent, --cap-drop all, --security-opt no-new-privileges"},
	})

	const (
		image = "alpine:3.20"
		// UID 65534 (nobody) is universally non-root on Alpine and stands in for
		// the "agent" user that dev.Dockerfile defines in production images.
		nonRootUID = "65534"
	)

	name := fmt.Sprintf("zdx-spec121-demo-%d", time.Now().UnixNano())

	// Mirror the security flags from buildContainerArgs.
	createArgs := []string{
		"create",
		"--name", name,
		"--user", nonRootUID,
		"--cap-drop", "all",
		"--security-opt", "no-new-privileges",
		image,
		"sleep", "30",
	}
	if out, err := exec.Command("docker", createArgs...).CombinedOutput(); err != nil {
		t.Fatalf("docker create: %s: %v", strings.TrimSpace(string(out)), err)
	}
	defer func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	}()

	// 1) Daemon-level: docker inspect reports the requested security options.
	secOptOut, err := exec.Command("docker", "inspect",
		"--format", "{{.HostConfig.SecurityOpt}}", name).CombinedOutput()
	if err != nil {
		t.Fatalf("docker inspect SecurityOpt: %s: %v", strings.TrimSpace(string(secOptOut)), err)
	}
	secOpts := strings.TrimSpace(string(secOptOut))
	if !strings.Contains(secOpts, "no-new-privileges") {
		t.Errorf("docker inspect SecurityOpt = %q, want no-new-privileges", secOpts)
	}

	// 2) Kernel-level: start the container and read /proc/1/status from inside.
	if out, err := exec.Command("docker", "start", name).CombinedOutput(); err != nil {
		t.Fatalf("docker start: %s: %v", strings.TrimSpace(string(out)), err)
	}

	// 2a) UID must be non-root.
	uidOut, err := exec.Command("docker", "exec", name, "id", "-u").CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec id -u: %s: %v", strings.TrimSpace(string(uidOut)), err)
	}
	if uid := strings.TrimSpace(string(uidOut)); uid == "0" {
		t.Errorf("container running as root (UID=0); want non-root")
	}

	// 2b) All effective capabilities must be zeroed.
	capCmd := `grep '^CapEff:' /proc/1/status`
	capOut, err := exec.Command("docker", "exec", name, "sh", "-c", capCmd).CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec CapEff: %s: %v", strings.TrimSpace(string(capOut)), err)
	}
	capLine := strings.TrimSpace(string(capOut))
	// CapEff: 0000000000000000
	fields := strings.Fields(capLine)
	if len(fields) < 2 {
		t.Fatalf("unexpected CapEff line: %q", capLine)
	}
	capHex := strings.TrimLeft(fields[1], "0")
	if capHex != "" {
		t.Errorf("CapEff = %q (non-zero), want all capabilities dropped", capLine)
	}

	// 2c) no-new-privileges bit must be set.
	nnpCmd := `grep '^NoNewPrivs:' /proc/1/status`
	nnpOut, err := exec.Command("docker", "exec", name, "sh", "-c", nnpCmd).CombinedOutput()
	if err != nil {
		t.Fatalf("docker exec NoNewPrivs: %s: %v", strings.TrimSpace(string(nnpOut)), err)
	}
	nnpLine := strings.TrimSpace(string(nnpOut))
	nnpFields := strings.Fields(nnpLine)
	if len(nnpFields) < 2 || nnpFields[1] != "1" {
		t.Errorf("NoNewPrivs = %q, want 1", nnpLine)
	}
}
