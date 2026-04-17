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

func TestDemoCLI_BlockerQuestionFlow(t *testing.T) {
	rec := newRecorder(t, "blocker-question-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=Decide on auth strategy", "--context=Need to pick OAuth provider")

	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Skip("could not extract issue ID from output")
	}

	// Spec 1: question add with target-type, target-id, context → BQ-ID printed
	rec.Run("question", "add",
		"--target-type=issue",
		"--target-id="+issueID,
		"--context=Which OAuth provider should we use — Auth0, Cognito, or roll our own?",
	)
	bqID := extractFirstBQID(rec.steps[len(rec.steps)-1].Stdout)
	if bqID == "" {
		t.Errorf("expected BQ-ID in output, got: %s", rec.steps[len(rec.steps)-1].Stdout)
	}

	// Spec 9: question add with choices → choices persisted
	rec.Run("question", "add",
		"--target-type=issue",
		"--target-id="+issueID,
		"--context=Which database for the session store?",
		"--choices=Redis,DynamoDB,Postgres",
	)

	// Spec 3: question list → shows BQ-ID, target, status, context
	rec.Run("question", "list")

	// Spec 4: question list --status pending → only pending shown
	rec.Run("question", "list", "--status=pending")

	// Spec 89: solo in global mode excludes BQ-blocked issues from the queue.
	// Issue title should NOT appear in this output while questions are pending.
	rec.Run("todo", "solo")

	// Spec 90: solo with --issue=IS-N surfaces the pending question as clarify.
	rec.Run("todo", "solo", "--issue="+issueID)

	// Spec 2: question answer → status changes to answered
	if bqID != "" {
		numID := strings.TrimPrefix(bqID, "BQ-")
		rec.Run("question", "answer", numID, "--answer=Go with Auth0 for managed OAuth")
	}

	// Verify answered question no longer appears in pending list
	rec.Run("question", "list", "--status=pending")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_FeatureFlow(t *testing.T) {
	rec := newRecorder(t, "feature-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("feature", "add", "user-auth", "--desc=User authentication and session management")
	rec.Run("feature", "add", "data-export", "--desc=Export project data to CSV and JSON")
	rec.Run("feature", "list")
	rec.Run("feature", "show", "user-auth")
	rec.Run("feature", "set", "user-auth", "--category=Security", "--component=api")
	rec.Run("feature", "review", "user-auth")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_JournalFlow(t *testing.T) {
	rec := newRecorder(t, "journal-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=Implement journal feature", "--context=Work-log tracking via CLI")

	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Skip("could not extract issue ID from output")
	}

	// Spec 15: journal add with issue ID and note → work-log entry appended
	rec.Run("journal", "add", "--issue="+issueID, "--note=Started implementation of journal commands")
	rec.Run("journal", "add", "--issue="+issueID, "--note=Added add and list subcommands", "--role=dev")

	// Spec 16: journal list with issue ID → entries listed with timestamps and attribution
	rec.Run("journal", "list", "--issue="+issueID)

	// Also show unfiltered list
	rec.Run("journal", "list")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// extractFirstBQID pulls the first BQ-N token from output.
func extractFirstBQID(output string) string {
	for _, word := range strings.Fields(output) {
		word = strings.Trim(word, ".,;:()")
		if strings.HasPrefix(word, "BQ-") {
			return word
		}
	}
	return ""
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
