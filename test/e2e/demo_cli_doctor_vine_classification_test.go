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

// TestDemoCLI_DoctorVineClassification is the demo for spec 127:
// Given a project with classification library, tool, service, saas, or site,
// when dx doctor runs, then it evaluates the common rungs (scaffold, identity,
// planning, verification, agents) plus the classification-specific rung
// (distribution for library/tool, operations for service/saas, multi-tenancy
// for saas, publication for site).
//
// The demo creates a project with classification "tool" on the test server,
// runs dx doctor, and asserts that both the common rungs and the tool-specific
// "distribution" rung appear in the output.
func TestDemoCLI_DoctorVineClassification(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_vine_classification_test.go", Note: "spec 127 demo source"},
		{FilePath: "internal/doctor/vines.go", LineStart: 62, LineEnd: 79, Note: "Vine(): common rungs + classification-specific extension"},
		{FilePath: "internal/doctor/vines.go", LineStart: 175, LineEnd: 186, Note: "toolRungs(): distribution rung for tool classification"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 118, LineEnd: 140, Note: "print findings grouped by rung with ── {rung} ── headers"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	const slug = "demo-doctor-vine-classification"

	// Create project with no classification, then set it to "tool".
	apiDo(t, http.MethodPost, "/api/project", map[string]string{
		"slug": slug,
		"name": "Demo Doctor Vine Classification",
	}, nil)
	apiDo(t, http.MethodPost, "/api/dx/doctor/classify", map[string]string{
		"slug":           slug,
		"classification": "tool",
	}, nil)

	// Verify pre-condition: classification is set.
	var info struct {
		Classification string `json:"classification"`
	}
	apiDo(t, http.MethodGet, "/api/dx/project/info?slug="+slug, nil, &info)
	if info.Classification != "tool" {
		t.Fatalf("pre-condition: classification should be 'tool', got %q", info.Classification)
	}

	// Scaffold a temp project dir wired to the test server.
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

	// Common rungs must all appear — these are always evaluated regardless of classification.
	for _, rung := range []string{"scaffold", "identity", "planning", "verification", "agents"} {
		if !strings.Contains(out, "── "+rung+" ──") {
			t.Errorf("expected common rung %q header in output:\n%s", rung, out)
		}
	}

	// Classification-specific rung for "tool" must appear.
	if !strings.Contains(out, "── distribution ──") {
		t.Errorf("expected classification-specific rung 'distribution' for tool project:\n%s", out)
	}
}
