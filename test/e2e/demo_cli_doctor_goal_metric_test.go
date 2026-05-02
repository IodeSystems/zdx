package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDemoCLI_DoctorGoalMissingMetric is the demo for spec 112:
// Given a goal without a measurable outcome metric, when doctor runs, it flags
// the goal — "0/N goals have metrics" — and proposes the dx goal add command
// with --metric-name and --metric-unit so the developer can quantify the goal
// as an outcome rather than an engineering domain (e.g. "correctness").
func TestDemoCLI_DoctorGoalMissingMetric(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_goal_metric_test.go", Note: "demo source (spec 112)"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 319, LineEnd: 324, Note: "goals_quantified: flags unquantified goals and proposes --metric-name/--metric-unit"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 263, LineEnd: 275, Note: "populateRemoteState: counts GoalsTotal and GoalsQuantified from MetricName"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	const slug = "demo-doctor-goal-no-metric"

	// Create project and set classification via API.
	apiDo(t, "POST", "/api/project", map[string]string{"slug": slug, "name": "Demo Doctor Goal No Metric"}, nil)
	apiDo(t, "POST", "/api/dx/doctor/classify", map[string]any{"slug": slug, "classification": "tool"}, nil)

	// Add a goal without a metric using CLI env-var auth (no .zdx/ dir needed for non-doctor commands).
	addEnv := append(os.Environ(),
		"DX_REMOTE_URL="+srv.URL,
		"DX_REMOTE_API_KEY="+srv.AdminToken,
		"DX_REMOTE_SLUG="+slug,
	)
	addCmd := exec.Command(dxBin, "goal", "add", "Correctness")
	addCmd.Env = addEnv
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("dx goal add: %v\n%s", err, out)
	}

	// Doctor requires .zdx/credentials on disk; env var alone does not satisfy
	// DetectLocal's file check. Create a temp dir that looks like a configured project.
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
	cmd.Env = append(os.Environ(), "DX_REMOTE_API_KEY="+srv.AdminToken)
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

	// Spec 112 contract 1: the failing check message cites the X/Y count.
	if !strings.Contains(out, "goals have metrics") {
		t.Errorf("expected 'goals have metrics' count in output:\n%s", out)
	}

	// Spec 112 contract 2: the proposal includes the metric flags so the
	// developer knows how to turn the goal into a measurable outcome.
	if !strings.Contains(out, "--metric-name") {
		t.Errorf("expected '--metric-name' in proposal:\n%s", out)
	}
	if !strings.Contains(out, "--metric-unit") {
		t.Errorf("expected '--metric-unit' in proposal:\n%s", out)
	}
}
