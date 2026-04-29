package devtools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
)

func TestKpiSampleBodyOmitsRegressionFlagBelowThreshold(t *testing.T) {
	got, err := kpiSampleBody(kpiSampleParams{
		Slug: "demo", Scope: "tech", CheckName: "lint", Value: 4000, Unit: "ms",
		RegressionThresholdMs: 15000,
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["is_regression_candidate"]; ok {
		t.Fatalf("did not expect is_regression_candidate, got %s", got)
	}
	if m["check_name"] != "lint" || m["scope"] != "tech" || m["unit"] != "ms" {
		t.Fatalf("unexpected body: %s", got)
	}
	if m["value"].(float64) != 4000 {
		t.Fatalf("unexpected value: %v", m["value"])
	}
}

func TestKpiSampleBodySetsRegressionFlagAboveThreshold(t *testing.T) {
	got, err := kpiSampleBody(kpiSampleParams{
		Slug: "demo", Scope: "tech", CheckName: "lint", Value: 16000, Unit: "ms",
		RegressionThresholdMs: 15000,
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if v, _ := m["is_regression_candidate"].(bool); !v {
		t.Fatalf("expected is_regression_candidate=true, got %s", got)
	}
}

func TestKpiSampleBodyRegressionFlagOnlyForMs(t *testing.T) {
	got, err := kpiSampleBody(kpiSampleParams{
		Slug: "demo", Scope: "tech", CheckName: "errors", Value: 9000, Unit: "count",
		RegressionThresholdMs: 15000,
	})
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["is_regression_candidate"]; ok {
		t.Fatalf("regression flag must be ms-only, got %s", got)
	}
}

func TestRunKpiSamplePostsCorrectBody(t *testing.T) {
	var gotPath, gotMethod, gotKey string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotKey = r.Header.Get("X-Api-Key")
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": 7, "sampled_at": "2026-04-29T00:00:00Z"}`))
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "test-key", "demo")
	err := runKpiSample(context.Background(), c, kpiSampleParams{
		Slug: "demo", Scope: "tech", CheckName: "bin_lint", Value: 1234, Unit: "ms",
		RegressionThresholdMs: 15000,
	})
	if err != nil {
		t.Fatalf("runKpiSample: %v", err)
	}
	if gotMethod != "POST" || gotPath != "/api/dx/kpi/sample" {
		t.Fatalf("unexpected request: %s %s", gotMethod, gotPath)
	}
	if gotKey != "test-key" {
		t.Fatalf("missing api key, got %q", gotKey)
	}
	if gotBody["slug"] != "demo" || gotBody["check_name"] != "bin_lint" {
		t.Fatalf("unexpected body: %+v", gotBody)
	}
}

func TestRunKpiSampleRejectsInvalidScope(t *testing.T) {
	c := cli.NewClientWithSlug("http://example.invalid", "k", "demo")
	err := runKpiSample(context.Background(), c, kpiSampleParams{
		Slug: "demo", Scope: "garbage", CheckName: "x", Value: 1, Unit: "ms",
	})
	if err == nil {
		t.Fatal("expected error for invalid scope")
	}
}
