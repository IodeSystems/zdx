package tokencmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// fakeServer is a minimal in-memory implementation of /api/admin/tokens that
// the tokencmd CLI exercises via the typed dxclient.
type fakeServer struct {
	mu     sync.Mutex
	nextID int32
	tokens []map[string]any
}

func (f *fakeServer) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")

		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/admin/tokens":
			var body struct {
				ProjectSlug string `json:"project_slug"`
				Name        string `json:"name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.nextID++
			name := body.Name
			if name == "" {
				name = "admin-token"
			}
			t := map[string]any{
				"id":            f.nextID,
				"name":          name,
				"project_scope": []string{body.ProjectSlug},
				"created_at":    "2026-05-01T00:00:00Z",
				"token":         "raw-secret-token-value",
			}
			f.tokens = append(f.tokens, t)
			_ = json.NewEncoder(w).Encode(t)
		case r.Method == http.MethodGet && r.URL.Path == "/api/admin/tokens":
			out := make([]map[string]any, 0, len(f.tokens))
			for _, t := range f.tokens {
				view := map[string]any{
					"id":            t["id"],
					"name":          t["name"],
					"project_scope": t["project_scope"],
					"created_at":    t["created_at"],
					"user_email":    "admin@test",
				}
				out = append(out, view)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tokens": out})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/admin/tokens/"):
			id := strings.TrimPrefix(r.URL.Path, "/api/admin/tokens/")
			for i, t := range f.tokens {
				if jsonNumberEq(t["id"], id) {
					f.tokens = append(f.tokens[:i], f.tokens[i+1:]...)
					w.Header().Del("Content-Type")
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{"detail": "no such token"})
		default:
			http.NotFound(w, r)
		}
	})
}

func jsonNumberEq(stored any, want string) bool {
	switch v := stored.(type) {
	case int32:
		return want == intToStr(int(v))
	case int:
		return want == intToStr(v)
	}
	return false
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

func TestTokenMintListRevokeRoundTrip(t *testing.T) {
	fs := &fakeServer{}
	srv := httptest.NewServer(fs.handler())
	defer srv.Close()

	c := cli.NewClient(srv.URL, "test-token")
	ctx := context.Background()

	// Mint: token to stdout, warning to stderr.
	var stdout, stderr bytes.Buffer
	if err := runMint(ctx, c, "demo", "agent-1", &stdout, &stderr); err != nil {
		t.Fatalf("mint: %v", err)
	}
	if got := strings.TrimRight(stdout.String(), "\n"); got != "raw-secret-token-value" {
		t.Errorf("mint stdout = %q, want raw-secret-token-value", got)
	}
	if !strings.Contains(stderr.String(), "Token shown once") {
		t.Errorf("mint stderr missing warning: %q", stderr.String())
	}

	// Pipe-test: stdout contains ONLY the token (no banner mixed in).
	if strings.Contains(stdout.String(), "Token shown once") {
		t.Errorf("mint stdout leaked stderr warning: %q", stdout.String())
	}

	// List should show one row with the project_scope.
	var listOut bytes.Buffer
	if err := runList(ctx, c, &listOut); err != nil {
		t.Fatalf("list: %v", err)
	}
	listBody := listOut.String()
	for _, want := range []string{"agent-1", "admin@test", "demo"} {
		if !strings.Contains(listBody, want) {
			t.Errorf("list missing %q: %s", want, listBody)
		}
	}

	// Revoke: 204 → "revoked".
	var revokeOut bytes.Buffer
	if err := runRevoke(ctx, c, "1", &revokeOut); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if got := strings.TrimSpace(revokeOut.String()); got != "revoked" {
		t.Errorf("revoke stdout = %q, want revoked", got)
	}

	// Second revoke: 404 → "no such token".
	var revokeOut2 bytes.Buffer
	err := runRevoke(ctx, c, "1", &revokeOut2)
	if err == nil {
		t.Fatal("expected error on second revoke, got nil")
	}
	if !strings.Contains(err.Error(), "no such token") {
		t.Errorf("second revoke error = %v, want 'no such token'", err)
	}
}

func TestTokenMintRequiresProject(t *testing.T) {
	c := cli.NewClient("http://unused", "")
	var stdout, stderr bytes.Buffer
	err := runMint(context.Background(), c, "", "", &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when --project is empty")
	}
	if !strings.Contains(err.Error(), "--project is required") {
		t.Errorf("error = %v, want --project required", err)
	}
}

func TestTokenListEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"tokens": []map[string]any{}})
	}))
	defer srv.Close()
	c := cli.NewClient(srv.URL, "")
	var out bytes.Buffer
	if err := runList(context.Background(), c, &out); err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out.String(), "no tokens") {
		t.Errorf("expected 'no tokens', got %q", out.String())
	}
}
