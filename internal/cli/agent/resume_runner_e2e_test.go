package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/iodesystems/zdx-go/internal/config"
)

// TestRunResume_ShortCircuitsWithoutClaude exercises RunResume against a
// stubbed httptest server. PATH is cleared so exec.LookPath("claude") fails,
// verifying that RunResume reaches runSession (i.e., Take was called) rather
// than returning a zero TakeResult silently — the regression a no-op consumer
// would produce.
func TestRunResume_ShortCircuitsWithoutClaude(t *testing.T) {
	// Hide claude (and every other binary) so LookPath returns an error.
	t.Setenv("PATH", "")

	// Stub server: returns 200 on all requests.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	// Write a minimal .zdx/config.yaml so loadResumeConfig can read slug + agent config.
	dir := t.TempDir()
	zdxDir := filepath.Join(dir, ".zdx")
	if err := os.MkdirAll(zdxDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		Remote: config.Remote{URL: srv.URL, Slug: "test-slug"},
	}
	data, _ := yaml.Marshal(cfg)
	if err := os.WriteFile(filepath.Join(zdxDir, "config.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// RunLifecycle writes .zdx/logs/ and .zdx/state/ relative to cwd; chdir to
	// the temp dir so those files land there instead of the package directory.
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	res := RunResume(ctx, ResumeRunnerConfig{
		ServerURL: srv.URL,
		APIKey:    "test-key",
		AgentID:   "test-agent",
		WorkDir:   dir,
		SessionID: "prev-session-id",
		IssueID:   "IS-test",
		LogFn:     func(string, ...any) {},
	})

	// With no claude binary, runSession fails at LookPath — so res.Err must
	// be non-nil. A nil error signals Take was never called (wiring regression).
	if res.Err == nil {
		t.Error("expected error (claude not in PATH), got nil — RunResume may not be calling Take")
	}
	if res.Err != nil && !strings.Contains(res.Err.Error(), "claude") {
		t.Errorf("expected error to mention 'claude', got: %v", res.Err)
	}
}
