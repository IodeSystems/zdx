package devtools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/testharness"
)

func TestPostKpiSamplesAggregatesBySuite(t *testing.T) {
	results := []testharness.Result{
		{Test: "TestFoo", Component: "ui", Layer: testharness.LayerUnit, Status: "pass", DurationMs: 1500},
		{Test: "TestBar", Component: "ui", Layer: testharness.LayerUnit, Status: "fail", DurationMs: 200},
		{Test: "TestBaz", Component: "ui", Layer: testharness.LayerUnit, Status: "skip", DurationMs: 0},
		{Test: "TestApiOne", Component: "api", Layer: testharness.LayerIntegration, Status: "pass", DurationMs: 16000},
		{Test: "TestDemoBrowse", Component: "api", Layer: testharness.LayerDemo, Status: "pass", DurationMs: 4000},
	}

	var mu sync.Mutex
	var bodies []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dx/kpi/sample" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		mu.Lock()
		bodies = append(bodies, m)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1,"sampled_at":"2026-04-29T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "k", "demo")
	postKpiSamples(context.Background(), c, "demo", results)

	// 3 suites × 4 samples each = 12 POSTs.
	if len(bodies) != 12 {
		t.Fatalf("expected 12 POSTs, got %d", len(bodies))
	}

	byCheck := make(map[string]map[string]any)
	for _, b := range bodies {
		name, _ := b["check_name"].(string)
		byCheck[name] = b
	}

	cases := []struct {
		name             string
		value            float64
		unit             string
		regressionFlag   bool
		expectFlagInBody bool
	}{
		{"test_time_ms.ui.unit", 1700, "ms", false, false},
		{"test_pass_count.ui.unit", 1, "count", false, false},
		{"test_fail_count.ui.unit", 1, "count", false, false},
		{"test_skip_count.ui.unit", 1, "count", false, false},
		{"test_time_ms.api.integration", 16000, "ms", true, true},
		{"test_pass_count.api.integration", 1, "count", false, false},
		{"test_fail_count.api.integration", 0, "count", false, false},
		{"test_skip_count.api.integration", 0, "count", false, false},
		{"test_time_ms.demo.demo", 4000, "ms", false, false},
		{"test_pass_count.demo.demo", 1, "count", false, false},
		{"test_fail_count.demo.demo", 0, "count", false, false},
		{"test_skip_count.demo.demo", 0, "count", false, false},
	}

	for _, tc := range cases {
		body, ok := byCheck[tc.name]
		if !ok {
			t.Errorf("missing POST for check_name=%q", tc.name)
			continue
		}
		if v, _ := body["value"].(float64); v != tc.value {
			t.Errorf("%s: value=%v, want %v", tc.name, v, tc.value)
		}
		if u, _ := body["unit"].(string); u != tc.unit {
			t.Errorf("%s: unit=%q, want %q", tc.name, u, tc.unit)
		}
		if s, _ := body["scope"].(string); s != "tech" {
			t.Errorf("%s: scope=%q, want tech", tc.name, s)
		}
		flag, hasFlag := body["is_regression_candidate"]
		if tc.expectFlagInBody {
			if !hasFlag {
				t.Errorf("%s: expected is_regression_candidate=true, missing", tc.name)
			} else if v, _ := flag.(bool); v != tc.regressionFlag {
				t.Errorf("%s: is_regression_candidate=%v, want %v", tc.name, v, tc.regressionFlag)
			}
		} else if hasFlag {
			t.Errorf("%s: did not expect is_regression_candidate, got %v", tc.name, flag)
		}
	}
}

func TestPostKpiSamplesSkipsWhenSlugEmpty(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "k", "")
	postKpiSamples(context.Background(), c, "", []testharness.Result{
		{Test: "TestX", Component: "ui", Layer: testharness.LayerUnit, Status: "pass", DurationMs: 100},
	})
	if hits != 0 {
		t.Fatalf("expected no POSTs when slug empty, got %d", hits)
	}
}

func TestPostKpiSamplesSkipsWhenResultsEmpty(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "k", "demo")
	postKpiSamples(context.Background(), c, "demo", nil)
	if hits != 0 {
		t.Fatalf("expected no POSTs when results empty, got %d", hits)
	}
}

func TestPostKpiSamplesNormalizesDemoComponent(t *testing.T) {
	// TestDemo* names should fold under component=demo regardless of the
	// adapter-supplied Component field — matches syncTestResults behavior.
	results := []testharness.Result{
		{Test: "TestDemoFoo", Component: "api", Layer: testharness.LayerDemo, Status: "pass", DurationMs: 100},
	}

	var checkNames []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		if name, _ := m["check_name"].(string); name != "" {
			checkNames = append(checkNames, name)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "k", "demo")
	postKpiSamples(context.Background(), c, "demo", results)

	sort.Strings(checkNames)
	want := []string{
		"test_fail_count.demo.demo",
		"test_pass_count.demo.demo",
		"test_skip_count.demo.demo",
		"test_time_ms.demo.demo",
	}
	if len(checkNames) != len(want) {
		t.Fatalf("got %v, want %v", checkNames, want)
	}
	for i := range want {
		if checkNames[i] != want[i] {
			t.Fatalf("got %v, want %v", checkNames, want)
		}
	}
}
