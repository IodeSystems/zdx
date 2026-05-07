package mcpcmd

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
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
		Description: "Run a bash command at the repo root. Returns stdout+stderr and exit code. When output exceeds the inline cap, the full output is cached under .zdx/agent/scratch/ and the path is returned so you can grep/read_file it instead of rerunning. Intended for trusted agent use only.",
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
		output := buf.String()
		const maxLines = 2000
		const maxLineChars = 500
		lines := strings.Split(output, "\n")
		result := map[string]any{
			"command":   in.Command,
			"exit_code": exitCode,
		}
		// Apply per-line truncation to the inline head; the full untruncated
		// output is what spills to scratch, so the agent can still inspect a
		// single huge line via read_file with explicit offset/limit.
		var head string
		if len(lines) > maxLines {
			head = strings.Join(lines[:maxLines], "\n")
		} else {
			head = output
		}
		head, longLines, longBytes := truncateLongLines(head, maxLineChars)
		if longLines > 0 {
			result["truncated_lines"] = longLines
			result["truncated_line_chars"] = longBytes
		}
		if len(lines) > maxLines {
			result["output"] = head
			result["truncated"] = true
			result["total_lines"] = len(lines)
			if scratchPath, sErr := writeScratch(root, "run", output); sErr == nil {
				result["scratch_path"] = scratchPath
				result["hint"] = fmt.Sprintf("Output truncated to first %d of %d lines. Full untruncated output cached at %s — use the grep tool with path=%q to search it, or read_file with offset/limit to page through, instead of rerunning the command.", maxLines, len(lines), scratchPath, scratchPath)
			} else {
				result["hint"] = fmt.Sprintf("Output truncated to first %d of %d lines. (scratch cache failed: %v)", maxLines, len(lines), sErr)
			}
		} else {
			result["output"] = head
		}
		return nil, result, nil
	})
}
