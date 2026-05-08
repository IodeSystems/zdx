package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/iodesystems/zdx-go/internal/db"
)

// HandleAgentAuditStream serves a long-lived SSE stream of one agent
// session's events ordered by seq. Backfill happens via the existing
// `dx agent audit <id>` one-shot path; this endpoint is the live tail
// the CLI's --follow flag connects to once the backfill catches up.
//
// Wire shape (text/event-stream):
//
//	id: <seq>
//	data: {"seq":42,"event_type":"...","event_json":{...},"created_at":"..."}
//
// `id:` is the integer seq so a Last-Event-ID reconnect resumes from the
// right cursor without re-replaying state already on the operator's
// terminal.
//
// Cursor: `since=<seq>` query param or `Last-Event-ID` header, both as
// integers. The endpoint streams events strictly greater than the cursor.
//
// Polls zdx_claude_events at 500ms — same simple pattern as
// handlers_log_stream.go. Future: swap for pg_notify on insert.
func (h *Handler) HandleAgentAuditStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		idArg := r.URL.Query().Get("id")
		if slug == "" || idArg == "" {
			http.Error(w, "slug and id required", http.StatusBadRequest)
			return
		}

		p, err := getProject(r.Context(), h.Q, slug)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		sess, err := h.Q.ResolveClaudeSession(r.Context(), db.ResolveClaudeSessionParams{
			ProjectID: p.ID,
			SessionID: idArg,
		})
		if err != nil {
			http.Error(w, "no session matches id-or-alias: "+idArg, http.StatusNotFound)
			return
		}

		// Cursor priority: Last-Event-ID > since > -1 (replay all).
		var cursor int32 = -1
		if v := r.Header.Get("Last-Event-ID"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cursor = int32(n)
			}
		} else if v := r.URL.Query().Get("since"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cursor = int32(n)
			}
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, ":connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		pollInterval := 500 * time.Millisecond
		heartbeatInterval := 15 * time.Second
		pollTicker := time.NewTicker(pollInterval)
		defer pollTicker.Stop()
		heartbeatTicker := time.NewTicker(heartbeatInterval)
		defer heartbeatTicker.Stop()

		// poll fetches everything with seq > cursor, writes SSE frames,
		// advances cursor. Returns whether anything was sent so the
		// heartbeat can suppress redundant pings.
		poll := func() bool {
			rows, err := h.Q.ListClaudeEventsSinceSeq(ctx, db.ListClaudeEventsSinceSeqParams{
				SessionPk: sess.ID,
				Seq:       cursor,
				Limit:     500,
			})
			if err != nil || len(rows) == 0 {
				return false
			}
			for _, row := range rows {
				payload := map[string]any{
					"seq":          row.Seq,
					"event_type":   row.EventType,
					"event_json":   json.RawMessage(row.EventJson),
					"created_at":   fmtTS(row.CreatedAt),
					"agent_id":     row.AgentID,
					"agent_type":   row.AgentType,
					"is_sidechain": row.IsSidechain,
				}
				body, _ := json.Marshal(payload)
				fmt.Fprintf(w, "id: %d\ndata: %s\n\n", row.Seq, body)
				if row.Seq > cursor {
					cursor = row.Seq
				}
			}
			flusher.Flush()
			return true
		}

		_ = poll() // drain anything that's already past `since`

		lastWrite := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-pollTicker.C:
				if poll() {
					lastWrite = time.Now()
				}
			case <-heartbeatTicker.C:
				if time.Since(lastWrite) < heartbeatInterval {
					continue
				}
				if _, err := fmt.Fprint(w, ":heartbeat\n\n"); err != nil {
					return
				}
				flusher.Flush()
				lastWrite = time.Now()
			}
		}
	}
}
