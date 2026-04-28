package e2e

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_AgentContainerResourceLimits is the demo for spec 120: container
// resource limits (CPU, memory) are enforced so a runaway agent cannot starve
// the host or sibling agents. A real Docker container is launched with the
// same --memory and --cpus flags the agent harness applies in
// buildContainerArgs; the test then verifies enforcement at two layers:
//
//  1. Daemon: `docker inspect` reports HostConfig.Memory and HostConfig.NanoCpus
//     equal to the requested limits.
//  2. Kernel: cgroup files inside the running container reflect those limits,
//     proving the kernel — not just the docker daemon — is bounding the
//     container.
func TestDemoCLI_AgentContainerResourceLimits(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skipf("docker not available: %v", err)
	}

	rec := newDockerRecorder(t)
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_agent_resource_limits_test.go", Note: "agent container resource-limit demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/container.go", LineStart: 122, LineEnd: 128, Note: "buildContainerArgs adds --memory and --cpus from AgentConfig"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/container.go", LineStart: 174, LineEnd: 181, Note: "runContainerLoop applies 4g/2 defaults so srcless mode never runs unbounded"})
	t.Cleanup(rec.Save)

	const (
		image       = "alpine:3.20"
		memoryFlag  = "64m"
		memoryBytes = 64 * 1024 * 1024
		cpusFlag    = "0.5"
		nanoCPUs    = 500_000_000
		cpuQuotaUs  = 50_000 // 0.5 cpu = 50000us / 100000us period
	)

	name := fmt.Sprintf("zdx-spec120-demo-%d", time.Now().UnixNano())

	// Mirror the resource-limit flags from buildContainerArgs. Run a short
	// sleep so we have a live container to exec into for the kernel check.
	createArgs := []string{
		"create",
		"--name", name,
		"--memory", memoryFlag,
		"--cpus", cpusFlag,
		image,
		"sleep", "30",
	}
	if out, err := rec.Run(createArgs...); err != nil {
		t.Fatalf("docker create: %s: %v", strings.TrimSpace(string(out)), err)
	}
	defer func() {
		_ = exec.Command("docker", "rm", "-f", name).Run()
	}()

	// 1) Daemon-level: docker inspect reports the limits we asked for.
	inspectOut, err := rec.Run("inspect", "--format", "{{.HostConfig.Memory}}/{{.HostConfig.NanoCpus}}", name)
	if err != nil {
		t.Fatalf("docker inspect: %s: %v", strings.TrimSpace(string(inspectOut)), err)
	}
	got := strings.TrimSpace(string(inspectOut))
	want := fmt.Sprintf("%d/%d", memoryBytes, nanoCPUs)
	if got != want {
		t.Errorf("docker inspect HostConfig limits = %q, want %q", got, want)
	}

	// 2) Kernel-level: start the container and read the cgroup limits from
	// inside. cgroup v2 exposes memory.max / cpu.max under /sys/fs/cgroup;
	// cgroup v1 exposes memory.limit_in_bytes / cpu.cfs_quota_us under the
	// per-controller subdirs. The test accepts either layout.
	if out, err := rec.Run("start", name); err != nil {
		t.Fatalf("docker start: %s: %v", strings.TrimSpace(string(out)), err)
	}

	memOut, err := rec.Run("exec", name, "sh", "-c",
		"cat /sys/fs/cgroup/memory.max 2>/dev/null || cat /sys/fs/cgroup/memory/memory.limit_in_bytes 2>/dev/null")
	if err != nil {
		t.Fatalf("docker exec memory cgroup: %s: %v", strings.TrimSpace(string(memOut)), err)
	}
	if mem := strings.TrimSpace(string(memOut)); mem != strconv.Itoa(memoryBytes) {
		t.Errorf("kernel cgroup memory limit = %q, want %q (%s)", mem, strconv.Itoa(memoryBytes), memoryFlag)
	}

	cpuOut, err := rec.Run("exec", name, "sh", "-c",
		"cat /sys/fs/cgroup/cpu.max 2>/dev/null || cat /sys/fs/cgroup/cpu/cpu.cfs_quota_us 2>/dev/null")
	if err != nil {
		t.Fatalf("docker exec cpu cgroup: %s: %v", strings.TrimSpace(string(cpuOut)), err)
	}
	cpu := strings.TrimSpace(string(cpuOut))
	wantQuota := strconv.Itoa(cpuQuotaUs)
	// v2: "<quota> <period>" — accept any period, only verify quota.
	// v1: "<quota>" alone.
	cpuQuota := cpu
	if i := strings.IndexByte(cpu, ' '); i >= 0 {
		cpuQuota = cpu[:i]
	}
	if cpuQuota != wantQuota {
		t.Errorf("kernel cgroup cpu quota = %q (raw %q), want %q (0.5 cpu)", cpuQuota, cpu, wantQuota)
	}
}
