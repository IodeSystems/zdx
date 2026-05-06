package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/db"
)

// HandleLogStream serves a long-lived SSE stream of log events filtered by
// tag. Uses chunked streaming + flushing so events appear at the client as
// they're inserted into zdx_log_events.
//
// Wire shape (text/event-stream):
//
//	id: <unix_nano_ts>
//	data: {"id":42,"level":"info","message":"...","context_json":{...},"created_at":"..."}
//
//	:heartbeat
//
//	id: ...
//	data: ...
//
// Heartbeat (`:heartbeat\n\n`) fires every 15s when no real events flow;
// keeps idle-aware proxies (HAProxy, nginx) from closing the connection.
//
// Cursor: starts at `since=<rfc3339>` query param or `Last-Event-ID` header
// (which is the last event's RFC3339Nano timestamp from the previous
// connection). Polls the DB at 1s intervals — internal poll, not LISTEN/
// NOTIFY, so this stays simple. For sub-second latency, swap the poll for
// pg_notify on insert later.
//
// Mounted on the chi mux directly (not via huma) because huma is a typed
// JSON-RPC framework and doesn't model streaming responses.
func (h *Handler) HandleLogStream() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.URL.Query().Get("slug")
		if slug == "" {
			http.Error(w, "slug required", http.StatusBadRequest)
			return
		}

		// Project resolution + scope check via the same helper huma routes use.
		p, err := getProject(r.Context(), h.Q, slug)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		pid := pgtype.Int4{Int32: p.ID, Valid: true}

		var tagFilter []byte
		if tf := r.URL.Query().Get("tag_filter"); tf != "" {
			tagFilter = []byte(tf)
		}

		// Cursor priority: Last-Event-ID > since > now-1s.
		cursor := time.Now().UTC().Add(-1 * time.Second)
		if v := r.Header.Get("Last-Event-ID"); v != "" {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				cursor = t
			}
		} else if v := r.URL.Query().Get("since"); v != "" {
			if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
				cursor = t
			}
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
		w.WriteHeader(http.StatusOK)

		// Initial comment so any proxy sees response bytes immediately and
		// commits the connection upgrade. Some load balancers wait for the
		// first chunk before forwarding.
		fmt.Fprint(w, ":connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		pollInterval := 1 * time.Second
		heartbeatInterval := 15 * time.Second

		pollTicker := time.NewTicker(pollInterval)
		defer pollTicker.Stop()
		heartbeatTicker := time.NewTicker(heartbeatInterval)
		defer heartbeatTicker.Stop()

		// poll runs one DB query for events strictly after cursor and
		// writes them as SSE frames. Returns the new cursor (latest event's
		// CreatedAt) and whether any events were sent (resets heartbeat).
		poll := func() (sent bool) {
			until := pgtype.Timestamptz{}
			since := pgtype.Timestamptz{Time: cursor, Valid: true}
			b := db.WrapListLogEvents(db.ListLogEventsParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until,
			}).ApplyPagination(metaquery.PageRequest{Page: 0, Size: 200, Total: false})
			res, err := mqpgx.Scan[db.ZdxLogEvent](ctx, h.Pool, b)
			if err != nil {
				return false
			}
			// Server returns DESC; reverse for chronological playback.
			rows := res.Data
			for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
				rows[i], rows[j] = rows[j], rows[i]
			}
			for _, row := range rows {
				ts := fmtTS(row.CreatedAt)
				payload := map[string]any{
					"id":           row.ID,
					"level":        row.Level,
					"message":      row.Message,
					"source":       row.Source,
					"component":    row.Component,
					"environment":  row.Environment,
					"context_json": json.RawMessage(row.ContextJson),
					"created_at":   ts,
				}
				body, _ := json.Marshal(payload)
				// id: is the RFC3339Nano timestamp so a Last-Event-ID
				// reconnect resumes from the right cursor.
				fmt.Fprintf(w, "id: %s\ndata: %s\n\n", ts, body)
				if row.CreatedAt.Valid && row.CreatedAt.Time.After(cursor) {
					cursor = row.CreatedAt.Time.Add(time.Nanosecond)
				}
				sent = true
			}
			if sent {
				flusher.Flush()
			}
			return sent
		}

		// Drain whatever exists at startup so the client sees the most
		// recent events without waiting for the first poll tick.
		_ = poll()

		// Heartbeat suppression: skip the next scheduled heartbeat when
		// the last poll sent real events (those bytes already kept the
		// connection alive). Tracks via a last-write timestamp.
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
					return // client disconnected
				}
				flusher.Flush()
				lastWrite = time.Now()
			}
		}
	}
}

// fmtTS already exists in handlers (used by list-log-events). The same
// helper ensures stream and list endpoints produce identical timestamp
// formatting so cursor strings round-trip cleanly.
var _ = strconv.Atoi // keep strconv import if formatTS or other helpers move
