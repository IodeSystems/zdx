package server

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mailru/easyjson"

	"github.com/iodesystems/zdx-go/internal/db"
)

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

type Server struct {
	q         *db.Queries
	mux       *http.ServeMux
	staticDir string
}

func New(pool *pgxpool.Pool, staticDir string) *Server {
	s := &Server{q: db.New(pool), mux: http.NewServeMux(), staticDir: staticDir}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if isMaintenance() && r.URL.Path != "/health" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write(maintenancePage)
		return
	}
	// API and health routes go to the mux directly.
	if r.URL.Path == "/health" || strings.HasPrefix(r.URL.Path, "/api/") {
		s.mux.ServeHTTP(w, r)
		return
	}
	// SPA fallback: serve static files; unknown paths → index.html.
	if s.staticDir != "" {
		http.ServeFileFS(w, r, os.DirFS(s.staticDir), spaPath(r.URL.Path, s.staticDir))
		return
	}
	s.mux.ServeHTTP(w, r)
}

// spaPath returns the file path to serve for a given URL path.
// If the file exists in staticDir, serve it directly; otherwise serve index.html.
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

func (s *Server) routes() {
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Projects
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("POST /api/project", s.handleCreateProject)

	// Issues
	s.mux.HandleFunc("GET /api/dx/issues", s.handleListIssues)
	s.mux.HandleFunc("GET /api/dx/issue", s.handleGetIssue)
	s.mux.HandleFunc("POST /api/dx/issue", s.handleCreateIssue)
	s.mux.HandleFunc("PUT /api/dx/issue", s.handleUpdateIssue)
	s.mux.HandleFunc("POST /api/dx/issue/close", s.handleCloseIssue)
	s.mux.HandleFunc("POST /api/dx/issue/work", s.handleAppendIssueWork)

	// Tasks
	s.mux.HandleFunc("GET /api/dx/tasks", s.handleListTasks)
	s.mux.HandleFunc("POST /api/dx/task", s.handleCreateTask)
	s.mux.HandleFunc("PUT /api/dx/task/status", s.handleUpdateTaskStatus)
	s.mux.HandleFunc("POST /api/dx/task/done", s.handleMarkTaskDone)
	s.mux.HandleFunc("POST /api/dx/task/undone", s.handleMarkTaskUndone)

	// Features
	s.mux.HandleFunc("GET /api/dx/features", s.handleListFeatures)
	s.mux.HandleFunc("GET /api/dx/feature", s.handleGetFeature)
	s.mux.HandleFunc("POST /api/dx/feature", s.handleCreateFeature)
	s.mux.HandleFunc("PUT /api/dx/feature/field", s.handleUpdateFeatureField)
	s.mux.HandleFunc("POST /api/dx/spec", s.handleAddSpec)

	// Solo queue
	s.mux.HandleFunc("GET /api/dx/solo", s.handleSolo)
}

// ── response helpers ──────────────────────────────────────────────────────────

func ok(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if m, ok2 := v.(easyjson.Marshaler); ok2 {
		b, err := easyjson.Marshal(m)
		if err == nil {
			w.Write(b)
			return
		}
	}
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func bind[T any](r *http.Request) (T, error) {
	var v T
	return v, json.NewDecoder(r.Body).Decode(&v)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ok(w, map[string]string{"status": "ok"})
}
