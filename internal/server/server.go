package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
	"github.com/iodesystems/zdx-go/internal/ws"
)

type Server struct {
	q              *db.Queries
	mux            *chi.Mux
	buildSHA       string
	zdxProjectSlug string
	uploadsDir     string
	slot           string // "current", "next", or "" (dev); controls WS endpoint registration
	wsSecret       string
	broker         ws.Broker
	api            huma.API
	emb            *embedder
	features       SchemaFeatures
	sink           timingSink
	ingestLimiter  *ingestRateLimiter
}

func New(pool *pgxpool.Pool, sink timingSink, staticDir, buildSHA string) *Server {
	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		if zdxHome := os.Getenv("ZDX_HOME"); zdxHome != "" {
			uploadsDir = zdxHome + "/data/files"
		} else {
			uploadsDir = "uploads"
		}
	}
	vecDir := os.Getenv("VEC_DIR")
	if vecDir == "" {
		if zdxHome := os.Getenv("ZDX_HOME"); zdxHome != "" {
			vecDir = zdxHome + "/data/vec"
		} else {
			vecDir = "vec"
		}
	}

	wsSecret := os.Getenv("ZDX_WS_SECRET")
	if wsSecret == "" {
		wsSecret, _ = ws.GenerateSecret()
	}

	ctx := context.Background()
	s := &Server{
		q:              db.New(pool),
		mux:            chi.NewMux(),
		buildSHA:       buildSHA,
		zdxProjectSlug: os.Getenv("ZDX_PROJECT_SLUG"),
		uploadsDir:     uploadsDir,
		slot:           os.Getenv("ZDX_SLOT"),
		wsSecret:       wsSecret,
		broker:         ws.NewBroker(os.Getenv("ZDX_VALKEY_ADDR")),
		emb:            newEmbedder(vecDir, sink),
		features:       detectFeatures(ctx, pool),
		sink:           sink,
		ingestLimiter:  newIngestRateLimiter(1000, 10000),
	}

	// Load LLM config eagerly so embedder is ready on first request.
	s.reloadEmbedder(ctx)
	// Index any issues not yet in the vector store (e.g. pre-dating LLM config setup).
	go s.reindexAllIssues()

	cfg := huma.DefaultConfig("ZDX API", "1.0.0")
	cfg.Info.Description = "zdx developer-experience platform API"

	s.mux.Use(s.sourceMiddleware)
	s.mux.Use(s.apiKeyMiddleware)
	s.mux.Use(s.adminMiddleware)
	s.mux.Use(s.timingMiddleware)
	s.api = humachi.New(s.mux, cfg)

	s.registerRoutes(s.api)
	s.registerWSRoutes(s.api)

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

// IsWSEnabled reports whether this server instance should register WebSocket endpoints.
// WS is disabled on the "next" deploy slot to keep load segregated during rolling deploys.
func (s *Server) IsWSEnabled() bool {
	return s.slot != "next"
}

// reloadEmbedder re-reads the LLM config from DB and refreshes the embedder client.
// If triggerReindex is true and a valid config is present, bulk-indexes all existing
// issues in the background (needed after first-time LLM config, or config change).
func (s *Server) reloadEmbedder(ctx context.Context) {
	if !s.features.HasLLMConfig {
		s.emb.reload(nil)
		return
	}
	cfg, err := s.q.GetLLMConfig(ctx)
	if err != nil {
		s.emb.reload(nil)
		return
	}
	s.emb.reload(&llm.Config{
		Type:   cfg.Type,
		URL:    cfg.Url,
		Model:  cfg.Model,
		APIKey: cfg.ApiKey,
	})
}

// reindexAllIssues bulk-indexes all open issues and questions across all projects.
func (s *Server) reindexAllIssues() {
	ctx := context.Background()
	projects, err := s.q.ListProjects(ctx)
	if err != nil {
		log.Printf("reindex: list projects: %v", err)
		return
	}
	for _, p := range projects {
		issues, err := s.q.ListIssues(ctx, p.ID)
		if err != nil {
			log.Printf("reindex: list issues for %s: %v", p.Slug, err)
			continue
		}
		indexed := 0
		for _, iss := range issues {
			text := iss.Title
			if text == "" {
				text = iss.Context
			}
			if text == "" {
				continue
			}
			issID := fmt.Sprintf("IS-%s", iss.ID)
			s.emb.upsertIssue(ctx, p.ID, issID, text)
			indexed++
		}
		if indexed > 0 {
			log.Printf("reindex: indexed %d issues for project %s", indexed, p.Slug)
		}

		questions, err := s.q.ListQuestions(ctx, p.ID)
		if err != nil {
			log.Printf("reindex: list questions for %s: %v", p.Slug, err)
			continue
		}
		qIndexed := 0
		for _, q := range questions {
			if q.Question == "" {
				continue
			}
			s.emb.upsertQuestion(ctx, p.ID, q.ID, q.Question)
			qIndexed++
		}
		if qIndexed > 0 {
			log.Printf("reindex: indexed %d questions for project %s", qIndexed, p.Slug)
		}
	}
}

// findSimilarIssues embeds queryText and returns the top-n similar open issues.
func (s *Server) findSimilarIssues(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarIssueItem, error) {
	results, err := s.emb.topN(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarIssueItem{}, nil
	}
	// Fetch titles for the returned IDs.
	out := make([]SimilarIssueItem, 0, len(results))
	for _, r := range results {
		id := issueIDFromInt(int32(r.ID)) //nolint:gosec
		iss, err := s.q.GetIssue(ctx, db.GetIssueParams{ProjectID: projectID, ID: id})
		if err != nil {
			continue // stale index entry — skip
		}
		out = append(out, SimilarIssueItem{ID: id, Title: iss.Title, Context: iss.Context, Status: iss.Status, Score: r.Score})
	}
	return out, nil
}

type SimilarQuestionItem struct {
	ID       int32   `json:"id"`
	Question string  `json:"question"`
	Answer   string  `json:"answer"`
	Score    float32 `json:"score"`
}

func (s *Server) findSimilarQuestions(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarQuestionItem, error) {
	results, err := s.emb.topNQuestions(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarQuestionItem{}, nil
	}
	out := make([]SimilarQuestionItem, 0, len(results))
	for _, r := range results {
		q, err := s.q.GetQuestion(ctx, db.GetQuestionParams{ProjectID: projectID, ID: int32(r.ID)}) //nolint:gosec
		if err != nil {
			continue
		}
		out = append(out, SimilarQuestionItem{
			ID:       q.ID,
			Question: q.Question,
			Answer:   q.Answer.String,
			Score:    r.Score,
		})
	}
	return out, nil
}

func (s *Server) findSimilarTasks(ctx context.Context, projectID int32, queryText string, n int) ([]SimilarTaskItem, error) {
	results, err := s.emb.topNTasks(ctx, projectID, queryText, n)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return []SimilarTaskItem{}, nil
	}
	out := make([]SimilarTaskItem, 0, len(results))
	for _, r := range results {
		id := taskIDFromInt(int32(r.ID)) //nolint:gosec
		task, err := s.q.GetTask(ctx, id)
		if err != nil {
			continue
		}
		out = append(out, SimilarTaskItem{ID: id, Text: task.Text, Status: task.Status, Issue: task.Issue, Score: r.Score})
	}
	return out, nil
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

// ── Auth middleware ────────────────────────────────────────────────────────

type contextKey int

const (
	ctxAPIKeyID   contextKey = 1
	ctxUserID     contextKey = 2
	ctxQueryStart contextKey = 3
	ctxSource     contextKey = 4
	ctxUserRole   contextKey = 5
	ctxSkipTiming contextKey = 6
)

// apiKeyMiddleware validates X-Api-Key on /api/* requests, except health, openapi, and setup/bootstrap.
// Non-/api/ paths (SPA, static assets) pass through without auth.
func (s *Server) apiKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") ||
			(r.Method == http.MethodGet && (path == "/api/health" || path == "/api/error" || strings.HasPrefix(path, "/openapi"))) ||
			(r.Method == http.MethodPost && (path == "/api/setup/bootstrap" || path == "/api/auth/login" || path == "/api/auth/register" || path == "/api/dx/errors/report" || path == "/api/ingest/timings")) {
			next.ServeHTTP(w, r)
			return
		}
		token := r.Header.Get("X-Api-Key")
		if token == "" {
			http.Error(w, `{"title":"Unauthorized","status":401}`, http.StatusUnauthorized)
			return
		}
		key, err := s.q.GetApiKeyByToken(r.Context(), token)
		if err != nil {
			http.Error(w, `{"title":"Unauthorized","status":401}`, http.StatusUnauthorized)
			return
		}
		// Fire-and-forget last_used_at update; don't block the request.
		go func() { _ = s.q.TouchApiKey(context.Background(), key.ID) }()
		ctx := context.WithValue(r.Context(), ctxAPIKeyID, key.ID)
		ctx = context.WithValue(ctx, ctxUserID, key.UserID)
		if u, uErr := s.q.GetUserByID(r.Context(), key.UserID); uErr == nil {
			ctx = context.WithValue(ctx, ctxUserRole, u.Role)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminMiddleware rejects non-admin users from /api/admin/ routes.
func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin/") {
			role := ctxUserRoleVal(r.Context())
			if role != "admin" {
				http.Error(w, `{"title":"Forbidden","status":403,"detail":"admin role required"}`, http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// ── Timing middleware ──────────────────────────────────────────────────────

// sourceMiddleware stamps the request path into ctx so the QueryTracer can
// attribute every downstream query to a real URL instead of "background".
// Runs before any handler or downstream middleware that touches the DB.
func (s *Server) sourceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(WithSource(r.Context(), r.URL.Path)))
	})
}

// timingMiddleware records the wall-clock duration of each /api/ request.
// Health and openapi probes are skipped; sub-10ms requests are dropped as
// uninteresting. Everything else is shipped to the sink for upsert by the
// drainer. SQL-level timings are captured separately by the QueryTracer.
func (s *Server) timingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if !strings.HasPrefix(path, "/api/") ||
			path == "/api/health" ||
			path == "/api/ingest/timings" ||
			strings.HasPrefix(path, "/openapi") {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		next.ServeHTTP(w, r)
		durationMs := int32(time.Since(start).Milliseconds())
		if durationMs < 10 {
			return
		}
		s.sink.send(Timing{
			Name:       "http:" + r.Method + " " + path,
			DurationMs: durationMs,
			Source:     path,
		})
	})
}

// ── helpers used by handlers ───────────────────────────────────────────────

func apiErr(status int, msg string) huma.StatusError {
	return huma.NewError(status, msg)
}

func ctxUserIDVal(ctx context.Context) int32 {
	v, _ := ctx.Value(ctxUserID).(int32)
	return v
}

func ctxUserRoleVal(ctx context.Context) string {
	v, _ := ctx.Value(ctxUserRole).(string)
	return v
}

func getProject(ctx context.Context, q *db.Queries, slug string) (db.ZdxProject, error) {
	p, err := q.GetProjectBySlug(ctx, slug)
	if err != nil {
		return p, huma.NewError(http.StatusNotFound, "project not found: "+slug)
	}
	return p, nil
}
