package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// ── Token generation ──────────────────────────────────────────────────────

// integrationTokenPrefix is the human-visible lead-in so secrets are easy to
// grep for in logs and distinguish from other credential formats.
const integrationTokenPrefix = "zdxk_"

// generateIntegrationToken returns a fresh opaque bearer token. The prefix
// makes leaks easy to spot; the 32 random bytes (64 hex chars) give 256 bits
// of entropy, well beyond what's needed for an auth token.
func generateIntegrationToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return integrationTokenPrefix + hex.EncodeToString(raw[:]), nil
}

// hashIntegrationToken returns the sha256 hex digest stored in the DB.
// Tokens are never stored in plaintext; only the hash and a short prefix.
func hashIntegrationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// tokenPrefix returns the first 12 chars of the token for display/lookup.
// Long enough to be distinctive, short enough that seeing it in a dashboard
// doesn't meaningfully reduce the search space for an attacker.
func tokenPrefix(token string) string {
	if len(token) < 12 {
		return token
	}
	return token[:12]
}

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
				return nil, apiErr(http.StatusUnauthorized, "missing bearer token")
			}
			row, err := s.q.GetIntegrationTokenByHash(ctx, hashIntegrationToken(token))
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "invalid token")
			}
			if row.RevokedAt.Valid {
				return nil, apiErr(http.StatusUnauthorized, "token revoked")
			}
			if len(in.Body.Events) == 0 {
				return &struct {
					Body struct {
						Accepted int `json:"accepted"`
					}
				}{}, nil
			}
			if !s.ingestLimiter.allow(row.ID, float64(len(in.Body.Events))) {
				return nil, apiErr(http.StatusTooManyRequests, "rate limit exceeded")
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
	if s.zdxProjectSlug == "" {
		return "", nil
	}
	ctx = WithoutTiming(ctx)

	validate := func(tok string) bool {
		if tok == "" {
			return false
		}
		row, err := s.q.GetIntegrationTokenByHash(ctx, hashIntegrationToken(tok))
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

	p, err := s.q.GetProjectBySlug(ctx, s.zdxProjectSlug)
	if err != nil {
		return "", nil // project not yet provisioned — skip, not fatal
	}
	tok, err := generateIntegrationToken()
	if err != nil {
		return "", fmt.Errorf("generate self token: %w", err)
	}
	if _, err := s.q.CreateIntegrationToken(ctx, db.CreateIntegrationTokenParams{
		ProjectID:   p.ID,
		Component:   pgtype.Text{String: "zdx-server", Valid: true},
		Name:        "self-integration",
		TokenHash:   hashIntegrationToken(tok),
		TokenPrefix: tokenPrefix(tok),
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
