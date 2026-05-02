package testharness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// detectJSRunner returns "jest" when jest is installed locally (node_modules/.bin/jest
// exists) and vitest is not, otherwise "vitest". Both produce compatible JSON output.
func detectJSRunner(dir string) string {
	vitestBin := filepath.Join(dir, "node_modules", ".bin", "vitest")
	jestBin := filepath.Join(dir, "node_modules", ".bin", "jest")
	if _, err := os.Stat(vitestBin); err == nil {
		return "vitest"
	}
	if _, err := os.Stat(jestBin); err == nil {
		return "jest"
	}
	return "vitest" // fallback: let npx resolve it
}

// VitestAdapter runs vitest in a source directory and maps results to
// testharness Results. It covers the unit layer by default.
//
// Requires Node.js and vitest installed (npx vitest run).
// Source checkout is required — vitest operates on source, not dist.
type VitestAdapter struct {
	// Dir is the path to the directory containing package.json + vitest config.
	Dir string
	// Comp is the component name reported in results (e.g. "ui").
	Comp string
}

func (a *VitestAdapter) ID() string        { return "vitest:" + a.Comp }
func (a *VitestAdapter) Component() string { return a.Comp }
func (a *VitestAdapter) Layers() []Layer   { return []Layer{LayerUnit} }

// jsRunArgs returns the npx argv for the given runner, filter and output file.
// Both vitest and jest accept --testNamePattern and produce compatible JSON output.
func jsRunArgs(runner string, f Filter, outputFile string) []string {
	var args []string
	switch runner {
	case "jest":
		args = []string{"jest", "--json", "--outputFile=" + outputFile}
	default:
		args = []string{"vitest", "run", "--reporter=json", "--outputFile=" + outputFile}
	}
	if f.Name != "" {
		args = append(args, "--testNamePattern="+f.Name)
	} else if f.Feature != "" {
		args = append(args, "--testNamePattern="+f.Feature)
	}
	return args
}

// Run executes `npx vitest run` or `npx jest --json` and parses the JSON report.
// Both runners produce the same assertionResults JSON schema.
func (a *VitestAdapter) Run(ctx context.Context, f Filter) ([]Result, error) {
	tmp, err := os.CreateTemp("", "vitest-report-*.json")
	if err != nil {
		return nil, fmt.Errorf("vitest: create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	dir := filepath.Clean(a.Dir)
	runner := detectJSRunner(dir)
	args := jsRunArgs(runner, f, tmp.Name())

	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	// both runners exit non-zero on test failure — that's expected; we read results.
	_ = cmd.Run()

	return parseVitestReport(tmp.Name(), a.Comp)
}

// ── vitest JSON report parsing ─────────────────────────────────────────────

type vitestReport struct {
	TestResults []vitestFileResult `json:"testResults"`
}

type vitestFileResult struct {
	TestFilePath string             `json:"name"`
	Status       string             `json:"status"` // "passed" | "failed"
	TestResults  []vitestTestResult `json:"assertionResults"`
}

type vitestTestResult struct {
	FullName        string   `json:"fullName"`
	Status          string   `json:"status"`   // "passed" | "failed" | "skipped"
	Duration        float64  `json:"duration"` // ms
	FailureMessages []string `json:"failureMessages"`
}

func parseVitestReport(path, component string) ([]Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("vitest: read report: %w", err)
	}
	var report vitestReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("vitest: parse report: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var results []Result
	for _, file := range report.TestResults {
		for _, t := range file.TestResults {
			status := mapVitestStatus(t.Status)
			output := ""
			if len(t.FailureMessages) > 0 {
				for _, m := range t.FailureMessages {
					output += m + "\n"
				}
			}
			results = append(results, Result{
				Test:       t.FullName,
				Component:  component,
				Layer:      LayerUnit,
				Status:     status,
				DurationMs: int64(t.Duration),
				Output:     output,
				RunAt:      now,
			})
		}
	}
	return results, nil
}

func mapVitestStatus(s string) string {
	switch s {
	case "passed":
		return "pass"
	case "failed":
		return "fail"
	default:
		return "skip"
	}
}
