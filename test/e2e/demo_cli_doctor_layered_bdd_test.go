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

// TestDemoCLI_DoctorLayeredBDDActivation is the demo for spec 134:
// Given a service, saas, or site project with five or more UX-concern specs,
// when verification runs, then the layered-BDD test-adoption check activates;
// for other classifications or fewer UX specs, the check is skipped to avoid noise.
//
// Setup: create a saas project on the test server, add five specs to one
// feature, then run dx doctor. The layered-bdd-tests check must appear in the
// verification rung output and must NOT say "not applicable" — proving it
// activated rather than being skipped.
func TestDemoCLI_DoctorLayeredBDDActivation(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_layered_bdd_test.go", Note: "demo source (spec 134)"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 410, LineEnd: 426, Note: "layered-bdd-tests check: skips library/tool, activates for service/saas/site with 5+ specs"},
		{FilePath: "internal/doctor/vines.go", LineStart: 123, LineEnd: 123, Note: "layered-bdd-tests check definition in verification rung"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 284, LineEnd: 303, Note: "populateRemoteState: counts SpecCount across all features"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	const slug = "demo-doctor-layered-bdd"
	const featureName = "user-experience"

	// Create project and set classification to saas (service/saas/site triggers the check).
	apiDo(t, http.MethodPost, "/api/project", map[string]string{
		"slug": slug,
		"name": "Demo Doctor Layered BDD",
	}, nil)
	apiDo(t, http.MethodPost, "/api/dx/doctor/classify", map[string]any{
		"slug": slug, "classification": "saas",
	}, nil)

	// Create a feature and add five specs — the threshold that activates the check.
	apiDo(t, http.MethodPost, "/api/feature", map[string]string{
		"slug": slug, "name": featureName, "description": "UX feature for layered-bdd demo",
	}, nil)
	for i := 1; i <= 5; i++ {
		apiDo(t, http.MethodPost, "/api/dx/specs/update", map[string]string{
			"slug":    slug,
			"feature": featureName,
			"field":   "must",
			"value":   fmt.Sprintf("UX spec %d for layered-bdd activation demo", i),
		}, nil)
	}

	// Doctor requires .zdx/credentials on disk. Scaffold a temp dir wired to the
	// test server so DetectLocal's file checks pass.
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

	// Spec 134 contract 1: the layered-bdd-tests check description appears in
	// the verification rung — the check was evaluated, not silently omitted.
	if !strings.Contains(out, "layered BDD") {
		t.Errorf("expected 'layered BDD' check line in output:\n%s", out)
	}

	// Spec 134 contract 2: the check is NOT skipped as "not applicable" —
	// saas with 5+ specs activates the check rather than bypassing it.
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "layered BDD") {
			if strings.Contains(line, "not applicable") {
				t.Errorf("layered-bdd check should be active for saas with 5+ specs, got:\n  %s\nfull output:\n%s", line, out)
			}
			break
		}
	}
}
