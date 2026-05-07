package mcpcmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestTruncateLongLines(t *testing.T) {
	in := "short\n" + strings.Repeat("x", 1200) + "\nmiddle\n" + strings.Repeat("y", 800) + "\n"
	out, lines, bytes := truncateLongLines(in, 500)
	if lines != 2 {
		t.Errorf("expected 2 truncated lines, got %d", lines)
	}
	if bytes != (1200-500)+(800-500) {
		t.Errorf("expected %d truncated bytes, got %d", (1200-500)+(800-500), bytes)
	}
	if !strings.Contains(out, "[line 2 truncated; original=1200ch]") {
		t.Errorf("missing marker for line 2; got tail: %q", out[:200])
	}
	if !strings.Contains(out, "[line 4 truncated; original=800ch]") {
		t.Errorf("missing marker for line 4; got tail: %q", out[len(out)-300:])
	}
	if !strings.Contains(out, "short\n") || !strings.Contains(out, "middle\n") {
		t.Errorf("short lines should pass through unchanged")
	}
}

func TestReadFile_longLineTruncation(t *testing.T) {
	root := t.TempDir()
	// 600-char single-line "json" — mimics openapi.json shape.
	if err := os.WriteFile(filepath.Join(root, "long.json"), []byte(strings.Repeat("a", 600)), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := setupClient(t, root)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "long.json"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshalStructured(t, res.StructuredContent)
	if got["truncated_lines"] == nil {
		t.Errorf("expected truncated_lines in result, got %v", got)
	}
	content := got["content"].(string)
	if !strings.Contains(content, "[line 1 truncated; original=600ch]") {
		t.Errorf("content missing truncation marker, head: %q", content[:200])
	}
}
