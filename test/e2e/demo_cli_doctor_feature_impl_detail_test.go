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

// TestDemoCLI_DoctorFeatureImplDetail is the demo for spec 113:
// Given a project with a feature whose name looks like a code module or
// implementation detail (e.g. "redis-cache-layer"), when doctor runs, it flags
// that feature with guidance that features are demonstrable value drivers, not
// code modules — and proposes renaming to describe the user benefit.
func TestDemoCLI_DoctorFeatureImplDetail(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_doctor_feature_impl_detail_test.go", Note: "demo source (spec 113)"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 191, LineEnd: 212, Note: "IsImplDetailFeature: keyword-based detection of impl-detail feature names"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 364, LineEnd: 371, Note: "features_are_user_visible check: flags impl-detail features with guidance"},
		{FilePath: "internal/doctor/vines.go", LineStart: 112, LineEnd: 112, Note: "features_are_user_visible registered in planning rung"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 303, LineEnd: 305, Note: "populateRemoteState: counts FeaturesImplDetail per feature"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	const slug = "demo-doctor-feature-impl-detail"

	// Create project and set classification.
	apiDo(t, "POST", "/api/project", map[string]string{"slug": slug, "name": "Demo Doctor Feature Impl Detail"}, nil)
	apiDo(t, "POST", "/api/dx/doctor/classify", map[string]any{"slug": slug, "classification": "tool"}, nil)

	addEnv := append(os.Environ(),
		"DX_REMOTE_URL="+srv.URL,
		"DX_REMOTE_API_KEY="+srv.AdminToken,
		"DX_REMOTE_SLUG="+slug,
	)

	// Add an impl-detail feature: name looks like a code module.
	addCmd := exec.Command(dxBin, "feature", "add", "redis-cache-layer", "--desc", "Add Redis caching to the service layer")
	addCmd.Env = addEnv
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("dx feature add (impl-detail): %v\n%s", err, out)
	}

	// Add a user-visible feature so we confirm it is NOT flagged.
	addCmd2 := exec.Command(dxBin, "feature", "add", "faster-search", "--desc", "Users find results in under 200ms")
	addCmd2.Env = addEnv
	if out, err := addCmd2.CombinedOutput(); err != nil {
		t.Fatalf("dx feature add (user-visible): %v\n%s", err, out)
	}

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

	// Spec 113 contract 1: doctor flags the impl-detail feature — message cites
	// "implementation details" so the developer understands what was detected.
	if !strings.Contains(out, "implementation details") {
		t.Errorf("expected 'implementation details' in output:\n%s", out)
	}

	// Spec 113 contract 2: the proposal cites "demonstrable value driver" so the
	// developer understands what features should represent.
	if !strings.Contains(out, "demonstrable value driver") {
		t.Errorf("expected 'demonstrable value driver' in proposal:\n%s", out)
	}

	// Spec 113 contract 3: the proposal mentions "not code modules" (or similar)
	// to contrast what features are NOT.
	if !strings.Contains(out, "not code modules") {
		t.Errorf("expected 'not code modules' in proposal:\n%s", out)
	}
}
