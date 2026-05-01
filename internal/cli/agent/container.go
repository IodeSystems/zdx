package agent

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// buildDevImage builds dev.Dockerfile and returns a deterministic image tag
// based on the Dockerfile content hash. Skips the build if the tag already
// exists in local docker image storage.
func buildDevImage() (string, error) {
	const dockerfile = "dev.Dockerfile"
	data, err := os.ReadFile(dockerfile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("dev.Dockerfile not found — run 'dx doctor --fix' to scaffold it")
		}
		return "", fmt.Errorf("read dev.Dockerfile: %w", err)
	}
	h := sha256.Sum256(data)
	tag := fmt.Sprintf("zdx-agent:%x", h[:8])

	// Check if image already exists (non-empty JSON array from inspect).
	out, err := exec.Command("docker", "image", "inspect", tag).Output()
	if err == nil && len(strings.TrimSpace(string(out))) > 2 {
		fmt.Fprintf(os.Stderr, "container: image %s already up-to-date\n", tag)
		return tag, nil
	}

	fmt.Fprintf(os.Stderr, "container: building image %s from dev.Dockerfile...\n", tag)
	cmd := exec.Command("docker", "build", "-f", dockerfile, "-t", tag, ".")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker build: %w", err)
	}
	fmt.Fprintf(os.Stderr, "container: image %s built\n", tag)
	return tag, nil
}

// containerManager tracks running container names for cleanup.
type containerManager struct {
	rc           remoteConfig
	imageTag     string
	keepOnExit   bool
	mu           sync.Mutex
	containerIDs []string
}

// run launches a single container slot that runs `dx agent claude --loop`.
// Blocks until the container exits. Returns nil if the context was cancelled
// (clean shutdown), or a non-nil error on unexpected failure.
func (m *containerManager) run(ctx context.Context, slot int, alias string, agentCfg config.AgentConfig) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	name := fmt.Sprintf("zdx-agent-%s-%d", alias, slot)

	token, tokenID, err := mintScopedToken(ctx, m.rc, fmt.Sprintf("agent-container-%s-%d", alias, slot))
	if err != nil {
		return fmt.Errorf("container slot %d: %w", slot, err)
	}
	defer revokeScopedToken(context.Background(), m.rc, tokenID)

	envPairs := []string{"DX_REMOTE_API_KEY=" + token}
	if m.rc.url != "" {
		envPairs = append(envPairs, "ZDX_API_URL="+m.rc.url)
	}
	envPairs = append(envPairs, collectContainerEnv([]string{"ANTHROPIC_API_KEY", "DATABASE_URL", "NO_COLOR"})...)

	args := buildContainerArgs(name, m.imageTag, cwd, slot, alias, agentCfg, m.keepOnExit, envPairs)

	m.mu.Lock()
	m.containerIDs = append(m.containerIDs, name)
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.containerIDs = removeString(m.containerIDs, name)
		m.mu.Unlock()
	}()

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// stopAll sends `docker stop` to every tracked container. Best-effort; errors
// are printed to stderr but do not stop the sweep.
func (m *containerManager) stopAll() {
	m.mu.Lock()
	ids := append([]string(nil), m.containerIDs...)
	m.mu.Unlock()

	for _, id := range ids {
		out, err := exec.Command("docker", "stop", id).CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "container: stop %s: %s\n", id, strings.TrimSpace(string(out)))
		}
	}
}

// enforceContainerExecution gates the host-process agent path behind a
// dev-only escape hatch. Spec 117 requires every agent session to run inside
// a Docker container. When DX_AGENT_FORCE_CONTAINER is set (e.g. in srcless
// agent images, CI workers, or any operator who wants the spec enforced),
// invoking the agent without --container is a hard error: the host path is
// not allowed to spawn claude. When unset, the host path remains available
// for local dev so this rollout is opt-in until the default flips.
func enforceContainerExecution(container bool) error {
	if !container && os.Getenv("DX_AGENT_FORCE_CONTAINER") != "" {
		return fmt.Errorf("spec-117: agent must run inside a Docker container; pass --container or unset DX_AGENT_FORCE_CONTAINER")
	}
	return nil
}

// buildContainerArgs constructs the `docker run` argv for an agent slot.
// Pure function so the security-critical flag set is unit-testable.
func buildContainerArgs(name, imageTag, cwd string, slot int, alias string, agentCfg config.AgentConfig, keepOnExit bool, envPairs []string) []string {
	args := []string{"run", "--name", name}
	if !keepOnExit {
		args = append(args, "--rm")
	}
	args = append(args, "-v", cwd+":/workspace", "-w", "/workspace")

	// Non-root execution with no privilege escalation (spec-121).
	args = append(args, "--user", "agent")
	args = append(args, "--cap-drop", "all")
	args = append(args, "--security-opt", "no-new-privileges")

	// Resource limits — prevent a runaway agent from starving the host or sibling slots (spec-120).
	if agentCfg.ContainerMemory != "" {
		args = append(args, "--memory", agentCfg.ContainerMemory)
	}
	if agentCfg.ContainerCPUs != "" {
		args = append(args, "--cpus", agentCfg.ContainerCPUs)
	}

	for _, kv := range envPairs {
		args = append(args, "-e", kv)
	}

	slotAlias := fmt.Sprintf("%s-%d", alias, slot)
	args = append(args, imageTag,
		"./bin/dx", "agent", "claude", "--loop",
		"--alias", slotAlias,
	)
	if agentCfg.ClaudeModel != "" {
		args = append(args, "--model", agentCfg.ClaudeModel)
	}
	// Disable chrome inside containers (no browser available).
	args = append(args, "--chrome=false")
	return args
}

// collectContainerEnv returns KEY=VAL pairs for every key with a non-empty value in the host env.
func collectContainerEnv(keys []string) []string {
	var out []string
	for _, key := range keys {
		if val := os.Getenv(key); val != "" {
			out = append(out, key+"="+val)
		}
	}
	return out
}

func removeString(ss []string, s string) []string {
	out := ss[:0]
	for _, v := range ss {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}

// runContainerLoop builds the dev image and runs up to agentCfg.MaxWorktrees
// containers in parallel, each running `dx agent claude --loop`. Containers
// are restarted on unexpected exit. SIGINT/SIGTERM stops all containers and
// exits cleanly.
func runContainerLoop(rc remoteConfig, alias string, agentCfg config.AgentConfig, keepContainer bool) error {
	if rc.slug == "" {
		return fmt.Errorf("--container requires a project config with a remote slug")
	}
	// Apply container resource defaults — srcless mode constructs AgentConfig
	// directly and skips ResolvedAgent, so we normalize here too.
	if agentCfg.ContainerMemory == "" {
		agentCfg.ContainerMemory = "4g"
	}
	if agentCfg.ContainerCPUs == "" {
		agentCfg.ContainerCPUs = "2"
	}

	imageTag, err := buildDevImage()
	if err != nil {
		return err
	}

	mgr := &containerManager{
		rc:         rc,
		imageTag:   imageTag,
		keepOnExit: keepContainer,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logf := func(format string, args ...any) {
		fmt.Printf("[%s] "+format+"\n",
			append([]any{time.Now().Format(time.RFC3339)}, args...)...)
	}

	// Signal handling: stop containers then exit.
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logf("received signal %s: stopping containers...", sig)
		cancel()
		mgr.stopAll()
		select {
		case <-time.After(10 * time.Second):
		case <-sigCh:
		}
		os.Exit(130)
	}()

	maxSlots := agentCfg.MaxWorktrees
	if maxSlots <= 0 {
		maxSlots = 1
	}
	logf("container mode: image=%s slots=%d keep=%v memory=%s cpus=%s",
		imageTag, maxSlots, keepContainer, agentCfg.ContainerMemory, agentCfg.ContainerCPUs)

	var wg sync.WaitGroup
	for i := 0; i < maxSlots; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				logf("container slot %d: starting", slot)
				err := mgr.run(ctx, slot, alias, agentCfg)
				if ctx.Err() != nil {
					return
				}
				if err != nil {
					logf("container slot %d exited: %v; restarting in 10s", slot, err)
				} else {
					logf("container slot %d exited cleanly; restarting in 10s", slot)
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(10 * time.Second):
				}
			}
		}(i)
	}
	wg.Wait()
	return nil
}
