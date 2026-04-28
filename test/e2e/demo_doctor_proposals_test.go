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

// TestDemoCLI_DoctorProposesRemediationCommands is the demo for spec 129:
// when doctor evaluates a failing check that has no auto-fix function, the
// printed finding includes a concrete `dx ...` command the user can copy
// and paste to remediate. That is what makes doctor actionable rather than
// merely diagnostic — and is the contract that lets agents close the loop
// from "what's wrong" to "what to run".
//
// The demo runs `dx doctor` in a hermetic empty tmp dir with no
// .zdx/credentials, so populateRemoteState is skipped and every
// remote-derived check defaults to its empty state and fails. Each failure
// without an auto-fix MUST surface a `dx ...` line under the `→` marker.
//
// On completion the captured stdin/stdout/exit are written to
// .zdx/demo/cli/<t.Name()>.json so CollectDemoMetadata picks up the artifact
// and clears demo-gap on this spec.
func TestDemoCLI_DoctorProposesRemediationCommands(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_doctor_proposals_test.go", Note: "doctor proposal demo source"},
		{FilePath: "internal/doctor/doctor.go", LineStart: 223, LineEnd: 435, Note: "runCheck: each non-auto-fix failure returns a concrete `dx ...` proposal string"},
		{FilePath: "internal/cli/project/doctor.go", LineStart: 154, LineEnd: 158, Note: "doctor CLI prints `      → <proposal>` after each failing finding"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	dxBin := filepath.Join(root, "bin", "dx")
	if _, err := os.Stat(dxBin); err != nil {
		t.Skipf("dx binary not built at %s — run `make build`", dxBin)
	}

	tmp := t.TempDir()

	cmd := exec.Command(dxBin, "doctor")
	cmd.Dir = tmp
	// "tool\n" answers the classification prompt by name (matches ClassTool
	// in promptClassification's default branch). Subsequent ReadString
	// calls inside the maturity / defer loops hit EOF and return empty
	// strings, which both loops treat as skip / no-op.
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

	if runErr != nil {
		t.Fatalf("dx doctor: %v\nstdout:\n%s\nstderr:\n%s",
			runErr, stdout.String(), stderr.String())
	}
	out := stdout.String()

	if !strings.Contains(out, "→") {
		t.Errorf("expected `→` proposal marker in output:\n%s", out)
	}

	// Spec 129 contract: every non-auto-fixable failing check returns a
	// proposal that doctor prints as `      → <proposal>`. We probe a
	// representative cross-section of rungs (scaffold / identity / planning)
	// rather than asserting every line — the unit test
	// TestEvaluateProposalsForNonAutoFixFailures owns the exhaustive guardrail.
	wantCmds := []string{
		"dx login",          // scaffold rung — credentials_exist
		"dx vision set",     // identity rung — has_vision
		"dx goal add",       // identity rung — has_goals
		"dx constraint add", // identity rung — has_constraints
		"dx feature add",    // planning rung — has_features
	}
	for _, want := range wantCmds {
		if !strings.Contains(out, want) {
			t.Errorf("expected proposal containing %q in dx doctor output:\n%s", want, out)
		}
	}
}
