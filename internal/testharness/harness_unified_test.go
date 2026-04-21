package testharness_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

type stubAdapter struct {
	id        string
	component string
	layers    []testharness.Layer
	results   []testharness.Result
}

func (s *stubAdapter) ID() string                { return s.id }
func (s *stubAdapter) Component() string         { return s.component }
func (s *stubAdapter) Layers() []testharness.Layer { return s.layers }
func (s *stubAdapter) Run(_ context.Context, _ testharness.Filter) ([]testharness.Result, error) {
	return s.results, nil
}

func makeResult(component, test, status string) testharness.Result {
	return testharness.Result{
		Test:       test,
		Component:  component,
		Layer:      testharness.LayerUnit,
		Status:     status,
		DurationMs: 42,
		RunAt:      time.Now().UTC().Format(time.RFC3339),
	}
}

func TestUnifiedOutput(t *testing.T) {
	vitestAdapter := &stubAdapter{
		id:        "vitest:ui",
		component: "ui",
		layers:    []testharness.Layer{testharness.LayerUnit},
		results:   []testharness.Result{makeResult("ui", "renders dashboard", "pass")},
	}
	goAdapter := &stubAdapter{
		id:        "go:api",
		component: "api",
		layers:    []testharness.Layer{testharness.LayerUnit},
		results:   []testharness.Result{makeResult("api", "GET /api/issues returns 200", "pass")},
	}
	demoAdapter := &stubAdapter{
		id:        "demo:cli",
		component: "demo",
		layers:    []testharness.Layer{testharness.LayerDemo},
		results:   []testharness.Result{makeResult("demo", "dx todo take flow", "pass")},
	}

	h := testharness.New()
	h.Register(vitestAdapter)
	h.Register(goAdapter)
	h.Register(demoAdapter)

	ctx := context.Background()
	results, err := h.Run(ctx, testharness.Filter{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// All three adapters contribute
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	byComponent := make(map[string]testharness.Result, len(results))
	for _, r := range results {
		byComponent[r.Component] = r
	}
	for _, want := range []string{"ui", "api", "demo"} {
		if _, ok := byComponent[want]; !ok {
			t.Errorf("missing component %q in results", want)
		}
	}

	// Each result has required fields
	for _, r := range results {
		if r.Status == "" {
			t.Errorf("result %q: empty Status", r.Test)
		}
		if r.DurationMs == 0 {
			t.Errorf("result %q: zero DurationMs", r.Test)
		}
		if r.RunAt == "" {
			t.Errorf("result %q: empty RunAt", r.Test)
		}
	}

	// WriteResults produces exactly one file; file decodes back to all results
	dir := t.TempDir()
	outPath := filepath.Join(dir, "results.json")
	if err := testharness.WriteResults(outPath, results); err != nil {
		t.Fatalf("WriteResults: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 file in tmpdir, got %d", len(entries))
	}

	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var decoded []testharness.Result
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(decoded) != len(results) {
		t.Fatalf("decoded %d results, want %d", len(decoded), len(results))
	}
}
