package cli

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterShellTools registers shell tools (run_bash) scoped to the given root.
// The command runs with cwd set to root; there is no syscall sandbox — callers
// must enable this only when the MCP client is trusted.
func RegisterShellTools(srv *mcp.Server, root string) {
	type runBashIn struct {
		Command        string `json:"command" jsonschema:"required,bash -lc command to run"`
		TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"hard timeout (default 120)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "run_bash",
		Description: "Run a bash command at the repo root. Returns stdout+stderr and exit code. Intended for trusted agent use only.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in runBashIn) (*mcp.CallToolResult, any, error) {
		if in.Command == "" {
			return nil, nil, fmt.Errorf("command required")
		}
		timeout := time.Duration(in.TimeoutSeconds) * time.Second
		if timeout <= 0 {
			timeout = 120 * time.Second
		}
		cctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		cmd := exec.CommandContext(cctx, "bash", "-lc", in.Command)
		cmd.Dir = root
		var buf bytes.Buffer
		cmd.Stdout = &buf
		cmd.Stderr = &buf
		runErr := cmd.Run()
		exitCode := 0
		if runErr != nil {
			if ee, ok := runErr.(*exec.ExitError); ok {
				exitCode = ee.ExitCode()
			} else {
				exitCode = -1
			}
		}
		return nil, map[string]any{
			"command":   in.Command,
			"exit_code": exitCode,
			"output":    buf.String(),
		}, nil
	})
}
