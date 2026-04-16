package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNotFoundHandler_APIReturnsJSON404(t *testing.T) {
	h := notFoundHandler("")

	cases := []struct {
		name, method, path string
	}{
		{"POST unknown api", http.MethodPost, "/api/does-not-exist"},
		{"GET unknown api", http.MethodGet, "/api/dx/missing"},
		{"DELETE unknown api", http.MethodDelete, "/api/foo"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("status=%d, want 404", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
				t.Errorf("Content-Type=%q, want application/json", ct)
			}
			body := rec.Body.String()
			if !strings.Contains(body, `"error":"not found"`) {
				t.Errorf("body missing error field: %q", body)
			}
			if !strings.Contains(body, `"path":"`+tc.path+`"`) {
				t.Errorf("body missing path=%q: %q", tc.path, body)
			}
		})
	}
}

func TestNotFoundHandler_SPAFallbackServesIndex(t *testing.T) {
	dir := t.TempDir()
	indexBody := "<!doctype html><html><body>spa-index</body></html>"
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(indexBody), 0o644); err != nil {
		t.Fatal(err)
	}

	h := notFoundHandler(dir)
	req := httptest.NewRequest(http.MethodGet, "/unknown-route", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "spa-index") {
		t.Errorf("body did not contain index.html contents: %q", body)
	}
}

func TestNotFoundHandler_NoStaticDirReturnsPlain404(t *testing.T) {
	h := notFoundHandler("")
	req := httptest.NewRequest(http.MethodGet, "/unknown-route", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "application/json") {
		t.Errorf("non-/api/ path should not return JSON, got Content-Type=%q", ct)
	}
}

func TestWriteAPINotFound_EscapesQuotesAndBackslashes(t *testing.T) {
	rec := httptest.NewRecorder()
	writeAPINotFound(rec, `/api/weird"path\with-chars`)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
	want := `{"error":"not found","path":"/api/weird\"path\\with-chars"}`
	if got := rec.Body.String(); got != want {
		t.Errorf("body=%q\n want=%q", got, want)
	}
}
