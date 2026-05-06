//go:build docker_e2e

// Real-Docker verification of dx-agent --mcp-stdio reached over `docker exec`.
// Skipped under the default test run; opt in via:
//
//	go test -tags docker_e2e ./cmd/dx-agent/
//
// Requires a working docker daemon. Spins up a vanilla golang:1.25-bookworm
// container with bin/dx-agent bind-mounted, exercises the MCP wire end-to-
// end (ListTools, read_file, run_bash), and asserts container-level shell
// isolation. No prod tokens, no LLM, no zdx-server — just the wire.
package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	verifyImage = "golang:1.25-bookworm"
)

// dockerOK reports whether the local docker daemon is reachable. Skips
// loudly so CI without docker doesn't silently mask coverage gaps.
func dockerOK(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, "docker", "version").Run(); err != nil {
		t.Skipf("docker daemon not reachable: %v", err)
	}
}

// findRepoRoot walks up from the test's CWD until it finds a directory
// containing go.mod. The mcp-stdio test build runs in cmd/dx-agent;
// findRepoRoot lets `bin/dx-agent` mount work regardless of where `go test`
// was invoked from.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := wd
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("could not find repo root from %s", wd)
	return ""
}

// TestMCPStdio_DockerExec_EndToEnd is the real-Docker counterpart to the
// in-process TestMCPStdio_EndToEnd. Same MCP exchange; the subprocess
// runs inside a container reached via `docker exec -i` rather than as a
// local child process. Catches transport-level surprises that the local
// test can't (volume mount permissions, exec encoding, network namespace
// behavior under MCP framing).
func TestMCPStdio_DockerExec_EndToEnd(t *testing.T) {
	dockerOK(t)

	repoRoot := findRepoRoot(t)
	binPath := filepath.Join(repoRoot, "bin", "dx-agent")
	if _, err := os.Stat(binPath); err != nil {
		t.Fatalf("bin/dx-agent not found at %s — run `make build` first: %v", binPath, err)
	}

	slot := "zdx-mcp-e2e-" + filepath.Base(t.TempDir())
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Spin up an idle slot container with bin/dx-agent bind-mounted.
	startArgs := []string{
		"run", "-d", "--rm", "--name", slot,
		"-v", repoRoot + ":/workspace",
		"-w", "/workspace",
		"-v", binPath + ":/usr/local/bin/dx-agent:ro",
		verifyImage, "sleep", "infinity",
	}
	out, err := exec.CommandContext(ctx, "docker", startArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("docker run: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_ = exec.Command("docker", "stop", slot).Run()
	})

	// Connect MCP client via `docker exec -i <slot> dx-agent --mcp-stdio`.
	cmd := exec.CommandContext(ctx,
		"docker", "exec", "-i", slot,
		"/usr/local/bin/dx-agent", "--mcp-stdio", "--mcp-root", "/workspace",
	)
	cmd.Stderr = os.Stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("client connect over docker exec: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string]bool{
		"read_file": false, "write_file": false, "edit_file": false,
		"list_dir": false, "glob": false, "grep": false, "run_bash": false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for n, present := range want {
		if !present {
			t.Errorf("expected tool %q to be registered, was not in ListTools", n)
		}
	}

	// read_file: README.md exists in the workspace bind mount and is
	// visible inside the container.
	readResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "README.md"},
	})
	if err != nil || readResult.IsError {
		t.Fatalf("read_file README.md: err=%v isError=%v", err, readResult != nil && readResult.IsError)
	}
	body := textFromContent(readResult.Content)
	if len(body) == 0 {
		t.Errorf("read_file returned empty body")
	}

	// run_bash hostname: must NOT match host hostname — proves shell
	// isolation across the MCP boundary.
	hostHostname, _ := os.Hostname()
	bashResult, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "run_bash",
		Arguments: map[string]any{"command": "cat /etc/hostname"},
	})
	if err != nil || bashResult.IsError {
		t.Fatalf("run_bash hostname: err=%v isError=%v", err, bashResult != nil && bashResult.IsError)
	}
	bashBody := textFromContent(bashResult.Content)
	if strings.Contains(bashBody, hostHostname) {
		t.Errorf("run_bash hostname matched host (%q) — boundary not isolated; got %q", hostHostname, bashBody)
	}
}

func textFromContent(content []mcp.Content) string {
	var b strings.Builder
	for _, c := range content {
		if tc, ok := c.(*mcp.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}
