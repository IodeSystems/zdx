package e2e

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ApiDemoRecorder records HTTP API interactions to .zdx/demo/api/<name>.json.
// Analogous to DemoRecorder but for tests that drive the server via HTTP instead of CLI.
type ApiDemoRecorder struct {
	t     *testing.T
	name  string
	steps []apiDemoStep
	refs  []coderef
}

type apiDemoStep struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	ReqBody    any    `json:"req_body,omitempty"`
	RespStatus int    `json:"resp_status"`
	DurationMs int64  `json:"duration_ms"`
}

type apiDemoLog struct {
	Name       string        `json:"name"`
	RecordedAt string        `json:"recorded_at"`
	Steps      []apiDemoStep `json:"steps"`
}

func newApiRecorder(t *testing.T, name string) *ApiDemoRecorder {
	t.Helper()
	return &ApiDemoRecorder{t: t, name: name}
}

// Do wraps apiDo, records the interaction, and returns the response.
func (r *ApiDemoRecorder) Do(method, path string, body any, out any) *http.Response {
	r.t.Helper()
	start := time.Now()
	resp := apiDo(r.t, method, path, body, out)
	dur := time.Since(start).Milliseconds()
	r.steps = append(r.steps, apiDemoStep{
		Method:     method,
		Path:       path,
		ReqBody:    body,
		RespStatus: resp.StatusCode,
		DurationMs: dur,
	})
	return resp
}

// AddCoderef registers an additional source file to attach to the test row.
func (r *ApiDemoRecorder) AddCoderef(ref coderef) {
	r.refs = append(r.refs, ref)
}

// Save writes the structured log to .zdx/demo/api/<name>.json and emits a
// coderefs sidecar. Called via t.Cleanup.
func (r *ApiDemoRecorder) Save() {
	root, err := findRoot()
	if err != nil {
		r.t.Logf("api demo save: find root: %v", err)
		return
	}
	dir := filepath.Join(root, ".zdx", "demo", "api")
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Logf("api demo save: mkdir: %v", err)
		return
	}
	out := apiDemoLog{
		Name:       r.name,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:      r.steps,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	path := filepath.Join(dir, r.name+".json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		r.t.Logf("api demo save: %v", err)
		return
	}
	r.t.Logf("API demo saved → %s", path)
	writeDemoCoderefs(r.t, r.t.Name(), r.refs)
}
