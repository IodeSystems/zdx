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

// Driver selects which backend a BDD scenario runs against.
type Driver string

const (
	DriverAPI Driver = "api" // fast programmatic driver (default)
	DriverUI  Driver = "ui"  // full browser/UI driver
)

// DriverSet is a bitmask of drivers a step supports.
type DriverSet uint8

const (
	NeedsAPI DriverSet = 1 << iota // step can run against the API driver
	NeedsUI                        // step requires a UI driver
)

// StepDriver is implemented by BDD steps that declare which drivers they support.
type StepDriver interface {
	Capability() DriverSet
}

// ScenarioBuilder wires BDD given/when/then steps to the appropriate driver.
// Given-steps always resolve to the API driver; when/then steps use the
// configured driver.
type ScenarioBuilder struct {
	driver Driver
}

// NewScenarioBuilder returns a builder for the driver specified in f.
// If f.Driver is empty, DriverAPI is used.
func NewScenarioBuilder(f Filter) *ScenarioBuilder {
	d := f.Driver
	if d == "" {
		d = DriverAPI
	}
	return &ScenarioBuilder{driver: d}
}

// Given always resolves to the API driver regardless of the selected driver.
func (b *ScenarioBuilder) Given(step StepDriver) Driver {
	return DriverAPI
}

// When resolves to the configured driver.
func (b *ScenarioBuilder) When(step StepDriver) Driver {
	return b.driver
}

// Then resolves to the configured driver.
func (b *ScenarioBuilder) Then(step StepDriver) Driver {
	return b.driver
}

// Filter scopes which tests to run. Empty fields match everything.
type Filter struct {
	Name       string // substring / regex applied to test names
	Component  string // e.g. "ui", "api", "cli"
	Feature    string // feature name token in test name
	Layer      Layer  // empty = all layers
	Driver     Driver // api | ui; empty defaults to api
	Importance string // must | should | nice-to-have; empty = no filter
}

// FilterFromEnv builds a Filter from the DX_TEST_* environment variables.
func FilterFromEnv() Filter {
	return Filter{
		Component:  os.Getenv("DX_TEST_COMPONENT"),
		Feature:    os.Getenv("DX_TEST_FEATURE"),
		Layer:      Layer(os.Getenv("DX_TEST_LAYER")),
		Driver:     Driver(os.Getenv("DX_TEST_DRIVER")),
		Importance: os.Getenv("DX_TEST_IMPORTANCE"),
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
// Walks recursively so multi-browser tests may group per-user videos under
// nested directories (e.g. video/multi/alice/<hash>.webm).
func CollectDemoMetadata(demoDir string, cutoff time.Time) []DemoMeta {
	var metas []DemoMeta
	for _, sub := range []struct {
		dir      string
		demoType string
	}{
		{"cli", "cli"},
		{"video", "video"},
		{"api", "api"},
	} {
		dir := filepath.Join(demoDir, sub.dir)
		_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil || info.ModTime().Before(cutoff) {
				return nil
			}
			name := d.Name()
			testName := name[:len(name)-len(filepath.Ext(name))]
			metas = append(metas, DemoMeta{
				Test:         testName,
				DemoType:     sub.demoType,
				ArtifactPath: path,
				RecordedAt:   info.ModTime().UTC().Format(time.RFC3339),
			})
			return nil
		})
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
