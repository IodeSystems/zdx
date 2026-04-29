package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_DoctorClassificationPrompt is the demo for spec 130:
// Given a project without a classification set, when dx doctor runs
// interactively, the user is prompted to choose one of
// library/tool/service/saas/site and the selection is persisted to the server.
//
// The demo creates a project on the test server with no classification, runs
// dx doctor with "tool\n" piped on stdin, asserts the prompt menu and choice
// line are emitted, and then queries /api/dx/project/info to confirm the
// classification was persisted.
func TestDemoCLI_DoctorClassificationPrompt(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_classification_test.go", Note: "demo source (spec 130)"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 47, LineEnd: 62, Note: "runDoctor: prompt classification when empty, persist to server"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 349, LineEnd: 381, Note: "promptClassification: menu of library/tool/service/saas/site"},
		{FilePath: "internal/server/handlers/handlers_doctor.go", LineStart: 48, LineEnd: 67, Note: "POST /api/dx/doctor/classify persists selection"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	const slug = "demo-doctor-classify-prompt"

	// Create the project with no classification — this is the gap under test.
	apiDo(t, http.MethodPost, "/api/project", map[string]string{
		"slug": slug,
		"name": "Demo Doctor Classification Prompt",
	}, nil)

	// Pre-condition: classification is empty.
	var infoBefore struct {
		Classification string `json:"classification"`
	}
	apiDo(t, http.MethodGet, "/api/dx/project/info?slug="+slug, nil, &infoBefore)
	if infoBefore.Classification != "" {
		t.Fatalf("pre-condition: classification should be empty, got %q", infoBefore.Classification)
	}

	// Doctor requires .zdx/credentials on disk; env var auth alone does not
	// satisfy DetectLocal's file check. Scaffold a temp project dir wired to
	// the test server.
	tmp := t.TempDir()
	zdxDir := filepath.Join(tmp, ".zdx")
	if err := os.MkdirAll(zdxDir, 0755); err != nil {
		t.Fatalf("mkdir .zdx: %v", err)
	}
	configYAML := fmt.Sprintf("remote:\n  url: %q\n  slug: %q\nhooks: {}\ncomponents: {}\n", srv.URL, slug)
	if err := os.WriteFile(filepath.Join(zdxDir, "config.yaml"), []byte(configYAML), 0644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(zdxDir, "credentials"), []byte(srv.AdminToken+"\n"), 0600); err != nil {
		t.Fatalf("write credentials: %v", err)
	}

	cmd := exec.Command(dxBin, "doctor")
	cmd.Dir = tmp
	// "tool\n" answers the classification prompt; subsequent ReadString calls
	// hit EOF and skip remaining maturity questions.
	cmd.Stdin = strings.NewReader("tool\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	runErr := cmd.Run()
	dur := time.Since(start).Milliseconds()
	exitCode := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		} else {
			exitCode = -1
		}
	}

	t.Cleanup(func() {
		dir := filepath.Join(root, ".zdx", "demo", "cli")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Logf("demo save: mkdir: %v", err)
			return
		}
		log := demoLog{
			Name:       t.Name(),
			RecordedAt: time.Now().UTC().Format(time.RFC3339),
			Steps: []demoStep{{
				Cmd:        "dx doctor",
				Args:       []string{"dx", "doctor"},
				Stdout:     stdout.String(),
				Stderr:     stderr.String(),
				ExitCode:   exitCode,
				DurationMs: dur,
			}},
		}
		b, _ := json.MarshalIndent(log, "", "  ")
		path := filepath.Join(dir, t.Name()+".json")
		if err := os.WriteFile(path, b, 0644); err != nil {
			t.Logf("demo save: %v", err)
			return
		}
		t.Logf("CLI demo saved → %s", path)
	})

	out := stdout.String()

	// Spec 130 contract 1: the prompt is shown.
	if !strings.Contains(out, "What kind of project is this?") {
		t.Errorf("expected classification prompt header in output:\n%s", out)
	}
	if !strings.Contains(out, "Choice [1-5]") {
		t.Errorf("expected 'Choice [1-5]' input line in output:\n%s", out)
	}

	// Spec 130 contract 2: all five classifications are offered.
	for _, name := range []string{"Library", "Tool", "Service", "SaaS", "Site"} {
		if !strings.Contains(out, name) {
			t.Errorf("expected classification %q in prompt menu:\n%s", name, out)
		}
	}

	// Spec 130 contract 3: the selection is persisted to the server.
	var infoAfter struct {
		Classification string `json:"classification"`
	}
	apiDo(t, http.MethodGet, "/api/dx/project/info?slug="+slug, nil, &infoAfter)
	if infoAfter.Classification != "tool" {
		t.Errorf("expected classification persisted as 'tool', got %q", infoAfter.Classification)
	}
}
