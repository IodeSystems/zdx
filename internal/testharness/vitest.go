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

// Run executes `npx vitest run` and parses the JSON report.
func (a *VitestAdapter) Run(ctx context.Context, f Filter) ([]Result, error) {
	// Write report to a temp file; vitest's JSON reporter defaults to a file.
	tmp, err := os.CreateTemp("", "vitest-report-*.json")
	if err != nil {
		return nil, fmt.Errorf("vitest: create temp: %w", err)
	}
	defer os.Remove(tmp.Name())
	tmp.Close()

	args := []string{"vitest", "run",
		"--reporter=json",
		"--outputFile=" + tmp.Name(),
	}
	if f.Name != "" {
		args = append(args, "--testNamePattern="+f.Name)
	} else if f.Feature != "" {
		args = append(args, "--testNamePattern="+f.Feature)
	}

	cmd := exec.CommandContext(ctx, "npx", args...)
	cmd.Dir = filepath.Clean(a.Dir)
	cmd.Stdout = os.Stderr // vitest progress to stderr
	cmd.Stderr = os.Stderr
	// vitest exits non-zero on test failure — that's expected; we read results.
	_ = cmd.Run()

	return parseVitestReport(tmp.Name(), a.Comp)
}

// ── vitest JSON report parsing ─────────────────────────────────────────────

type vitestReport struct {
	TestResults []vitestFileResult `json:"testResults"`
}

type vitestFileResult struct {
	TestFilePath string             `json:"testFilePath"`
	Status       string             `json:"status"` // "passed" | "failed"
	TestResults  []vitestTestResult `json:"testResults"`
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
