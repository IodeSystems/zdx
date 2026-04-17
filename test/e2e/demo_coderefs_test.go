package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// coderef is a single {file_path, line_start, line_end, note} record serialised
// into the sidecar .zdx/demo/coderefs/<test-name>.json that `dx test`'s
// syncTestResults reads after the run to call AttachCodeRefToTest.
type coderef struct {
	FilePath  string `json:"file_path"`
	LineStart int32  `json:"line_start,omitempty"`
	LineEnd   int32  `json:"line_end,omitempty"`
	Note      string `json:"note,omitempty"`
}

// writeDemoCoderefs writes refs to .zdx/demo/coderefs/<testName>.json. It is a
// no-op on I/O error (logged via t.Logf) — coderef attachment is a best-effort
// enrichment, not a correctness guard.
func writeDemoCoderefs(t *testing.T, testName string, refs []coderef) {
	t.Helper()
	if len(refs) == 0 {
		return
	}
	root, err := findRoot()
	if err != nil {
		t.Logf("coderefs: find root: %v", err)
		return
	}
	dir := filepath.Join(root, ".zdx", "demo", "coderefs")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Logf("coderefs: mkdir: %v", err)
		return
	}
	b, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		t.Logf("coderefs: marshal: %v", err)
		return
	}
	path := filepath.Join(dir, testName+".json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Logf("coderefs: write: %v", err)
	}
}
