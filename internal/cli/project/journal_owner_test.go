package project

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// TestCollectAndAttachOwnerMetricsPostsKpiSamplesAndStateJson is the IS-589
// owner-branch contract test: hitting /api/dx/owner/snapshot, persisting the
// snapshot as StateJson on the journal body, and posting one KPI sample per
// metric with scope=owner.
func TestCollectAndAttachOwnerMetricsPostsKpiSamplesAndStateJson(t *testing.T) {
	type kpiCall struct {
		Slug      string  `json:"slug"`
		Scope     string  `json:"scope"`
		CheckName string  `json:"check_name"`
		Value     float64 `json:"value"`
		Unit      string  `json:"unit"`
	}

	var (
		mu       sync.Mutex
		kpiCalls []kpiCall
	)
	snapshotMetrics := map[string]float64{
		"features_total":                          5,
		"features_with_specs":                     4,
		"features_with_demos":                     2,
		"spec_coverage_must_pct":                  80,
		"spec_coverage_should_pct":                50,
		"spec_coverage_nice_pct":                  10,
		"specs_implemented_per_period":            6,
		"issues_closed_per_period":                7,
		"maturity_rungs_passed":                   3,
		"maturity_rungs_failed":                   1,
		"blocker_question_avg_resolution_minutes": 22.5,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/dx/owner/snapshot":
			if r.Method != http.MethodGet {
				t.Errorf("snapshot: expected GET, got %s", r.Method)
			}
			if r.URL.Query().Get("slug") != "demo" {
				t.Errorf("snapshot: missing slug query, got %q", r.URL.Query().Get("slug"))
			}
			if r.URL.Query().Get("period_days") != "30" {
				t.Errorf("snapshot: missing period_days=30, got %q", r.URL.Query().Get("period_days"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metrics":      snapshotMetrics,
				"period_days":  30,
				"generated_at": "2026-04-29T12:00:00Z",
			})
		case "/api/dx/journal/show":
			// no prior entry -> baseline
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"entries":[]}`))
		case "/api/dx/kpi/sample":
			b, _ := io.ReadAll(r.Body)
			var k kpiCall
			if err := json.Unmarshal(b, &k); err != nil {
				t.Errorf("kpi/sample: bad body: %v", err)
			}
			mu.Lock()
			kpiCalls = append(kpiCalls, k)
			mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1,"sampled_at":"2026-04-29T12:00:00Z"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "test-key", "demo")
	body := dxclient.JournalCheckinRequest{Slug: "demo", Role: "owner"}
	if err := collectAndAttachOwnerMetrics(context.Background(), c, &body); err != nil {
		t.Fatalf("collectAndAttachOwnerMetrics: %v", err)
	}

	// StateJson present and parseable
	if body.StateJson == nil || *body.StateJson == "" {
		t.Fatal("expected StateJson to be populated")
	}
	var state map[string]float64
	if err := json.Unmarshal([]byte(*body.StateJson), &state); err != nil {
		t.Fatalf("StateJson invalid: %v", err)
	}
	if state["features_total"] != 5 {
		t.Errorf("StateJson features_total = %v, want 5", state["features_total"])
	}
	if state["spec_coverage_must_pct"] != 80 {
		t.Errorf("StateJson spec_coverage_must_pct = %v, want 80", state["spec_coverage_must_pct"])
	}

	// Baseline: ChangelogJson is "[]" — we have no prior entry.
	if body.ChangelogJson == nil || *body.ChangelogJson != "[]" {
		got := ""
		if body.ChangelogJson != nil {
			got = *body.ChangelogJson
		}
		t.Errorf("ChangelogJson = %q, want %q (no prior entry)", got, "[]")
	}

	// One KPI sample per metric (11 total), all scope=owner.
	mu.Lock()
	defer mu.Unlock()
	if len(kpiCalls) != 11 {
		t.Fatalf("kpi calls = %d, want 11; got %+v", len(kpiCalls), kpiCalls)
	}
	expectedUnits := map[string]string{
		"features_total":                          "count",
		"features_with_specs":                     "count",
		"features_with_demos":                     "count",
		"spec_coverage_must_pct":                  "percent",
		"spec_coverage_should_pct":                "percent",
		"spec_coverage_nice_pct":                  "percent",
		"specs_implemented_per_period":            "count",
		"issues_closed_per_period":                "count",
		"maturity_rungs_passed":                   "count",
		"maturity_rungs_failed":                   "count",
		"blocker_question_avg_resolution_minutes": "min",
	}
	seen := map[string]bool{}
	for _, k := range kpiCalls {
		if k.Scope != "owner" {
			t.Errorf("kpi sample %s scope = %q, want owner", k.CheckName, k.Scope)
		}
		if k.Slug != "demo" {
			t.Errorf("kpi sample %s slug = %q, want demo", k.CheckName, k.Slug)
		}
		if want, ok := expectedUnits[k.CheckName]; !ok {
			t.Errorf("unexpected check_name: %q", k.CheckName)
		} else if k.Unit != want {
			t.Errorf("kpi sample %s unit = %q, want %q", k.CheckName, k.Unit, want)
		}
		if k.Value != snapshotMetrics[k.CheckName] {
			t.Errorf("kpi sample %s value = %v, want %v", k.CheckName, k.Value, snapshotMetrics[k.CheckName])
		}
		seen[k.CheckName] = true
	}
	for name := range expectedUnits {
		if !seen[name] {
			t.Errorf("expected kpi sample for %s, not seen", name)
		}
	}
}

// TestCollectAndAttachOwnerMetricsWithPriorEntry exercises the delta path:
// when /api/dx/journal/show returns a prior owner entry, the body's
// ChangelogJson is the JSON-marshaled diff (not "[]").
func TestCollectAndAttachOwnerMetricsWithPriorEntry(t *testing.T) {
	priorState := `{"features_total":3,"issues_closed_per_period":4,"spec_coverage_must_pct":60}`
	currentMetrics := map[string]float64{
		"features_total":                          5,
		"features_with_specs":                     0,
		"features_with_demos":                     0,
		"spec_coverage_must_pct":                  80,
		"spec_coverage_should_pct":                0,
		"spec_coverage_nice_pct":                  0,
		"specs_implemented_per_period":            0,
		"issues_closed_per_period":                7,
		"maturity_rungs_passed":                   0,
		"maturity_rungs_failed":                   0,
		"blocker_question_avg_resolution_minutes": 0,
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/dx/owner/snapshot":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metrics":      currentMetrics,
				"period_days":  30,
				"generated_at": "2026-04-29T12:00:00Z",
			})
		case "/api/dx/journal/show":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"entries": []map[string]any{
					{"id": 99, "role": "owner", "date": "2026-04-22", "state_json": priorState},
				},
			})
		case "/api/dx/kpi/sample":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1,"sampled_at":"x"}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "test-key", "demo")
	body := dxclient.JournalCheckinRequest{Slug: "demo", Role: "owner"}
	if err := collectAndAttachOwnerMetrics(context.Background(), c, &body); err != nil {
		t.Fatalf("collectAndAttachOwnerMetrics: %v", err)
	}

	if body.ChangelogJson == nil || *body.ChangelogJson == "[]" || *body.ChangelogJson == "" {
		got := ""
		if body.ChangelogJson != nil {
			got = *body.ChangelogJson
		}
		t.Fatalf("ChangelogJson should hold deltas, got %q", got)
	}
	var deltas []map[string]any
	if err := json.Unmarshal([]byte(*body.ChangelogJson), &deltas); err != nil {
		t.Fatalf("ChangelogJson invalid: %v", err)
	}
	want := map[string]float64{
		"features_total":           2,  // 3 → 5
		"issues_closed_per_period": 3,  // 4 → 7
		"spec_coverage_must_pct":   20, // 60 → 80
	}
	got := map[string]float64{}
	for _, d := range deltas {
		got[d["name"].(string)] = d["diff"].(float64)
	}
	for name, wantDiff := range want {
		if got[name] != wantDiff {
			t.Errorf("delta %s = %v, want %v", name, got[name], wantDiff)
		}
	}
}
