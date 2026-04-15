// Package testharness is a multi-adapter test runner.
//
// Inspired by Zig's test model: all components (Go binary, vitest UI, demo
// recordings) are fused through a single Harness. One command — dx test —
// runs everything. Adapters are registered at runtime so other projects can
// embed the harness and register their own test sources.
//
// Runtime metadata (component name, project slug, layer filter) is read from
// environment variables so the same compiled binary can be re-targeted without
// recompilation.
//
//	DX_TEST_COMPONENT  — override the default component for un-tagged adapters
//	DX_TEST_LAYER      — run only "unit", "integration", or "demo"
//	DX_TEST_FEATURE    — run only tests whose name contains this feature token
package testharness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Layer classifies a test by proximity to production and external dependencies.
type Layer string

const (
	LayerUnit        Layer = "unit"        // fast, self-contained, no I/O
	LayerIntegration Layer = "integration" // real boundaries (DB, HTTP), no browser
	LayerDemo        Layer = "demo"        // records video/CLI log; mocks only 3rd parties
)

// Filter scopes which tests to run. Empty fields match everything.
type Filter struct {
	Name      string // substring / regex applied to test names
	Component string // e.g. "ui", "api", "cli"
	Feature   string // feature name token in test name
	Layer     Layer  // empty = all layers
}

// FilterFromEnv builds a Filter from the DX_TEST_* environment variables.
func FilterFromEnv() Filter {
	return Filter{
		Component: os.Getenv("DX_TEST_COMPONENT"),
		Feature:   os.Getenv("DX_TEST_FEATURE"),
		Layer:     Layer(os.Getenv("DX_TEST_LAYER")),
	}
}

// Result is one completed test.
type Result struct {
	Test       string `json:"test"`
	Component  string `json:"component"`
	Layer      Layer  `json:"layer"`
	Feature    string `json:"feature,omitempty"`
	Status     string `json:"status"` // pass | fail | skip
	DurationMs int64  `json:"duration_ms"`
	Output     string `json:"output,omitempty"`
	RunAt      string `json:"run_at"`
	Branch     string `json:"branch,omitempty"`
	GitSHA     string `json:"git_sha,omitempty"`
}

// Adapter is anything that can enumerate and run a class of tests.
// Register implementations with Harness.Register.
type Adapter interface {
	// ID is a stable identifier, e.g. "vitest:ui" or "go:api".
	ID() string
	// Component returns the component this adapter covers.
	Component() string
	// Layers returns which test layers this adapter runs.
	Layers() []Layer
	// Run executes tests matching f and returns results.
	Run(ctx context.Context, f Filter) ([]Result, error)
}

// Harness fuses multiple adapters into a single test run.
type Harness struct {
	adapters []Adapter
}

// New returns an empty Harness.  Component/layer defaults come from env.
func New() *Harness { return &Harness{} }

// Register adds an adapter to the harness. Returns self for chaining.
func (h *Harness) Register(a Adapter) *Harness {
	h.adapters = append(h.adapters, a)
	return h
}

// Run executes all adapters that match f, merging their results.
func (h *Harness) Run(ctx context.Context, f Filter) ([]Result, error) {
	var all []Result
	for _, a := range h.adapters {
		if f.Component != "" && a.Component() != f.Component {
			continue
		}
		if f.Layer != "" && !hasLayer(a, f.Layer) {
			continue
		}
		results, err := a.Run(ctx, f)
		if err != nil {
			return all, fmt.Errorf("adapter %s: %w", a.ID(), err)
		}
		all = append(all, results...)
	}
	return all, nil
}

// WriteResults persists results to path (JSON). Parent dir is created if needed.
func WriteResults(path string, results []Result) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// Summary prints a one-line pass/fail count to stderr.
func Summary(results []Result) {
	pass, fail, skip := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "pass":
			pass++
		case "fail":
			fail++
		case "skip":
			skip++
		}
	}
	fmt.Fprintf(os.Stderr, "\n  %d passed  %d failed  %d skipped  (%s)\n",
		pass, fail, skip, time.Now().Format("15:04:05"))
}

// HasFailure returns true if any result has status "fail".
func HasFailure(results []Result) bool {
	for _, r := range results {
		if r.Status == "fail" {
			return true
		}
	}
	return false
}

func hasLayer(a Adapter, l Layer) bool {
	for _, al := range a.Layers() {
		if al == l {
			return true
		}
	}
	return false
}

// DemoMeta describes a single demo artifact produced by a test run.
type DemoMeta struct {
	Test         string `json:"test"`
	DemoType     string `json:"demo_type"`
	ArtifactPath string `json:"artifact_path"`
	RecordedAt   string `json:"recorded_at"`
}

// CollectDemoMetadata scans demoDir for CLI and video artifacts produced
// during the test run (modified after cutoff) and returns metadata entries.
func CollectDemoMetadata(demoDir string, cutoff time.Time) []DemoMeta {
	var metas []DemoMeta
	for _, sub := range []struct {
		dir      string
		demoType string
	}{
		{"cli", "cli"},
		{"video", "video"},
	} {
		dir := filepath.Join(demoDir, sub.dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, err := e.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				continue
			}
			name := e.Name()
			testName := name[:len(name)-len(filepath.Ext(name))]
			metas = append(metas, DemoMeta{
				Test:         testName,
				DemoType:     sub.demoType,
				ArtifactPath: filepath.Join(demoDir, sub.dir, name),
				RecordedAt:   info.ModTime().UTC().Format(time.RFC3339),
			})
		}
	}
	return metas
}

// WriteDemoMetadata writes one JSON object per line to path.
func WriteDemoMetadata(path string, metas []DemoMeta) error {
	if len(metas) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, m := range metas {
		if err := enc.Encode(m); err != nil {
			return err
		}
	}
	return nil
}
