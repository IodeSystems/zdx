package mcpcmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestScratchSpillover_smoke(t *testing.T) {
	root := t.TempDir()

	// Create a file with > 2000 lines so read_file truncates
	var big strings.Builder
	for i := 0; i < 3000; i++ {
		big.WriteString("line ")
		big.WriteString(string(rune('a' + (i % 26))))
		big.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(root, "big.txt"), []byte(big.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	RegisterFSTools(srv, root)
	RegisterShellTools(srv, root)
	RegisterOutlineTools(srv, root)

	t1, t2 := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, t1, nil); err != nil {
		t.Fatal(err)
	}
	c := mcp.NewClient(&mcp.Implementation{Name: "tester", Version: "0"}, nil)
	cs, err := c.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatal(err)
	}

	// 1: read_file applies default 2000-line cap
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "read_file",
		Arguments: map[string]any{"path": "big.txt"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := unmarshalStructured(t, res.StructuredContent)
	if got["truncated"] != true {
		t.Errorf("read_file: expected truncated=true, got %v", got["truncated"])
	}
	if n := strings.Count(got["content"].(string), "\n"); n > 2001 {
		t.Errorf("read_file: content has %d newlines, want <=2000", n)
	}

	// 2: run_bash spills to scratch
	res2, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "run_bash",
		Arguments: map[string]any{"command": "yes hi | head -5000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got2 := unmarshalStructured(t, res2.StructuredContent)
	if got2["truncated"] != true {
		t.Errorf("run_bash: expected truncated=true, got %v", got2["truncated"])
	}
	scratch, ok := got2["scratch_path"].(string)
	if !ok || scratch == "" {
		t.Fatalf("run_bash: missing scratch_path: %v", got2)
	}
	if !strings.HasPrefix(scratch, ".zdx/agent/scratch/") {
		t.Errorf("run_bash: scratch_path %q not under .zdx/agent/scratch/", scratch)
	}
	if _, err := os.Stat(filepath.Join(root, scratch)); err != nil {
		t.Errorf("run_bash: scratch file not created: %v", err)
	}
	if hint, _ := got2["hint"].(string); !strings.Contains(hint, "grep") {
		t.Errorf("run_bash: hint missing grep guidance: %q", hint)
	}

	// 3: grep spills when matches exceed limit
	res3, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "grep",
		Arguments: map[string]any{"pattern": "line", "path": "big.txt", "limit": 10},
	})
	if err != nil {
		t.Fatal(err)
	}
	got3 := unmarshalStructured(t, res3.StructuredContent)
	if got3["truncated"] != true {
		t.Errorf("grep: expected truncated=true, got %v", got3["truncated"])
	}
	if scratch3, _ := got3["scratch_path"].(string); !strings.HasPrefix(scratch3, ".zdx/agent/scratch/") {
		t.Errorf("grep: scratch_path %q wrong", scratch3)
	}
}

func unmarshalStructured(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}
