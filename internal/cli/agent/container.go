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
	imageTag     string
	keepOnExit   bool
	mu           sync.Mutex
	containerIDs []string
}

// run launches a single container slot that runs `dx agent claude --loop`.
// Blocks until the container exits. Returns nil if the context was cancelled
// (clean shutdown), or a non-nil error on unexpected failure.
func (m *containerManager) run(ctx context.Context, slot int, alias string, agentCfg config.AgentConfig) error {
	name := fmt.Sprintf("zdx-agent-%s-%d", alias, slot)

	args := []string{"run", "--name", name}
	if !m.keepOnExit {
		args = append(args, "--rm")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}
	args = append(args, "-v", cwd+":/workspace", "-w", "/workspace")

	// Pass through env vars the inner agent needs.
	for _, key := range []string{
		"DX_REMOTE_API_KEY",
		"ANTHROPIC_API_KEY",
		"DATABASE_URL",
		"ZDX_API_URL",
		"NO_COLOR",
	} {
		if val := os.Getenv(key); val != "" {
			args = append(args, "-e", key+"="+val)
		}
	}

	slotAlias := fmt.Sprintf("%s-%d", alias, slot)
	args = append(args, m.imageTag,
		"./bin/dx", "agent", "claude", "--loop",
		"--alias", slotAlias,
	)
	if agentCfg.ClaudeModel != "" {
		args = append(args, "--model", agentCfg.ClaudeModel)
	}
	// Disable chrome inside containers (no browser available).
	args = append(args, "--chrome=false")

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
func runContainerLoop(alias string, agentCfg config.AgentConfig, keepContainer bool) error {
	imageTag, err := buildDevImage()
	if err != nil {
		return err
	}

	mgr := &containerManager{
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
	logf("container mode: image=%s slots=%d keep=%v", imageTag, maxSlots, keepContainer)

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
