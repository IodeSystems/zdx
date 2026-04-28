package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestExtensionForMimeType(t *testing.T) {
	cases := []struct {
		mime string
		want string
	}{
		{"image/png", ".png"},
		{"image/jpeg", ".jpg"},
		{"image/gif", ".gif"},
		{"image/webp", ".webp"},
		{"application/json", ".json"},
		{"application/json; charset=utf-8", ".json"}, // strips parameters
		{"video/webm", ".webm"},
		{"video/mp4", ".mp4"},
		{"text/plain", ".txt"},
		{"VIDEO/WEBM", ".webm"},   // case-insensitive
		{"  image/png  ", ".png"}, // trims whitespace
		{"application/octet-stream", ".bin"},
		{"", ".bin"},
	}
	for _, tc := range cases {
		t.Run(tc.mime, func(t *testing.T) {
			got := extensionForMimeType(tc.mime)
			if got != tc.want {
				t.Errorf("extensionForMimeType(%q) = %q, want %q", tc.mime, got, tc.want)
			}
		})
	}
}

func TestContentDispositionInline(t *testing.T) {
	got := contentDispositionInline("demo-42.webm")
	want := `inline; filename="demo-42.webm"; filename*=UTF-8''demo-42.webm`
	if got != want {
		t.Errorf("contentDispositionInline = %q, want %q", got, want)
	}

	// Unicode filename: quoted-string keeps raw chars; filename* percent-encodes.
	got = contentDispositionInline("Café.json")
	if !strings.HasPrefix(got, `inline; filename="Café.json"; filename*=UTF-8''`) {
		t.Errorf("contentDispositionInline(unicode) prefix wrong: %q", got)
	}
	if !strings.Contains(got, "Caf%C3%A9.json") {
		t.Errorf("contentDispositionInline(unicode) missing percent-encoded UTF-8: %q", got)
	}

	// Unsafe chars (quote, backslash, CR, LF) are stripped so the header stays
	// well-formed and quoted-string parsing is unambiguous.
	got = contentDispositionInline("evil\"\\\r\nname.txt")
	if strings.Count(got, `"`) != 2 {
		t.Errorf("contentDispositionInline(unsafe) = %q, expected exactly two quotes", got)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Errorf("contentDispositionInline(unsafe) leaked CR/LF: %q", got)
	}
	if strings.Contains(got, "\\") {
		t.Errorf("contentDispositionInline(unsafe) leaked backslash: %q", got)
	}
}

// TestHandleServeDemo_SetsContentDisposition verifies that downloads of demo
// artifacts include a Content-Disposition header with the correct filename and
// extension so mobile download managers don't fall back to .txt (IS-344).
func TestHandleServeDemo_SetsContentDisposition(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("DEMOS_DIR", dir)

	if err := os.MkdirAll(filepath.Join(dir, "cli"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "api"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "video"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite := func(p, body string) {
		if err := os.WriteFile(filepath.Join(dir, p), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("cli/abc123.json", `{"steps":[]}`)
	mustWrite("api/legacy001.json", `{"steps":[]}`)
	mustWrite("video/vid42.webm", "WEBM-FAKE")
	mustWrite("video/vid43.mp4", "MP4-FAKE")

	h := &Handler{}

	cases := []struct {
		name        string
		demoType    string
		demoName    string
		wantStatus  int
		wantCType   string
		wantDispExt string
	}{
		{"cli json", "cli", "abc123", http.StatusOK, "application/json", ".json"},
		{"api legacy json", "api", "legacy001", http.StatusOK, "application/json", ".json"},
		{"video webm", "video", "vid42", http.StatusOK, "video/webm", ".webm"},
		{"video mp4", "video", "vid43", http.StatusOK, "video/mp4", ".mp4"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := newChiRequest(http.MethodGet,
				"/api/dx/demos/"+tc.demoType+"/"+tc.demoName,
				map[string]string{"type": tc.demoType, "name": tc.demoName})
			rec := httptest.NewRecorder()

			h.handleServeDemo(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status=%d, want %d; body=%s", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if ct := rec.Header().Get("Content-Type"); ct != tc.wantCType {
				t.Errorf("Content-Type=%q, want %q", ct, tc.wantCType)
			}
			disp := rec.Header().Get("Content-Disposition")
			if disp == "" {
				t.Fatal("Content-Disposition header missing")
			}
			if !strings.HasPrefix(disp, "inline; ") {
				t.Errorf("Content-Disposition=%q, want inline disposition (preserves <video>/<img> rendering)", disp)
			}
			wantName := tc.demoName + tc.wantDispExt
			if !strings.Contains(disp, `filename="`+wantName+`"`) {
				t.Errorf("Content-Disposition=%q, missing filename=%q", disp, wantName)
			}
		})
	}
}

func TestHandleServeDemo_RejectsBadType(t *testing.T) {
	h := &Handler{}
	req := newChiRequest(http.MethodGet, "/api/dx/demos/garbage/foo",
		map[string]string{"type": "garbage", "name": "foo"})
	rec := httptest.NewRecorder()

	h.handleServeDemo(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

// newChiRequest constructs an *http.Request with a chi RouteContext attached so
// chi.URLParam(r, name) resolves during direct handler unit tests (i.e. without
// routing through chi.Mux).
func newChiRequest(method, target string, params map[string]string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	rctx := chi.NewRouteContext()
	for k, v := range params {
		rctx.URLParams.Add(k, v)
	}
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
