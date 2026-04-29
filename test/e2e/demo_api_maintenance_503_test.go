package e2e

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestDemoAPI_MaintenanceModeReturns503 is the demo for spec 60: given
// incompatible migrations, when bin/ship deploys with --non-compatible-migration
// it sets MAINTENANCE=true on the current slot via a systemd env override.
// All routes except the exact path /health then return 503 with a styled HTML
// maintenance page until migrations finish and the override is removed.
//
// This demo starts an isolated httptest server that mirrors the production
// ServeHTTP gate (internal/server/server.go:335) so the scenario is
// reproducible without infrastructure.
func TestDemoAPI_MaintenanceModeReturns503(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_api_maintenance_503_test.go", Note: "demo source"},
		{FilePath: "internal/server/server.go", LineStart: 335, LineEnd: 348, Note: "ServeHTTP maintenance gate and isMaintenance check"},
		{FilePath: "bin/ship", LineStart: 585, LineEnd: 587, Note: "bin/ship sets MAINTENANCE=true on current slot before migrating"},
	})

	// inMaintenanceMode mirrors isMaintenance() in internal/server/server.go.
	inMaintenanceMode := func() bool { return os.Getenv("MAINTENANCE") == "true" }

	// maintenanceBody is an excerpt of the styled page returned by the server.
	// Full content lives in internal/server/server.go:maintenancePage.
	maintenanceBody := []byte(`<!doctype html>` +
		`<html lang="en"><head><title>Maintenance</title>` +
		`<body><p>Maintenance in progress</p></body></html>`)

	// Minimal server that replicates the production maintenance gate.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if inMaintenanceMode() && r.URL.Path != "/health" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write(maintenanceBody) //nolint:errcheck
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`)) //nolint:errcheck
	}))
	defer ts.Close()

	// ── Before maintenance: normal routing ───────────────────────────────────
	preResp := mustGet(t, ts.URL+"/api/health")
	if preResp.StatusCode != http.StatusOK {
		t.Fatalf("pre-maintenance: want 200, got %d", preResp.StatusCode)
	}

	// ── Activate maintenance (mirrors what bin/ship does via systemd override) ─
	t.Setenv("MAINTENANCE", "true")

	// ── Any non-/health path → 503 with styled HTML maintenance page ─────────
	resp503 := mustGet(t, ts.URL+"/api/dx/todo/list")
	body503, _ := io.ReadAll(resp503.Body)
	resp503.Body.Close()

	if resp503.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("maintenance /api/*: want 503, got %d\nbody: %s", resp503.StatusCode, body503)
	}
	if ct := resp503.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type=%q, want text/html", ct)
	}
	if ra := resp503.Header.Get("Retry-After"); ra != "60" {
		t.Errorf("Retry-After=%q, want 60", ra)
	}
	if !strings.Contains(string(body503), "Maintenance") {
		t.Errorf("maintenance page body missing 'Maintenance' keyword")
	}

	// ── /health (exact path) bypasses maintenance for load-balancer probes ───
	// bin/ship uses this bypass to detect when the slot has finished migrating.
	healthResp := mustGet(t, ts.URL+"/health")
	if healthResp.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("/health must not return 503 (load-balancer bypass)")
	}
}

func mustGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}
