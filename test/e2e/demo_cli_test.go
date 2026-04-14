//go:build demo

package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DemoRecorder runs dx CLI commands against the test server and records each
// interaction to a structured JSON log in .zdx/demo/cli/<name>.json.
type DemoRecorder struct {
	t     *testing.T
	name  string
	dxBin string
	env   []string
	steps []demoStep
}

type demoStep struct {
	Cmd        string   `json:"cmd"`
	Args       []string `json:"args"`
	Stdout     string   `json:"stdout"`
	Stderr     string   `json:"stderr,omitempty"`
	ExitCode   int      `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
}

type demoLog struct {
	Name       string     `json:"name"`
	RecordedAt string     `json:"recorded_at"`
	Steps      []demoStep `json:"steps"`
}

// newRecorder returns a recorder pointed at the test server.
// dxBin must be a path to the compiled dx binary (e.g. "bin/dx").
func newRecorder(t *testing.T, name, dxBin string) *DemoRecorder {
	t.Helper()
	root, err := findRoot()
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	abs := filepath.Join(root, dxBin)
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("dx binary not found at %q — run: go build -o %s ./cmd/dx/", abs, dxBin)
	}
	slug := "demo-" + name
	apiDo(t, "POST", "/api/project",
		map[string]string{"slug": slug, "name": "Demo " + name}, nil)
	return &DemoRecorder{
		t:     t,
		name:  name,
		dxBin: abs,
		env: append(os.Environ(),
			"DX_REMOTE_URL="+srv.URL,
			"DX_REMOTE_API_KEY="+srv.AdminToken,
			"DX_REMOTE_SLUG="+slug,
		),
	}
}

// Run executes a dx subcommand and records the interaction.
func (r *DemoRecorder) Run(args ...string) {
	r.t.Helper()
	cmd := exec.Command(r.dxBin, args...)
	cmd.Env = r.env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start).Milliseconds()

	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}

	r.steps = append(r.steps, demoStep{
		Cmd:        "dx " + strings.Join(args, " "),
		Args:       args,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   code,
		DurationMs: dur,
	})

	if code != 0 {
		r.t.Logf("dx %s → exit %d\n%s", strings.Join(args, " "), code, stderr.String())
	}
}

// Save writes the structured log. Called automatically via t.Cleanup.
func (r *DemoRecorder) Save() {
	root, _ := findRoot()
	dir := filepath.Join(root, ".zdx", "demo", "cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Logf("demo save: mkdir: %v", err)
		return
	}
	out := demoLog{
		Name:       r.name,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:      r.steps,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	path := filepath.Join(dir, r.name+".json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		r.t.Logf("demo save: %v", err)
		return
	}
	r.t.Logf("CLI demo saved → %s", path)
}

// ── Demo tests ────────────────────────────────────────────────────────────────

func TestDemoCLI_ProjectAndIssueFlow(t *testing.T) {
	rec := newRecorder(t, "project-issue-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=First issue", "--context=Added via CLI demo")
	rec.Run("issue", "list")
	rec.Run("todo", "solo")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_TaskFlow(t *testing.T) {
	rec := newRecorder(t, "task-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=Implement feature X")

	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Skip("could not extract issue ID from output")
	}

	rec.Run("todo", "tech", "add", "--issue="+issueID, "--text=Write the implementation")
	rec.Run("todo", "list", "--issue="+issueID)
	rec.Run("todo", "solo")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// extractFirstID pulls the first IS-N or TK-N token from output.
func extractFirstID(output string) string {
	for _, word := range strings.Fields(output) {
		word = strings.Trim(word, ".,;:()")
		if strings.HasPrefix(word, "IS-") || strings.HasPrefix(word, "TK-") {
			return word
		}
	}
	return ""
}
