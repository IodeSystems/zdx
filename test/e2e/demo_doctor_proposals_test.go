package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	if err := cmd.Run(); err != nil {
		t.Fatalf("dx doctor: %v\nstdout:\n%s\nstderr:\n%s",
			err, stdout.String(), stderr.String())
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
