package server

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/db"
)

type Server struct {
	q   *db.Queries
	mux *chi.Mux
}

func New(pool *pgxpool.Pool, staticDir string) *Server {
	s := &Server{q: db.New(pool), mux: chi.NewMux()}

	cfg := huma.DefaultConfig("ZDX API", "1.0.0")
	cfg.Info.Description = "zdx developer-experience platform API"
	api := humachi.New(s.mux, cfg)

	s.registerRoutes(api)

	// SPA fallback: anything not under /api/ or /openapi* serves static files.
	if staticDir != "" {
		s.mux.NotFound(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFileFS(w, r, os.DirFS(staticDir), spaPath(r.URL.Path, staticDir))
		})
	}

	return s
}

// spaPath returns the file path relative to staticDir for the given URL path.
// Falls back to index.html for unknown paths.
func spaPath(urlPath, staticDir string) string {
	candidate := strings.TrimPrefix(urlPath, "/")
	if candidate == "" {
		candidate = "index.html"
	}
	if _, err := os.Stat(staticDir + "/" + candidate); err == nil {
		return candidate
	}
	return "index.html"
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isMaintenance() && r.URL.Path != "/health" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write(maintenancePage) //nolint:errcheck
		return
	}
	s.mux.ServeHTTP(w, r)
}

func isMaintenance() bool {
	return os.Getenv("MAINTENANCE") == "true"
}

var maintenancePage = []byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width,initial-scale=1">
  <title>Maintenance</title>
  <style>
    *{box-sizing:border-box;margin:0;padding:0}
    body{font-family:system-ui,sans-serif;background:#0f0f0f;color:#e0e0e0;
         display:flex;align-items:center;justify-content:center;min-height:100vh}
    .card{text-align:center;padding:3rem 4rem;border:1px solid #2a2a2a;border-radius:8px;
          background:#161616;max-width:480px}
    h1{font-size:1.5rem;font-weight:600;margin-bottom:.75rem}
    p{color:#888;line-height:1.6;font-size:.95rem}
    .dot{display:inline-block;width:8px;height:8px;border-radius:50%;
         background:#f59e0b;margin-right:.5rem;animation:pulse 2s infinite}
    @keyframes pulse{0%,100%{opacity:1}50%{opacity:.3}}
  </style>
</head>
<body>
  <div class="card">
    <p><span class="dot"></span>Maintenance in progress</p>
    <h1>Back shortly</h1>
    <p>We are applying a database migration.<br>This usually takes less than a minute.</p>
  </div>
</body>
</html>`)

// ── helpers used by handlers ───────────────────────────────────────────────

func apiErr(status int, msg string) huma.StatusError {
	return huma.NewError(status, msg)
}

func getProject(ctx context.Context, q *db.Queries, slug string) (db.ZdxProject, error) {
	p, err := q.GetProjectBySlug(ctx, slug)
	if err != nil {
		return p, huma.NewError(http.StatusNotFound, "project not found: "+slug)
	}
	return p, nil
}
