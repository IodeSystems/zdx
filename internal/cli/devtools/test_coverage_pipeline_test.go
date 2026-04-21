package devtools

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoveragePipeline verifies the full dx test --coverage pipeline:
// flag parsing → GOCOVERDIR injection → raw data collection → textfmt merge → html report.
//
// Runs from the package source dir (within the zdx-go module) so that
// go tool cover -html can resolve package source files.
func TestCoveragePipeline(t *testing.T) {
	// Remove any coverage artifacts written by this test on exit.
	t.Cleanup(func() { os.RemoveAll(".zdx") })

	// Build a test binary from a zdx-go package with -cover so that
	// source files are resolvable by go tool cover -html.
	binPath := filepath.Join(t.TempDir(), "testharness.test")
	buildCmd := exec.Command("go", "test", "-c", "-cover", "-o", binPath,
		"github.com/iodesystems/zdx-go/internal/testharness")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build test binary: %v\n%s", err, out)
	}

	// Override testBin so testHarnessRunE uses our coverage-enabled binary.
	origBin := testBin
	testBin = binPath
	t.Cleanup(func() { testBin = origBin })

	// Capture stderr to assert the [coverage] summary line.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = w

	// Run: dx test --coverage --component api --no-ephemeral
	cmd := TestCmd()
	cmd.SetArgs([]string{"--coverage", "--component", "api", "--no-ephemeral"})
	runErr := cmd.Execute()

	w.Close()
	os.Stderr = origStderr
	stderrBytes, _ := io.ReadAll(r)
	r.Close()

	if runErr != nil {
		t.Fatalf("cmd.Execute: %v\nstderr: %s", runErr, stderrBytes)
	}

	// 1. .zdx/coverage/api/ has raw GOCOVERDIR data.
	entries, err := os.ReadDir(filepath.Join(".zdx", "coverage", "api"))
	if err != nil {
		t.Fatalf("read coverDir: %v", err)
	}
	if len(entries) == 0 {
		t.Error(".zdx/coverage/api/ has no GOCOVERDIR data")
	}

	// 2. .zdx/coverage/api.txt profile created.
	if _, err := os.Stat(filepath.Join(".zdx", "coverage", "api.txt")); err != nil {
		t.Errorf(".zdx/coverage/api.txt not created: %v", err)
	}

	// 3. .zdx/coverage/api.html report created.
	if _, err := os.Stat(filepath.Join(".zdx", "coverage", "api.html")); err != nil {
		t.Errorf(".zdx/coverage/api.html not created: %v", err)
	}

	// 4. Stderr contains the coverage summary line from testHarnessRunE.
	wantLine := "[coverage] api → .zdx/coverage/api.html"
	if !strings.Contains(string(stderrBytes), wantLine) {
		t.Errorf("stderr missing %q\nfull stderr:\n%s", wantLine, stderrBytes)
	}
}
