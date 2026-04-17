package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// Unified, provider-agnostic agent session lifecycle.
// Endpoints:
//   POST /api/dx/agent/sessions/{sid}/create  — idempotent by (project, sid)
//   POST /api/dx/agent/sessions/{sid}/events  — application/x-ndjson, {seq, event_type, event_json, agent_id?}
//   POST /api/dx/agent/sessions/{sid}/close   — marks closed and enqueues summarize
//
// The legacy /api/dx/claude/sessions/ingest/stream endpoint is aliased to the
// events ingestion path via shared helpers (ingestAgentEvent,
// getOrCreateAgentSession). The batch /ingest endpoint is left intact; a later
// task removes it.

func (h *Handler) registerAgentSessionRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "create-agent-session",
		Method:      http.MethodPost,
		Path:        "/api/dx/agent/sessions/{sid}/create",
	}, func(ctx context.Context, in *struct {
		Sid  string `path:"sid" required:"true"`
		Slug string `query:"slug" required:"true"`
		Body struct {
			Provider         string `json:"provider"`
			IssueID          string `json:"issue_id"`
			Alias            string `json:"alias"`
			Trigger          string `json:"trigger"`
			Title            string `json:"title" required:"false"`
			AgentID          string `json:"agent_id" required:"false"`
			AgentType        string `json:"agent_type" required:"false"`
			AgentDescription string `json:"agent_description" required:"false"`
			TodoID           int32  `json:"todo_id" required:"false"`
		}
	}) (*struct {
		Body struct {
			ID        int64  `json:"id"`
			SessionID string `json:"session_id"`
			Created   bool   `json:"created"`
		}
	}, error) {
		p, err := getProject(ctx, h.Q, in.Slug)
		if err != nil {
			return nil, err
		}
		sess, created, err := h.getOrCreateAgentSession(ctx, p.ID, in.Sid, in.Body.IssueID, in.Body.Alias, in.Body.Title, in.Body.TodoID)
		if err != nil {
			return nil, apiErr(500, err.Error())
		}
		if created {
			h.Broker.PublishAgentSessionLifecycle(in.Slug, sess.SessionID, "agent.session-created", map[string]any{
				"session_id": sess.SessionID,
				"session_pk": sess.ID,
				"provider":   in.Body.Provider,
				"issue_id":   sess.IssueID,
				"alias":      sess.Alias,
				"trigger":    in.Body.Trigger,
				"agent_id":   in.Body.AgentID,
			})
		}
		return &struct {
			Body struct {
				ID        int64  `json:"id"`
				SessionID string `json:"session_id"`
				Created   bool   `json:"created"`
			}
		}{Body: struct {
			ID        int64  `json:"id"`
			SessionID string `json:"session_id"`
			Created   bool   `json:"created"`
		}{ID: sess.ID, SessionID: sess.SessionID, Created: created}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "close-agent-session",
		Method:      http.MethodPost,
		Path:        "/api/dx/agent/sessions/{sid}/close",
	}, func(ctx context.Context, in *struct {
		Sid  string `path:"sid" required:"true"`
		Slug string `query:"slug" required:"true"`
		Body struct {
			ExitCode   int32 `json:"exit_code"`
			DurationMs int64 `json:"duration_ms"`
			EventCount int32 `json:"event_count"`
			Tokens     struct {
				Input      int64 `json:"input" required:"false"`
				Output     int64 `json:"output" required:"false"`
				CacheRead  int64 `json:"cache_read" required:"false"`
				CacheWrite int64 `json:"cache_write" required:"false"`
			} `json:"tokens"`
		}
	}) (*struct {
		Body struct {
			SessionID string `json:"session_id"`
			Status    string `json:"status"`
		}
	}, error) {
		p, err := getProject(ctx, h.Q, in.Slug)
		if err != nil {
			return nil, err
		}
		sess, err := h.Q.GetClaudeSessionBySessionID(ctx, db.GetClaudeSessionBySessionIDParams{
			ProjectID: p.ID,
			SessionID: in.Sid,
		})
		if err != nil {
			return nil, apiErr(404, "session not found")
		}

		_ = h.Q.CloseClaudeSession(ctx, sess.ID)

		h.Broker.PublishAgentSessionLifecycle(in.Slug, sess.SessionID, "agent.session-closed", map[string]any{
			"session_id":  sess.SessionID,
			"session_pk":  sess.ID,
			"exit_code":   in.Body.ExitCode,
			"duration_ms": in.Body.DurationMs,
			"event_count": in.Body.EventCount,
			"tokens": map[string]int64{
				"input":       in.Body.Tokens.Input,
				"output":      in.Body.Tokens.Output,
				"cache_read":  in.Body.Tokens.CacheRead,
				"cache_write": in.Body.Tokens.CacheWrite,
			},
		})
		h.Broker.PublishClaudeSessionLifecycle(in.Slug, sess.SessionID, "claude.session-closed", map[string]any{
			"session_id":  sess.SessionID,
			"session_pk":  sess.ID,
			"event_count": in.Body.EventCount,
		})

		go h.summarizeSessionFromDBAsync(p.ID, sess.ID)

		return &struct {
			Body struct {
				SessionID string `json:"session_id"`
				Status    string `json:"status"`
			}
		}{Body: struct {
			SessionID string `json:"session_id"`
			Status    string `json:"status"`
		}{SessionID: sess.SessionID, Status: "closed"}}, nil
	})

	// Events ingestion: raw ndjson body, wrapped-event format per line.
	h.Mux.Post("/api/dx/agent/sessions/{sid}/events", h.handleAgentSessionEvents)
}

// ── Shared helpers ─────────────────────────────────────────────────────────

// getOrCreateAgentSession looks up a session by (project_id, session_id) and
// creates one if it doesn't exist. The second return is true when this call
// created the row.
func (h *Handler) getOrCreateAgentSession(ctx context.Context, projectID int32, sessionID, issueID, alias, title string, todoID int32) (db.CreateClaudeSessionRow, bool, error) {
	todo := pgtype.Int4{}
	if todoID > 0 {
		todo = pgtype.Int4{Int32: todoID, Valid: true}
	}
	sess, err := h.Q.CreateClaudeSession(ctx, db.CreateClaudeSessionParams{
		ProjectID: projectID,
		IssueID:   issueID,
		SessionID: sessionID,
		Title:     title,
		Alias:     alias,
		TodoID:    todo,
	})
	if err == nil {
		return sess, true, nil
	}
	if !strings.Contains(err.Error(), "duplicate key") {
		return db.CreateClaudeSessionRow{}, false, err
	}
	existing, lookupErr := h.Q.GetClaudeSessionBySessionID(ctx, db.GetClaudeSessionBySessionIDParams{
		ProjectID: projectID,
		SessionID: sessionID,
	})
	if lookupErr != nil {
		return db.CreateClaudeSessionRow{}, false, lookupErr
	}
	return db.CreateClaudeSessionRow{
		ID:        existing.ID,
		ProjectID: existing.ProjectID,
		IssueID:   existing.IssueID,
		SessionID: existing.SessionID,
		Title:     existing.Title,
		Alias:     existing.Alias,
		Header:    existing.Header,
		Summary:   existing.Summary,
		Status:    existing.Status,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: existing.UpdatedAt,
		ClosedAt:  existing.ClosedAt,
		TodoID:    existing.TodoID,
	}, false, nil
}

// ingestAgentEvent persists one event and publishes it on the WS channel
// under both agent.event (new) and claude.event (legacy, for UI transition).
func (h *Handler) ingestAgentEvent(
	ctx context.Context,
	slug, sessionID string,
	sessionPK int64,
	seq int32,
	eventType string,
	eventJSON []byte,
	agentID, agentType, agentDesc string,
) {
	isSidechain := agentID != ""
	_ = h.Q.CreateClaudeEvent(ctx, db.CreateClaudeEventParams{
		SessionPk:        sessionPK,
		Seq:              seq,
		EventType:        eventType,
		EventJson:        eventJSON,
		AgentID:          agentID,
		IsSidechain:      isSidechain,
		AgentType:        agentType,
		AgentDescription: agentDesc,
	})
	_ = h.Q.TouchClaudeSession(ctx, sessionPK)

	var evPayload any
	var parsed map[string]any
	if json.Unmarshal(eventJSON, &parsed) == nil {
		evPayload = parsed
	} else {
		evPayload = json.RawMessage(eventJSON)
	}

	payload := map[string]any{
		"session_id":        sessionID,
		"session_pk":        sessionPK,
		"seq":               seq,
		"event_type":        eventType,
		"event_json":        evPayload,
		"agent_id":          agentID,
		"agent_type":        agentType,
		"agent_description": agentDesc,
	}
	h.Broker.PublishClaudeEvent(slug, sessionID, "agent.event", payload)
	// Emit legacy event name so the current UI keeps working during transition.
	h.Broker.PublishClaudeEvent(slug, sessionID, "claude.event", payload)
}

// ── /events handler ───────────────────────────────────────────────────────

func (h *Handler) handleAgentSessionEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	sid := chi.URLParam(r, "sid")
	slug := r.URL.Query().Get("slug")
	if slug == "" || sid == "" {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"slug and sid required"}`, http.StatusBadRequest)
		return
	}

	p, err := h.Q.GetProjectBySlug(ctx, slug)
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"project not found"}`, http.StatusNotFound)
		return
	}

	sess, err := h.Q.GetClaudeSessionBySessionID(ctx, db.GetClaudeSessionBySessionIDParams{
		ProjectID: p.ID,
		SessionID: sid,
	})
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"session not found; call /create first"}`, http.StatusNotFound)
		return
	}

	maxSeq, err := h.Q.GetMaxClaudeEventSeq(ctx, sess.ID)
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	persisted := 0
	skipped := 0

	scanner := bufio.NewScanner(r.Body)
	scanner.Buffer(make([]byte, 1<<20), 4<<20)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		var ev struct {
			Seq              int32           `json:"seq"`
			EventType        string          `json:"event_type"`
			EventJSON        json.RawMessage `json:"event_json"`
			AgentID          string          `json:"agent_id"`
			AgentType        string          `json:"agent_type"`
			AgentDescription string          `json:"agent_description"`
		}
		if err := json.Unmarshal(line, &ev); err != nil {
			skipped++
			continue
		}
		if ev.Seq <= maxSeq {
			skipped++
			continue
		}
		eventBytes := []byte(ev.EventJSON)
		if len(eventBytes) == 0 {
			eventBytes = []byte("{}")
		}
		h.ingestAgentEvent(ctx, slug, sess.SessionID, sess.ID, ev.Seq, ev.EventType, eventBytes, ev.AgentID, ev.AgentType, ev.AgentDescription)
		if ev.Seq > maxSeq {
			maxSeq = ev.Seq
		}
		persisted++
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": sess.SessionID,
		"session_pk": sess.ID,
		"persisted":  persisted,
		"skipped":    skipped,
		"max_seq":    maxSeq,
	})
}

// ── Summarize from DB ─────────────────────────────────────────────────────

// summarizeSessionFromDBAsync loads all events for the session from the DB,
// reconstructs the transcript, and runs the shared summarize pipeline. Used by
// the /close endpoint where the request body does not carry the events.
func (h *Handler) summarizeSessionFromDBAsync(projectID int32, sessionPK int64) {
	ctx := WithSource(context.Background(), "auto-summarize")
	events, err := h.Q.ListClaudeEvents(ctx, sessionPK)
	if err != nil {
		return
	}
	lines := make([]string, 0, len(events))
	for _, ev := range events {
		lines = append(lines, string(ev.EventJson))
	}
	h.summarizeSessionAsync(projectID, sessionPK, lines)
}
