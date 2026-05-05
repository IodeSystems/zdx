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

// TestMCPStdio_EndToEnd builds the dx-agent binary, spawns it in --mcp-stdio
// mode against a temp workspace, and exercises the MCP surface end-to-end:
// list tools → expect fs+shell, then read_file an actual file. This is the
// proof-of-concept for IS-1032 phase A: the in-container MCP host can be
// reached over stdin/stdout from a parent MCP client without any network
// transport.
func TestMCPStdio_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("end-to-end stdio test is heavy under -short")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("from-the-container\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// Build the binary into a temp path so the test doesn't depend on
	// `make build` having been run, and survives a stale ./bin/dx-agent.
	binPath := filepath.Join(t.TempDir(), "dx-agent")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("go build dx-agent: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "--mcp-stdio", "--mcp-root", root)
	cmd.Stderr = os.Stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	cs, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	want := map[string]bool{
		"read_file":  false,
		"write_file": false,
		"edit_file":  false,
		"list_dir":   false,
		"glob":       false,
		"grep":       false,
		"run_bash":   false,
	}
	for _, tool := range tools.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, present := range want {
		if !present {
			t.Errorf("expected tool %q to be registered, was not in ListTools", name)
		}
	}

	// Round-trip a real read_file call. The path is relative to --mcp-root,
	// so `hello.txt` resolves to <tempdir>/hello.txt inside dx-agent's
	// resolveInRoot — same sandboxing the in-process providers use.
	out, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "hello.txt"},
	})
	if err != nil {
		t.Fatalf("CallTool read_file: %v", err)
	}
	if out.IsError {
		t.Fatalf("read_file reported an error: %+v", out)
	}
	// The content envelope is StructuredContent; the text should appear
	// somewhere in the response — assert via marshalled output to avoid
	// hard-coupling to internal field names that may evolve.
	body := ""
	for _, c := range out.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			body += tc.Text
		}
	}
	if !strings.Contains(body, "from-the-container") {
		t.Errorf("read_file content missing expected payload; got %q", body)
	}
}
