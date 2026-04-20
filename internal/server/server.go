package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
	"github.com/iodesystems/zdx-go/internal/server/handlers"
	"github.com/iodesystems/zdx-go/internal/ws"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

type Server struct {
	pool           *pgxpool.Pool
	q              *db.Queries
	mux            *chi.Mux
	buildSHA       string
	zdxProjectSlug string
	uploadsDir     string
	reposDir       string
	slot           string // "current", "next", or "" (dev); controls WS endpoint registration
	wsSecret       string
	broker         ws.Broker
	api            huma.API
	emb            *embedder
	features       handlers.SchemaFeatures
	sink           timingSink
	ingestLimiter  *ingestRateLimiter
	errorClient    *zdxclient.Client

	wsClientsMu sync.Mutex
	wsClients   map[int64]*wsClientEntry
	wsClientSeq int64
}

// wsClientEntry tracks a single live WebSocket connection for admin diagnostics.
// A client may hold multiple channel subscriptions on the single multiplexed
// connection.
type wsClientEntry struct {
	ID          int64
	Channels    []string
	UserID      int64
	RemoteAddr  string
	ConnectedAt time.Time
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
	reposDir := os.Getenv("ZDX_REPOS_DIR")
	if reposDir == "" {
		if zdxHome := os.Getenv("ZDX_HOME"); zdxHome != "" {
			reposDir = zdxHome + "/data/repos"
		} else {
			reposDir = "repos"
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
		pool:           pool,
		q:              db.New(pool),
		mux:            chi.NewMux(),
		buildSHA:       buildSHA,
		zdxProjectSlug: os.Getenv("ZDX_PROJECT_SLUG"),
		uploadsDir:     uploadsDir,
		reposDir:       reposDir,
		slot:           os.Getenv("ZDX_SLOT"),
		wsSecret:       wsSecret,
		broker:         ws.NewBroker(os.Getenv("ZDX_VALKEY_ADDR")),
		emb:            newEmbedder(vecDir, sink),
		features:       detectFeatures(ctx, pool),
		sink:           sink,
		ingestLimiter:  newIngestRateLimiter(1000, 10000),
		wsClients:      make(map[int64]*wsClientEntry),
	}

	// Load LLM config eagerly so embedder is ready on first request.
	s.ReloadEmbedder(ctx)
	// Index any issues not yet in the vector store (e.g. pre-dating LLM config setup).
	go s.ReindexAllIssues()

	cfg := huma.DefaultConfig("ZDX API", "1.0.0")
	cfg.Info.Description = "zdx developer-experience platform API"

	s.mux.Use(s.errorMiddleware)
	s.mux.Use(s.sourceMiddleware)
	s.mux.Use(s.apiKeyMiddleware)
	s.mux.Use(s.adminMiddleware)
	s.mux.Use(s.timingMiddleware)
	s.api = humachi.New(s.mux, cfg)

	handlers.Register(s.api, s.buildDeps())
	s.registerIngestRoutes(s.api)
	s.registerCounterIngestRoutes(s.api)
	s.registerErrorIngestRoutes(s.api)
	s.registerLogIngestRoutes(s.api)
	s.registerPromIngestRoutes(s.api)
	s.registerWSRoutes(s.api)

	s.mux.NotFound(notFoundHandler(staticDir))

	return s
}

// notFoundHandler returns the router-level 404 handler: JSON 404 for unmatched
// /api/* requests, SPA index.html fallback for everything else. Serving index.html
// for unknown /api/* paths would cause clients to treat missing endpoints as success.
func notFoundHandler(staticDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			writeAPINotFound(w, r.URL.Path)
			return
		}
		if staticDir != "" {
			http.ServeFileFS(w, r, os.DirFS(staticDir), spaPath(r.URL.Path, staticDir))
			return
		}
		http.NotFound(w, r)
	}
}

// writeAPINotFound emits a JSON 404 for unmatched /api/* requests.
func writeAPINotFound(w http.ResponseWriter, path string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	// Hand-rolled JSON: path is a URL path, so we only need to escape backslashes and quotes.
	escaped := strings.ReplaceAll(path, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	fmt.Fprintf(w, `{"error":"not found","path":"%s"}`, escaped)
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

// buildDeps constructs the Deps struct handed to the handlers package so every
// HTTP handler sees the same state the Server is holding. Server satisfies the
// Broker / Reconciler / IngestRegistrar interfaces directly; *embedder
// satisfies Embedder via the exported Upsert*/TopN*/Complete/Reload methods.
func (s *Server) buildDeps() *handlers.Deps {
	return &handlers.Deps{
		Pool:            s.pool,
		Q:               s.q,
		Features:        s.features,
		Emb:             s.emb,
		Broker:          s,
		Reconciler:      s,
		IngestRegistrar: s,
		ErrorClient:     s.errorClient,
		BuildSHA:        s.buildSHA,
		ZDXProjectSlug:  s.zdxProjectSlug,
		UploadsDir:      s.uploadsDir,
		ReposDir:        s.reposDir,
		Slot:            s.slot,
		WSSecret:        s.wsSecret,
		Mux:             s.mux,
	}
}

// reloadEmbedder re-reads the LLM config from DB and refreshes the embedder client.
// If triggerReindex is true and a valid config is present, bulk-indexes all existing
// issues in the background (needed after first-time LLM config, or config change).
func (s *Server) ReloadEmbedder(ctx context.Context) {
	if !s.features.HasLLMConfig {
		s.emb.Reload(nil)
		return
	}
	cfg, err := s.q.GetPrimaryLLMConfigWithEmbedding(ctx)
	if err != nil {
		s.emb.Reload(nil)
		return
	}
	s.emb.Reload(&llm.Config{
		Type:   cfg.Type,
		URL:    cfg.Url,
		Model:  cfg.EmbeddingModel.String,
		APIKey: cfg.ApiKey,
	})
}

// reindexAllIssues bulk-indexes all open issues and questions across all projects.
func (s *Server) ReindexAllIssues() {
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
			s.emb.UpsertIssue(ctx, p.ID, issID, text)
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
			s.emb.UpsertQuestion(ctx, p.ID, q.ID, q.Question)
			qIndexed++
		}
		if qIndexed > 0 {
			log.Printf("reindex: indexed %d questions for project %s", qIndexed, p.Slug)
		}
	}
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

// isSignedAssetPath returns true for asset routes that may be served via signed URL
// (no X-Api-Key required). Limited to read-only artifact endpoints; everything else
// must still authenticate.
func isSignedAssetPath(path string) bool {
	return strings.HasPrefix(path, "/api/files/") ||
		strings.HasPrefix(path, "/api/dx/demos/cli/") ||
		strings.HasPrefix(path, "/api/dx/demos/video/")
}

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
		// Signed asset URLs bypass the X-Api-Key requirement so <video src> and bare fetch
		// can load demo artifacts. Only GETs to whitelisted asset paths are eligible; the
		// signature is HMAC'd with s.wsSecret and carries an expiry.
		if r.Method == http.MethodGet && isSignedAssetPath(path) && handlers.VerifyAssetURL(s.wsSecret, r) {
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
		ctx := context.WithValue(r.Context(), handlers.CtxAPIKeyID, key.ID)
		ctx = context.WithValue(ctx, handlers.CtxUserID, key.UserID)
		if u, uErr := s.q.GetUserByID(r.Context(), key.UserID); uErr == nil {
			ctx = context.WithValue(ctx, handlers.CtxUserRole, u.Role)
		}
		if v := r.Header.Get("X-ZDX-Agent-Id"); v != "" {
			ctx = context.WithValue(ctx, handlers.CtxAgentID, v)
		}
		if v := r.Header.Get("X-ZDX-Session-Id"); v != "" {
			ctx = context.WithValue(ctx, handlers.CtxSessionID, v)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// adminMiddleware rejects non-admin users from /api/admin/ routes.
func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/admin/") {
			role := handlers.UserRoleFromContext(r.Context())
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
		rctx := chi.RouteContext(r.Context())
		source := r.URL.Path
		if rctx != nil && rctx.RoutePattern() != "" {
			source = rctx.RoutePattern()
		}
		next.ServeHTTP(w, r.WithContext(handlers.WithSource(r.Context(), source)))
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
		// Use route template pattern for batching; fall back to literal path.
		pattern := path
		if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
			pattern = rctx.RoutePattern()
		}
		s.sink.send(Timing{
			Name:       "http:" + r.Method + " " + pattern,
			DurationMs: durationMs,
			Source:     pattern,
		})
	})
}

// SetErrorClient wires the zdxclient used for self-reporting errors.
// Called from main.go after the self-integration client is created.
func (s *Server) SetErrorClient(c *zdxclient.Client) {
	s.errorClient = c
}

// statusCapture wraps http.ResponseWriter to capture the status code.
type statusCapture struct {
	http.ResponseWriter
	code int
}

func (w *statusCapture) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// errorMiddleware catches panics and 5xx responses, reporting them through
// the self-integration zdxclient so server errors appear on the errors page.
func (s *Server) errorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		sc := &statusCapture{ResponseWriter: w, code: 200}
		defer func() {
			if rec := recover(); rec != nil {
				stack := fmt.Sprintf("%v", rec)
				log.Printf("panic: %s %s: %s", r.Method, r.URL.Path, stack)
				if s.errorClient != nil {
					s.errorClient.RecordErrorWithStack(
						"panic",
						fmt.Sprintf("%v", rec),
						stack,
						r.Method+" "+r.URL.Path,
						nil,
					)
				}
				http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(sc, r)
		if sc.code >= 500 && s.errorClient != nil {
			s.errorClient.RecordError(
				fmt.Sprintf("http:%d", sc.code),
				fmt.Sprintf("%s %s returned %d", r.Method, r.URL.Path, sc.code),
				nil,
			)
		}
	})
}
