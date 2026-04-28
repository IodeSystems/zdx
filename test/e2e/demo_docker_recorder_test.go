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

// DockerDemoRecorder wraps docker CLI commands and records each step to
// .zdx/demo/cli/<test-name>.json using the same demoLog/demoStep shape as
// DemoRecorder, so CollectDemoMetadata picks up the artifact.
type DockerDemoRecorder struct {
	t     *testing.T
	steps []demoStep
	refs  []coderef
}

func newDockerRecorder(t *testing.T) *DockerDemoRecorder {
	t.Helper()
	return &DockerDemoRecorder{t: t}
}

// AddCoderef registers a source file to attach to the demo row.
func (r *DockerDemoRecorder) AddCoderef(ref coderef) {
	r.refs = append(r.refs, ref)
}

// Run executes `docker <args...>`, records the step, and returns combined output + error.
func (r *DockerDemoRecorder) Run(args ...string) ([]byte, error) {
	r.t.Helper()
	start := time.Now()
	out, err := exec.Command("docker", args...).CombinedOutput()
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
		Cmd:        "docker " + strings.Join(args, " "),
		Args:       append([]string{"docker"}, args...),
		Stdout:     string(out),
		ExitCode:   code,
		DurationMs: dur,
	})
	return out, err
}

// Save writes the structured log to .zdx/demo/cli/<t.Name()>.json and emits
// a coderefs sidecar. Register via t.Cleanup(rec.Save).
func (r *DockerDemoRecorder) Save() {
	root, err := findRoot()
	if err != nil {
		r.t.Logf("docker demo save: find root: %v", err)
		return
	}
	dir := filepath.Join(root, ".zdx", "demo", "cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Logf("docker demo save: mkdir: %v", err)
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
		r.t.Logf("docker demo save: %v", err)
		return
	}
	r.t.Logf("CLI demo saved → %s", path)
	writeDemoCoderefs(r.t, r.t.Name(), r.refs)
}
