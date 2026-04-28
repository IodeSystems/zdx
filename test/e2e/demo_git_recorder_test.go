package e2e

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// GitDemoRecorder wraps git CLI commands and records each step to
// .zdx/demo/cli/<test-name>.json using the same demoLog/demoStep shape as
// DemoRecorder, so CollectDemoMetadata picks up the artifact.
type GitDemoRecorder struct {
	t     *testing.T
	steps []demoStep
	refs  []coderef
}

func newGitRecorder(t *testing.T) *GitDemoRecorder {
	t.Helper()
	return &GitDemoRecorder{t: t}
}

// AddCoderef registers a source file to attach to the demo row.
func (r *GitDemoRecorder) AddCoderef(ref coderef) {
	r.refs = append(r.refs, ref)
}

// Run executes `git <args...>` in cwd, records the step, and fatals on error.
// cwd="" uses the process working directory.
func (r *GitDemoRecorder) Run(cwd string, args ...string) {
	r.t.Helper()
	out, err := r.exec(cwd, args...)
	if err != nil {
		r.t.Fatalf("git %v: %s: %v", args, strings.TrimSpace(string(out)), err)
	}
}

// RunOutput executes `git <args...>` in cwd, records the step, and returns
// combined output without failing on non-zero exit.
func (r *GitDemoRecorder) RunOutput(cwd string, args ...string) []byte {
	r.t.Helper()
	out, _ := r.exec(cwd, args...)
	return out
}

func (r *GitDemoRecorder) exec(cwd string, args ...string) ([]byte, error) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	start := time.Now()
	out, err := cmd.CombinedOutput()
	dur := time.Since(start).Milliseconds()

	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
		}
	}
	r.steps = append(r.steps, demoStep{
		Cmd:        "git " + strings.Join(args, " "),
		Args:       append([]string{"git"}, args...),
		Stdout:     string(out),
		ExitCode:   code,
		DurationMs: dur,
	})
	return out, err
}

// Save writes the structured log to .zdx/demo/cli/<t.Name()>.json and emits
// a coderefs sidecar. Register via t.Cleanup(rec.Save).
func (r *GitDemoRecorder) Save() {
	root, err := findRoot()
	if err != nil {
		r.t.Logf("git demo save: find root: %v", err)
		return
	}
	dir := filepath.Join(root, ".zdx", "demo", "cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Logf("git demo save: mkdir: %v", err)
		return
	}
	out := demoLog{
		Name:       r.t.Name(),
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:      r.steps,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	path := filepath.Join(dir, r.t.Name()+".json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		r.t.Logf("git demo save: %v", err)
		return
	}
	r.t.Logf("CLI demo saved → %s", path)
	writeDemoCoderefs(r.t, r.t.Name(), r.refs)
}
