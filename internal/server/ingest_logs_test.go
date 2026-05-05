package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestApiKeyMiddleware_LogIngestBypass verifies that POST /api/ingest/logs
// reaches the handler without an X-Api-Key header. Integration tokens (and
// the dual-auth path inside the handler) authenticate themselves via
// Authorization: Bearer, so the middleware must let the request through.
func TestApiKeyMiddleware_LogIngestBypass(t *testing.T) {
	s := &Server{wsSecret: "test-secret"}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mw := s.apiKeyMiddleware(next)

	req := httptest.NewRequest(http.MethodPost, "/api/ingest/logs", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if !called {
		t.Fatalf("middleware did not pass /api/ingest/logs through; status=%d", rec.Code)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status=%d, want 200", rec.Code)
	}
}

func TestHasCapability(t *testing.T) {
	cases := []struct {
		name string
		caps []string
		want string
		ok   bool
	}{
		{"present", []string{"ingest:logs", "ingest:metrics"}, "ingest:logs", true},
		{"missing", []string{"ingest:metrics"}, "ingest:logs", false},
		{"empty", nil, "ingest:logs", false},
		{"case sensitive", []string{"Ingest:Logs"}, "ingest:logs", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasCapability(tc.caps, tc.want); got != tc.ok {
				t.Errorf("hasCapability(%v, %q) = %v; want %v", tc.caps, tc.want, got, tc.ok)
			}
		})
	}
}
