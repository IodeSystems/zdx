package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// auditNote records a server-side mutation event in zdx_log_events,
// attributing it to the agent + session + user it inherits from ctx (the
// cli.Client stamps X-ZDX-Agent-Id / X-ZDX-Session-Id headers, the
// apiKeyMiddleware resolves user_id, and ctx-readers wrap them via
// ctxAgentIDVal / ctxSessionIDVal / ctxUserIDVal).
//
// `dx log tail --tag alias=<agent_id>` will surface these events because
// the `alias` key in context_json is matched via jsonb @> containment in
// ListLogEvents (queries/log_events.sql:10).
//
// Call once per mutation, after the DB write that produced the row succeeded:
//
//	auditNote(ctx, h.Q, p.ID, "task.reviewed",
//	    "task_id", id, "verdict", verdict, "review_id", rev.ID)
//
// Variadic kv pairs match the existing tracelog.Logger convention. Odd-
// length kv slices drop the dangling key with a stderr warning rather
// than panicking — best-effort: a logging-side bug must not fail the
// mutation handler that called it.
//
// This is the audit-tag-permeation primitive (plan/spike-audit-tag-
// permeation.md). Phase 1 of the rollout: helper exists. Phase 2:
// review handler uses it. Phase 3: every other mutation handler.
func auditNote(ctx context.Context, q *db.Queries, projectID int32, eventType string, kv ...any) {
	if q == nil {
		return // tests sometimes wire a nil Queries; don't panic.
	}
	tags := buildAuditTags(ctx, kv)
	payload, err := json.Marshal(tags)
	if err != nil {
		log.Printf("audit.Note: marshal context for %q: %v", eventType, err)
		return
	}
	if err := q.InsertLogEvent(ctx, db.InsertLogEventParams{
		ProjectID:   pgtype.Int4{Int32: projectID, Valid: true},
		Component:   "server",
		Environment: "audit",
		Level:       "info",
		Message:     eventType,
		Source:      "audit",
		ContextJson: payload,
	}); err != nil {
		log.Printf("audit.Note: insert %q: %v", eventType, err)
	}
}

// buildAuditTags packs ctx-derived attribution + caller-supplied kv into
// the context_json shape `dx log tail --tag` filters by. Factored out so
// the tag contract is unit-testable without a DB mock; auditNote layers
// the InsertLogEvent on top.
//
// Contract:
//   - kv is a flat (key, value, key, value, ...) slice — odd-length
//     trims the dangling key with a stderr warning.
//   - non-string keys are skipped with a stderr warning.
//   - ctx attribution (agent_id, session_id, user_id) is added unconditionally
//     when the corresponding ctx value is non-zero, AFTER kv pairs — so caller
//     kv cannot accidentally overwrite the attribution keys.
//   - `alias` mirrors `agent_id` so existing `dx log tail --tag alias=X`
//     filters work without changing tooling.
func buildAuditTags(ctx context.Context, kv []any) map[string]any {
	tags := make(map[string]any, len(kv)/2+3)

	if len(kv)%2 != 0 {
		log.Printf("audit: odd-length kv slice (dropping last key %v)", kv[len(kv)-1])
		kv = kv[:len(kv)-1]
	}
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			log.Printf("audit: non-string key at index %d (skipping)", i)
			continue
		}
		tags[k] = kv[i+1]
	}

	if v := ctxAgentIDVal(ctx); v != "" {
		// alias is the canonical tag-key used by `dx log tail --tag alias=X`;
		// agent_id is the same value under a more explicit name for callers
		// that want to disambiguate from cluster_id, slot index, etc.
		tags["alias"] = v
		tags["agent_id"] = v
	}
	if v := ctxSessionIDVal(ctx); v != "" {
		tags["session_id"] = v
	}
	if v := ctxUserIDVal(ctx); v != 0 {
		tags["user_id"] = v
	}
	return tags
}
