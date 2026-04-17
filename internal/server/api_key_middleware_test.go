package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/server/handlers"
)

// TestApiKeyMiddleware_SignedAssetBypass verifies that a properly-signed asset URL
// can be served without an X-Api-Key header (so <video src> and bare fetch can hit
// /api/files and /api/dx/demos/* artifacts).
func TestApiKeyMiddleware_SignedAssetBypass(t *testing.T) {
	s := &Server{wsSecret: "test-secret"}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	mw := s.apiKeyMiddleware(next)

	cases := []struct {
		name string
		path string
	}{
		{"file asset", "/api/files/42"},
		{"cli demo", "/api/dx/demos/cli/some-demo"},
		{"video demo", "/api/dx/demos/video/some-demo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			signed := handlers.SignAssetURL(s.wsSecret, tc.path, time.Hour)
			req := httptest.NewRequest(http.MethodGet, signed, nil)
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200; body=%s", rec.Code, rec.Body.String())
			}
			if !called {
				t.Fatal("downstream handler was not invoked")
			}
		})
	}
}

func TestApiKeyMiddleware_UnsignedAssetReturns401(t *testing.T) {
	s := &Server{wsSecret: "test-secret"}
	mw := s.apiKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler should not be invoked without auth")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/files/42", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401; body=%s", rec.Code, rec.Body.String())
	}
}

func TestApiKeyMiddleware_SignedNonAssetPathStill401(t *testing.T) {
	s := &Server{wsSecret: "test-secret"}
	mw := s.apiKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler should not be invoked: signed URL must not bypass non-asset paths")
	}))

	// Sign a non-asset path; bypass must not apply.
	signed := handlers.SignAssetURL(s.wsSecret, "/api/dx/issues/list", time.Hour)
	req := httptest.NewRequest(http.MethodGet, signed, nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
}

func TestApiKeyMiddleware_SignedAssetPOSTRejected(t *testing.T) {
	s := &Server{wsSecret: "test-secret"}
	mw := s.apiKeyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler should not be invoked: POST must not be eligible for signed bypass")
	}))

	signed := handlers.SignAssetURL(s.wsSecret, "/api/files/42", time.Hour)
	req := httptest.NewRequest(http.MethodPost, signed, strings.NewReader(""))
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, want 401", rec.Code)
	}
}
