package handlers

import (
	"context"
	"encoding/json"
	"log"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// traceEvent records a structured server-side event in zdx_log_events,
// stamped with the trace correlation tags (trace_id, agent_id,
// session_id, user_id) the apiKeyMiddleware extracted from request
// headers. Industry term: a structured log event tagged for distributed
// trace correlation; the agent's loop tracelog already uses the same
// trace_id key, so a single `dx log tail --tag trace_id=<X>` filter
// surfaces the WHOLE flow — the agent's loop iterations AND every
// server-side mutation it caused.
//
// `dx log tail --tag alias=<agent_id>` and `--tag trace_id=<X>` both
// match via jsonb @> on context_json (queries/log_events.sql:10).
//
// Call once per mutation, after the DB write that produced the row succeeded:
//
//	traceEvent(ctx, h.Q, p.ID, "task.reviewed",
//	    "task_id", id, "verdict", verdict, "review_id", rev.ID)
//
// Variadic kv pairs match the existing tracelog.Logger convention. Odd-
// length kv slices drop the dangling key with a log warning rather than
// panicking — best-effort: a logging-side bug must not fail the
// mutation handler that called it.
//
// Trace-tag-permeation primitive (plan/spike-audit-tag-permeation.md).
// Phase 1: helper exists (here). Phase 2: review handler uses it.
// Phase 3: every other mutation handler.
func traceEvent(ctx context.Context, q *db.Queries, projectID int32, eventType string, kv ...any) {
	if q == nil {
		return // tests sometimes wire a nil Queries; don't panic.
	}
	tags := buildTraceTags(ctx, kv)
	payload, err := json.Marshal(tags)
	if err != nil {
		log.Printf("trace: marshal context for %q: %v", eventType, err)
		return
	}
	if err := q.InsertLogEvent(ctx, db.InsertLogEventParams{
		ProjectID:   pgtype.Int4{Int32: projectID, Valid: true},
		Component:   "server",
		Environment: "trace",
		Level:       "info",
		Message:     eventType,
		Source:      "trace",
		ContextJson: payload,
	}); err != nil {
		log.Printf("trace: insert %q: %v", eventType, err)
	}
}

// buildTraceTags packs ctx-derived correlation tags + caller-supplied kv
// into the context_json shape `dx log tail --tag` filters by. Factored
// out so the tag contract is unit-testable without a DB mock; traceEvent
// layers the InsertLogEvent on top.
//
// Contract:
//   - kv is a flat (key, value, key, value, ...) slice — odd-length
//     trims the dangling key with a log warning.
//   - non-string keys are skipped with a log warning.
//   - ctx correlation (trace_id, agent_id, session_id, user_id) is added
//     AFTER kv pairs, so caller kv cannot accidentally overwrite the
//     reserved correlation keys.
//   - `alias` mirrors `agent_id` so existing `dx log tail --tag alias=X`
//     filters keep working without tooling changes.
func buildTraceTags(ctx context.Context, kv []any) map[string]any {
	tags := make(map[string]any, len(kv)/2+4)

	if len(kv)%2 != 0 {
		log.Printf("trace: odd-length kv slice (dropping last key %v)", kv[len(kv)-1])
		kv = kv[:len(kv)-1]
	}
	for i := 0; i < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			log.Printf("trace: non-string key at index %d (skipping)", i)
			continue
		}
		tags[k] = kv[i+1]
	}

	// trace_id is the cross-process correlation key (matches the agent
	// loop's tracelog convention). When present, `dx log tail --tag
	// trace_id=<X>` will return both client-side loop events AND
	// server-side mutations the agent caused as one chain.
	if v := ctxTraceIDVal(ctx); v != "" {
		tags["trace_id"] = v
	}
	if v := ctxAgentIDVal(ctx); v != "" {
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
