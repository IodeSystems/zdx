package mcpcmd

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Sanity check on real source — verify both outline tools produce reasonable
// payload sizes (much smaller than read_file) on big repo files. Skips
// gracefully if the files have moved.
func TestOutline_realworld_sizes(t *testing.T) {
	repoRoot, err := findRepoRoot()
	if err != nil {
		t.Skip(err)
	}

	cs := setupClient(t, repoRoot)
	for _, c := range []struct {
		path string
	}{
		{"internal/cli/work/todo.go"},
		{"internal/cli/agent/opencode.go"},
		{"ui/src/api/index.ts"},
	} {
		t.Run("outline_"+c.path, func(t *testing.T) {
			full := filepath.Join(repoRoot, c.path)
			fi, err := os.Stat(full)
			if err != nil {
				t.Skipf("source missing: %v", err)
			}
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
				Name:      "outline",
				Arguments: map[string]any{"path": c.path, "lod": 2},
			})
			if err != nil {
				t.Fatal(err)
			}
			b, _ := json.Marshal(res.StructuredContent)
			t.Logf("outline on %s (%d bytes raw): outline %d bytes (%.0f%% of raw)",
				c.path, fi.Size(), len(b), 100*float64(len(b))/float64(fi.Size()))
			if len(b) >= int(fi.Size()) {
				t.Errorf("outline (%d bytes) is not smaller than raw file (%d bytes)", len(b), fi.Size())
			}
			// Sanity: payload must include at least one decl/import.
			var got map[string]any
			_ = json.Unmarshal(b, &got)
			files, _ := got["files"].([]any)
			if len(files) != 1 {
				t.Fatalf("expected 1 file, got %d", len(files))
			}
			f0 := files[0].(map[string]any)
			decls, _ := f0["decls"].([]any)
			if len(decls) == 0 {
				t.Errorf("no decls returned for %s", c.path)
			}
		})
	}
}

func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	cur := wd
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(cur, "go.mod")); err == nil {
			return cur, nil
		}
		next := filepath.Dir(cur)
		if next == cur {
			break
		}
		cur = next
	}
	return "", os.ErrNotExist
}
