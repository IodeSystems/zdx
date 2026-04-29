package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// newMaintenanceTestServer builds a minimal Server whose mux returns a known
// sentinel for any path, so we can distinguish "maintenance short-circuit fired"
// from "request reached the underlying handler".
func newMaintenanceTestServer() *Server {
	mux := chi.NewMux()
	sentinel := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.Handle("/*", sentinel)
	return &Server{mux: mux}
}

func TestServeHTTP_MaintenanceShortCircuit(t *testing.T) {
	t.Setenv("MAINTENANCE", "true")
	s := newMaintenanceTestServer()

	paths := []string{"/", "/api/anything", "/openapi.json", "/some/spa/route"}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if rec.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d, want 503", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
				t.Errorf("Content-Type=%q, want text/html", ct)
			}
			if ra := rec.Header().Get("Retry-After"); ra != "60" {
				t.Errorf("Retry-After=%q, want 60", ra)
			}
			if !bytes.Equal(rec.Body.Bytes(), maintenancePage) {
				t.Errorf("body did not equal maintenancePage; got %d bytes", rec.Body.Len())
			}
		})
	}
}

func TestServeHTTP_MaintenanceBypassesHealth(t *testing.T) {
	t.Setenv("MAINTENANCE", "true")
	s := newMaintenanceTestServer()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200 (health must bypass maintenance)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Errorf("body=%q, want underlying handler response", rec.Body.String())
	}
}

func TestServeHTTP_MaintenanceOffRoutesNormally(t *testing.T) {
	t.Setenv("MAINTENANCE", "")
	s := newMaintenanceTestServer()

	paths := []string{"/", "/api/anything", "/openapi.json"}
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200 (maintenance off should reach mux)", rec.Code)
			}
			if rec.Header().Get("Retry-After") != "" {
				t.Errorf("Retry-After should not be set when maintenance is off")
			}
		})
	}
}

// MAINTENANCE only activates on the literal string "true"; any other value
// (including "1", "yes") must NOT trigger maintenance mode. Keeps the toggle
// unambiguous for bin/ship's systemd override.
func TestServeHTTP_MaintenanceRequiresExactTrue(t *testing.T) {
	for _, v := range []string{"1", "yes", "TRUE", "True"} {
		t.Run("MAINTENANCE="+v, func(t *testing.T) {
			t.Setenv("MAINTENANCE", v)
			s := newMaintenanceTestServer()
			req := httptest.NewRequest(http.MethodGet, "/api/anything", nil)
			rec := httptest.NewRecorder()
			s.ServeHTTP(rec, req)

			if rec.Code == http.StatusServiceUnavailable {
				t.Fatalf("MAINTENANCE=%q unexpectedly triggered 503", v)
			}
		})
	}
}
