package testharness_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

// buildCoverTestBin compiles the testharness package as a -cover test binary.
// Building from within this module ensures go tool cover can resolve source paths
// when generating the HTML report.
func buildCoverTestBin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "testharness.test")
	// "." resolves to the testharness package dir (go test sets CWD to the package).
	cmd := exec.Command("go", "test", "-c", "-cover", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test -c -cover: %v\n%s", err, out)
	}
	return bin
}

// TestCoveragePipeline verifies that:
//  1. GoBinAdapter passes GOCOVERDIR to a -cover binary so coverage files are written.
//  2. MergeCoverage produces a non-empty text profile from that directory.
//  3. CoverageReport produces a valid HTML file containing "coverage".
func TestCoveragePipeline(t *testing.T) {
	bin := buildCoverTestBin(t)
	coverDir := t.TempDir()

	// Run only TestHarness_* to avoid recursive execution of TestCoverage_*.
	a := &testharness.GoBinAdapter{
		Bin:      bin,
		Comp:     "cov",
		CoverDir: coverDir,
	}
	if _, err := a.Run(context.Background(), testharness.Filter{Name: "TestHarness"}); err != nil {
		t.Fatalf("GoBinAdapter.Run: %v", err)
	}

	entries, err := os.ReadDir(coverDir)
	if err != nil {
		t.Fatalf("ReadDir coverDir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("GOCOVERDIR not populated after run")
	}

	outDir := t.TempDir()
	profile := filepath.Join(outDir, "coverage.txt")
	if err := testharness.MergeCoverage(coverDir, profile); err != nil {
		t.Fatalf("MergeCoverage: %v", err)
	}
	data, err := os.ReadFile(profile)
	if err != nil || len(data) == 0 {
		t.Fatalf("profile empty or unreadable: %v", err)
	}

	htmlPath := filepath.Join(outDir, "coverage.html")
	if err := testharness.CoverageReport(profile, htmlPath); err != nil {
		t.Fatalf("CoverageReport: %v", err)
	}
	html, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	if !strings.Contains(strings.ToLower(string(html)), "coverage") {
		t.Error("html report missing 'coverage'")
	}
}

// TestCoverage_HTMLSmoke mirrors the post-run coverage path in testHarnessRunE:
// MergeCoverage → CoverageReport → assert .zdx/coverage/api.html exists on disk.
func TestCoverage_HTMLSmoke(t *testing.T) {
	bin := buildCoverTestBin(t)

	base := t.TempDir()
	coverDir := filepath.Join(base, ".zdx", "coverage", "api")
	profile := filepath.Join(base, ".zdx", "coverage", "api.txt")
	html := filepath.Join(base, ".zdx", "coverage", "api.html")

	a := &testharness.GoBinAdapter{
		Bin:      bin,
		Comp:     "api",
		CoverDir: coverDir,
	}
	if _, err := a.Run(context.Background(), testharness.Filter{Name: "TestHarness"}); err != nil {
		t.Fatalf("GoBinAdapter.Run: %v", err)
	}
	if err := testharness.MergeCoverage(coverDir, profile); err != nil {
		t.Fatalf("MergeCoverage: %v", err)
	}
	if err := testharness.CoverageReport(profile, html); err != nil {
		t.Fatalf("CoverageReport: %v", err)
	}
	if _, err := os.Stat(html); err != nil {
		t.Fatalf(".zdx/coverage/api.html not created: %v", err)
	}
}
