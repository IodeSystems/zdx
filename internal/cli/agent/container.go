package agent

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
