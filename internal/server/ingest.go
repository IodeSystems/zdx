package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/golang/snappy"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/prometheus/prometheus/prompb"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/handlers"
)

// ── Rate limiter ──────────────────────────────────────────────────────────

// ingestRateLimiter is a per-token token-bucket. All buckets share the same
// rate/burst for v1. Tokens that have never ingested get a fresh full bucket
// on first hit — buckets never expire but the map is bounded by the number
// of issued tokens, which is negligible for a dev portal.
type ingestRateLimiter struct {
	mu      sync.Mutex
	buckets map[int32]*rlBucket
	rate    float64 // tokens refilled per second
	burst   float64 // max tokens in bucket
}

type rlBucket struct {
	tokens float64
	last   time.Time
}

func newIngestRateLimiter(rate, burst float64) *ingestRateLimiter {
	return &ingestRateLimiter{
		buckets: make(map[int32]*rlBucket),
		rate:    rate,
		burst:   burst,
	}
}

// allow deducts cost tokens from the bucket for tokenID and returns whether
// there was room. Cost scales with the number of events in a batch so large
// batches are accounted for correctly.
func (l *ingestRateLimiter) allow(tokenID int32, cost float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[tokenID]
	now := time.Now()
	if !ok {
		b = &rlBucket{tokens: l.burst, last: now}
		l.buckets[tokenID] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rate)
	b.last = now
	if b.tokens < cost {
		return false
	}
	b.tokens -= cost
	return true
}

// ── Ingest endpoint ────────────────────────────────────────────────────────

// IngestEvent is one timing observation pushed by a client.
type IngestEvent struct {
	Name       string `json:"name"`
	DurationMs int32  `json:"duration_ms"`
	Source     string `json:"source,omitempty"`
	// Tags is a flat map of free-form labels. Serialized into context_json
	// alongside per-batch overrides so the dashboard can filter on them.
	Tags map[string]string `json:"tags,omitempty"`
}

// IngestBatch is the request body for POST /api/ingest/timings. Component,
// environment, and host are defaults for every event in the batch; each is
// optional and component falls back to the token's default.
type IngestBatch struct {
	Component   string        `json:"component,omitempty"`
	Environment string        `json:"environment,omitempty"`
	Host        string        `json:"host,omitempty"`
	Events      []IngestEvent `json:"events"`
}

// registerIngestRoutes wires POST /api/ingest/timings. Auth is bearer-only
// (NOT the admin X-Api-Key); apiKeyMiddleware bypasses this path.
func (s *Server) registerIngestRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ingest-timings",
		Method:      http.MethodPost,
		Path:        "/api/ingest/timings",
		Summary:     "Ingest a batch of timing events",
	},
		func(ctx context.Context, in *struct {
			Authorization string `header:"Authorization"`
			Body          IngestBatch
		}) (*struct {
			Body struct {
				Accepted int `json:"accepted"`
			}
		}, error) {
			// Skip timing on the ingest path itself — otherwise every POST from
			// the zdx self-client becomes another event that triggers more POSTs.
			ctx = WithoutTiming(ctx)
			token := strings.TrimPrefix(in.Authorization, "Bearer ")
			if token == in.Authorization || token == "" {
				return nil, handlers.APIErr(http.StatusUnauthorized, "missing bearer token")
			}
			row, err := s.q.GetIntegrationTokenByHash(ctx, handlers.HashIntegrationToken(token))
			if err != nil {
				return nil, handlers.APIErr(http.StatusUnauthorized, "invalid token")
			}
			if row.RevokedAt.Valid {
				return nil, handlers.APIErr(http.StatusUnauthorized, "token revoked")
			}
			if len(in.Body.Events) == 0 {
				return &struct {
					Body struct {
						Accepted int `json:"accepted"`
					}
				}{}, nil
			}
			if !s.ingestLimiter.allow(row.ID, float64(len(in.Body.Events))) {
				return nil, handlers.APIErr(http.StatusTooManyRequests, "rate limit exceeded")
			}

			// Resolve defaults: per-batch override wins over token default.
			component := in.Body.Component
			if component == "" && row.Component.Valid {
				component = row.Component.String
			}
			env := in.Body.Environment
			host := in.Body.Host

			accepted := 0
			for _, ev := range in.Body.Events {
				ctxJSON := buildContextJSON(host, ev.Tags)
				pid := pgtype.Int4{Int32: row.ProjectID, Valid: true}
				if err := s.q.UpsertTimed(ctx, db.UpsertTimedParams{
					ProjectID:   pid,
					Component:   component,
					Environment: env,
					Name:        ev.Name,
					DurationMs:  ev.DurationMs,
					Source:      ev.Source,
					ContextJson: ctxJSON,
					TotalMs:     int64(ev.DurationMs),
				}); err != nil {
					log.Printf("ingest: UpsertTimed %q: %v", ev.Name, err)
					continue
				}
				if err := s.q.InsertTimedEvent(ctx, db.InsertTimedEventParams{
					ProjectID:   pid,
					Component:   component,
					Environment: env,
					Name:        ev.Name,
					DurationMs:  ev.DurationMs,
					Source:      ev.Source,
					ContextJson: ctxJSON,
				}); err != nil {
					log.Printf("ingest: InsertTimedEvent %q: %v", ev.Name, err)
				}
				accepted++
			}
			return &struct {
				Body struct {
					Accepted int `json:"accepted"`
				}
			}{Body: struct {
				Accepted int `json:"accepted"`
			}{Accepted: accepted}}, nil
		})
}

// ── Counter ingest endpoint ────────────────────────────────────────────────

type IngestCounterEvent struct {
	Name   string            `json:"name"`
	Value  int32             `json:"value"`
	Source string            `json:"source,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

type IngestCounterBatch struct {
	Component   string               `json:"component,omitempty"`
	Environment string               `json:"environment,omitempty"`
	Host        string               `json:"host,omitempty"`
	Events      []IngestCounterEvent `json:"events"`
}

func (s *Server) registerCounterIngestRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ingest-counters",
		Method:      http.MethodPost,
		Path:        "/api/ingest/counters",
		Summary:     "Ingest a batch of counter events",
	},
		func(ctx context.Context, in *struct {
			Authorization string `header:"Authorization"`
			Body          IngestCounterBatch
		}) (*struct {
			Body struct {
				Accepted int `json:"accepted"`
			}
		}, error) {
			ctx = WithoutTiming(ctx)
			token := strings.TrimPrefix(in.Authorization, "Bearer ")
			if token == in.Authorization || token == "" {
				return nil, handlers.APIErr(http.StatusUnauthorized, "missing bearer token")
			}
			row, err := s.q.GetIntegrationTokenByHash(ctx, handlers.HashIntegrationToken(token))
			if err != nil {
				return nil, handlers.APIErr(http.StatusUnauthorized, "invalid token")
			}
			if row.RevokedAt.Valid {
				return nil, handlers.APIErr(http.StatusUnauthorized, "token revoked")
			}
			if len(in.Body.Events) == 0 {
				return &struct {
					Body struct {
						Accepted int `json:"accepted"`
					}
				}{}, nil
			}
			if !s.ingestLimiter.allow(row.ID, float64(len(in.Body.Events))) {
				return nil, handlers.APIErr(http.StatusTooManyRequests, "rate limit exceeded")
			}

			component := in.Body.Component
			if component == "" && row.Component.Valid {
				component = row.Component.String
			}
			env := in.Body.Environment
			host := in.Body.Host

			accepted := 0
			for _, ev := range in.Body.Events {
				ctxJSON := buildContextJSON(host, ev.Tags)
				pid := pgtype.Int4{Int32: row.ProjectID, Valid: true}
				if err := s.q.UpsertCounted(ctx, db.UpsertCountedParams{
					ProjectID:   pid,
					Component:   component,
					Environment: env,
					Name:        ev.Name,
					Value:       ev.Value,
					Source:      ev.Source,
					ContextJson: ctxJSON,
					TotalValue:  int64(ev.Value),
				}); err != nil {
					log.Printf("ingest: UpsertCounted %q: %v", ev.Name, err)
					continue
				}
				if err := s.q.InsertCounterEvent(ctx, db.InsertCounterEventParams{
					ProjectID:   pid,
					Component:   component,
					Environment: env,
					Name:        ev.Name,
					Value:       ev.Value,
					Source:      ev.Source,
					ContextJson: ctxJSON,
				}); err != nil {
					log.Printf("ingest: InsertCounterEvent %q: %v", ev.Name, err)
				}
				accepted++
			}
			return &struct {
				Body struct {
					Accepted int `json:"accepted"`
				}
			}{Body: struct {
				Accepted int `json:"accepted"`
			}{Accepted: accepted}}, nil
		})
}

// ── Error ingest endpoint ─────────────────────────────────────────────────

type IngestErrorEvent struct {
	Name       string            `json:"name"`
	Message    string            `json:"message,omitempty"`
	StackTrace string            `json:"stack_trace,omitempty"`
	Source     string            `json:"source,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type IngestErrorBatch struct {
	Component   string             `json:"component,omitempty"`
	Environment string             `json:"environment,omitempty"`
	Host        string             `json:"host,omitempty"`
	Events      []IngestErrorEvent `json:"events"`
}

func (s *Server) registerErrorIngestRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ingest-errors",
		Method:      http.MethodPost,
		Path:        "/api/ingest/errors",
		Summary:     "Ingest a batch of error events",
	},
		func(ctx context.Context, in *struct {
			Authorization string `header:"Authorization"`
			Body          IngestErrorBatch
		}) (*struct {
			Body struct {
				Accepted int `json:"accepted"`
			}
		}, error) {
			ctx = WithoutTiming(ctx)
			token := strings.TrimPrefix(in.Authorization, "Bearer ")
			if token == in.Authorization || token == "" {
				return nil, handlers.APIErr(http.StatusUnauthorized, "missing bearer token")
			}
			row, err := s.q.GetIntegrationTokenByHash(ctx, handlers.HashIntegrationToken(token))
			if err != nil {
				return nil, handlers.APIErr(http.StatusUnauthorized, "invalid token")
			}
			if row.RevokedAt.Valid {
				return nil, handlers.APIErr(http.StatusUnauthorized, "token revoked")
			}
			if len(in.Body.Events) == 0 {
				return &struct {
					Body struct {
						Accepted int `json:"accepted"`
					}
				}{}, nil
			}
			if !s.ingestLimiter.allow(row.ID, float64(len(in.Body.Events))) {
				return nil, handlers.APIErr(http.StatusTooManyRequests, "rate limit exceeded")
			}

			component := in.Body.Component
			if component == "" && row.Component.Valid {
				component = row.Component.String
			}
			env := in.Body.Environment
			host := in.Body.Host

			accepted := 0
			for _, ev := range in.Body.Events {
				ctxJSON := buildContextJSON(host, ev.Tags)
				pid := pgtype.Int4{Int32: row.ProjectID, Valid: true}
				if err := s.q.InsertErrorEvent(ctx, db.InsertErrorEventParams{
					ProjectID:   pid,
					Component:   component,
					Environment: env,
					Name:        ev.Name,
					Message:     ev.Message,
					StackTrace:  ev.StackTrace,
					Source:      ev.Source,
					ContextJson: ctxJSON,
				}); err != nil {
					log.Printf("ingest: InsertErrorEvent %q: %v", ev.Name, err)
					continue
				}
				accepted++
			}
			return &struct {
				Body struct {
					Accepted int `json:"accepted"`
				}
			}{Body: struct {
				Accepted int `json:"accepted"`
			}{Accepted: accepted}}, nil
		})
}

// ── Log ingest endpoint ──────────────────────────────────────────────────

type IngestLogEvent struct {
	Level   string            `json:"level,omitempty"`
	Message string            `json:"message"`
	Source  string            `json:"source,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
}

type IngestLogBatch struct {
	Component   string           `json:"component,omitempty"`
	Environment string           `json:"environment,omitempty"`
	Host        string           `json:"host,omitempty"`
	Events      []IngestLogEvent `json:"events"`
}

func (s *Server) registerLogIngestRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ingest-logs",
		Method:      http.MethodPost,
		Path:        "/api/ingest/logs",
		Summary:     "Ingest a batch of log events",
	},
		func(ctx context.Context, in *struct {
			Authorization string `header:"Authorization"`
			Body          IngestLogBatch
		}) (*struct {
			Body struct {
				Accepted int `json:"accepted"`
			}
		}, error) {
			ctx = WithoutTiming(ctx)
			token := strings.TrimPrefix(in.Authorization, "Bearer ")
			if token == in.Authorization || token == "" {
				return nil, handlers.APIErr(http.StatusUnauthorized, "missing bearer token")
			}
			row, err := s.q.GetIntegrationTokenByHash(ctx, handlers.HashIntegrationToken(token))
			if err != nil {
				return nil, handlers.APIErr(http.StatusUnauthorized, "invalid token")
			}
			if row.RevokedAt.Valid {
				return nil, handlers.APIErr(http.StatusUnauthorized, "token revoked")
			}
			if len(in.Body.Events) == 0 {
				return &struct {
					Body struct {
						Accepted int `json:"accepted"`
					}
				}{}, nil
			}
			if !s.ingestLimiter.allow(row.ID, float64(len(in.Body.Events))) {
				return nil, handlers.APIErr(http.StatusTooManyRequests, "rate limit exceeded")
			}

			component := in.Body.Component
			if component == "" && row.Component.Valid {
				component = row.Component.String
			}
			env := in.Body.Environment
			host := in.Body.Host

			accepted := 0
			for _, ev := range in.Body.Events {
				level := ev.Level
				if level == "" {
					level = "info"
				}
				ctxJSON := buildContextJSON(host, ev.Tags)
				pid := pgtype.Int4{Int32: row.ProjectID, Valid: true}
				if err := s.q.InsertLogEvent(ctx, db.InsertLogEventParams{
					ProjectID:   pid,
					Component:   component,
					Environment: env,
					Level:       level,
					Message:     ev.Message,
					Source:      ev.Source,
					ContextJson: ctxJSON,
				}); err != nil {
					log.Printf("ingest: InsertLogEvent: %v", err)
					continue
				}
				accepted++
			}
			return &struct {
				Body struct {
					Accepted int `json:"accepted"`
				}
			}{Body: struct {
				Accepted int `json:"accepted"`
			}{Accepted: accepted}}, nil
		})
}

// ── Prometheus remote_write ingest ───────────────────────────────────────

func (s *Server) registerPromIngestRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ingest-prom",
		Method:      http.MethodPost,
		Path:        "/api/ingest/prom",
		Summary:     "Ingest Prometheus remote_write v1",
	},
		func(ctx context.Context, in *struct {
			Authorization string `header:"Authorization"`
			ContentType   string `header:"Content-Type"`
			RawBody       []byte
		}) (*struct{}, error) {
			ctx = WithoutTiming(ctx)
			token := strings.TrimPrefix(in.Authorization, "Bearer ")
			if token == in.Authorization || token == "" {
				return nil, handlers.APIErr(http.StatusUnauthorized, "missing bearer token")
			}
			row, err := s.q.GetIntegrationTokenByHash(ctx, handlers.HashIntegrationToken(token))
			if err != nil {
				return nil, handlers.APIErr(http.StatusUnauthorized, "invalid token")
			}
			if row.RevokedAt.Valid {
				return nil, handlers.APIErr(http.StatusUnauthorized, "token revoked")
			}

			decoded, err := snappy.Decode(nil, in.RawBody)
			if err != nil {
				return nil, handlers.APIErr(http.StatusBadRequest, "snappy decode: "+err.Error())
			}

			var req prompb.WriteRequest
			if err := req.Unmarshal(decoded); err != nil {
				return nil, handlers.APIErr(http.StatusBadRequest, "protobuf decode: "+err.Error())
			}

			var sampleCount int
			for _, ts := range req.Timeseries {
				sampleCount += len(ts.Samples)
			}
			if sampleCount == 0 {
				return &struct{}{}, nil
			}
			if !s.ingestLimiter.allow(row.ID, float64(sampleCount)) {
				return nil, handlers.APIErr(http.StatusTooManyRequests, "rate limit exceeded")
			}

			component := ""
			if row.Component.Valid {
				component = row.Component.String
			}
			pid := pgtype.Int4{Int32: row.ProjectID, Valid: true}

			for _, ts := range req.Timeseries {
				name := ""
				tags := make(map[string]string)
				for _, l := range ts.Labels {
					if l.Name == "__name__" {
						name = l.Value
					} else {
						tags[l.Name] = l.Value
					}
				}
				if name == "" {
					continue
				}
				ctxJSON := buildContextJSON("", tags)

				for _, sample := range ts.Samples {
					durationMs := int32(math.Round(sample.Value))
					sampleTime := time.UnixMilli(sample.Timestamp)

					if err := s.q.UpsertTimed(ctx, db.UpsertTimedParams{
						ProjectID:   pid,
						Component:   component,
						Environment: "",
						Name:        name,
						DurationMs:  durationMs,
						Source:      "prometheus",
						ContextJson: ctxJSON,
						TotalMs:     int64(durationMs),
					}); err != nil {
						log.Printf("ingest/prom: UpsertTimed %q: %v", name, err)
						continue
					}
					if err := s.q.InsertTimedEventAt(ctx, db.InsertTimedEventAtParams{
						ProjectID:   pid,
						Component:   component,
						Environment: "",
						Name:        name,
						DurationMs:  durationMs,
						Source:      "prometheus",
						ContextJson: ctxJSON,
						CreatedAt:   pgtype.Timestamptz{Time: sampleTime, Valid: true},
					}); err != nil {
						log.Printf("ingest/prom: InsertTimedEventAt %q: %v", name, err)
					}
				}
			}
			return &struct{}{}, nil
		})
}

// buildContextJSON folds host + free-form tags into the context_json column.
func buildContextJSON(host string, tags map[string]string) []byte {
	m := make(map[string]string, len(tags)+1)
	if host != "" {
		m["host"] = host
	}
	for k, v := range tags {
		m[k] = v
	}
	if len(m) == 0 {
		return []byte("{}")
	}
	b, _ := json.Marshal(m)
	return b
}

// ── Self-token bootstrap ──────────────────────────────────────────────────

// BootstrapSelfIntegrationToken returns a valid integration token for zdx's
// own project, creating one if none exists. Lookup order:
//
//  1. ZDX_SELF_TOKEN env (dev/testing override).
//  2. $ZDX_HOME/data/self-integration-token on disk.
//  3. Create a new token + persist to disk.
//
// Returns ("", nil) when s.zdxProjectSlug is unset or the project is missing —
// the caller should skip self-wiring in that case. Any write/DB error is
// propagated since the client can't be initialized without a token.
func (s *Server) BootstrapSelfIntegrationToken(ctx context.Context) (string, error) {
	slug := s.zdxProjectSlug
	if slug == "" {
		// Auto-detect: if exactly one project exists, use it.
		projects, err := s.q.ListProjects(ctx)
		if err != nil || len(projects) != 1 {
			return "", nil
		}
		slug = projects[0].Slug
		s.zdxProjectSlug = slug
		log.Printf("self-integration: auto-detected project %q", slug)
	}
	ctx = WithoutTiming(ctx)

	validate := func(tok string) bool {
		if tok == "" {
			return false
		}
		row, err := s.q.GetIntegrationTokenByHash(ctx, handlers.HashIntegrationToken(tok))
		return err == nil && !row.RevokedAt.Valid
	}

	if tok := os.Getenv("ZDX_SELF_TOKEN"); validate(tok) {
		return tok, nil
	}
	path := selfIntegrationTokenPath()
	if b, err := os.ReadFile(path); err == nil {
		tok := strings.TrimSpace(string(b))
		if validate(tok) {
			return tok, nil
		}
	}

	p, err := s.q.GetProjectBySlug(ctx, slug)
	if err != nil {
		return "", nil // project not yet provisioned — skip, not fatal
	}
	tok, err := handlers.GenerateIntegrationToken()
	if err != nil {
		return "", fmt.Errorf("generate self token: %w", err)
	}
	if _, err := s.q.CreateIntegrationToken(ctx, db.CreateIntegrationTokenParams{
		ProjectID:   p.ID,
		Component:   pgtype.Text{String: "zdx-server", Valid: true},
		Name:        "self-integration",
		TokenHash:   handlers.HashIntegrationToken(tok),
		TokenPrefix: handlers.TokenPrefix(tok),
	}); err != nil {
		return "", fmt.Errorf("create self token: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(tok), 0o600)
	}
	return tok, nil
}

// selfIntegrationTokenPath is where we persist the self-token between restarts
// so a new token isn't issued on every process start.
func selfIntegrationTokenPath() string {
	if zdxHome := os.Getenv("ZDX_HOME"); zdxHome != "" {
		return zdxHome + "/data/self-integration-token"
	}
	return "data/self-integration-token"
}
