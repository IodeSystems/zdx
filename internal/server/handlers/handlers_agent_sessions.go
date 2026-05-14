package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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

// SpinLockAbort describes the consecutive identical-tool_call pattern that
// caused the dispatch loop to terminate the session. Populated by the
// client-side detector (TK-1792) on close-agent-session when AbortReason is
// "spin_lock"; consumed here to emit the session.aborted_spin_lock tracelog
// downstream consumers (reviewer IS-1172, churn-hint pipeline) read.
type SpinLockAbort struct {
	Tool        string `json:"tool"`
	ArgsDigest  string `json:"args_digest"`
	RepeatCount int32  `json:"repeat_count"`
	LastTurn    int32  `json:"last_turn"`
}

// spinLockTraceArgs builds the variadic kv slice traceEvent expects when
// emitting session.aborted_spin_lock. Factored out so the kv shape is
// independently unit-testable without a DB.
func spinLockTraceArgs(sessionID, alias, issueID string, sl SpinLockAbort) []any {
	return []any{
		"sid", sessionID,
		"alias", alias,
		"tool", sl.Tool,
		"args_digest", sl.ArgsDigest,
		"repeat_count", sl.RepeatCount,
		"last_turn", sl.LastTurn,
		"issue_id", issueID,
	}
}

// ineffectiveSessionMinTurns is the turn-count floor below which we do NOT
// emit session.ineffective — short sessions are noise (a single-turn probe,
// a fast yes/no answer). 5 matches the IS-1100 spec.
const ineffectiveSessionMinTurns = 5

// ineffectiveTraceArgs builds the variadic kv slice traceEvent expects when
// emitting session.ineffective. Factored out so the kv shape is
// independently unit-testable without a DB. v1: model/complexity/persona
// callers pass "" if unknown (persona column landed in IS-1096 but model
// and complexity are still not stamped on zdx_claude_sessions — see
// IS-1101 dashboard ticket for the enrichment path).
func ineffectiveTraceArgs(sessionID, alias, model, complexity, persona, issueID string, turnCount int32) []any {
	return []any{
		"sid", sessionID,
		"alias", alias,
		"model", model,
		"complexity", complexity,
		"persona", persona,
		"issue_id", issueID,
		"turn_count", turnCount,
	}
}

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
			Persona          string `json:"persona" required:"false"`
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
		sess, created, err := h.getOrCreateAgentSession(ctx, p.ID, in.Sid, in.Body.IssueID, in.Body.Alias, in.Body.Title, in.Body.TodoID, in.Body.Persona)
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
			AbortReason string         `json:"abort_reason,omitempty" required:"false"`
			SpinLock    *SpinLockAbort `json:"spin_lock,omitempty" required:"false"`
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

		if in.Body.AbortReason == "spin_lock" && in.Body.SpinLock != nil {
			traceEvent(ctx, h.Q, p.ID, "session.aborted_spin_lock",
				spinLockTraceArgs(sess.SessionID, sess.Alias, sess.IssueID, *in.Body.SpinLock)...)
		}

		if computeIneffective(ctx, h.Q, p.ID, sess, in.Body.EventCount) {
			traceEvent(ctx, h.Q, p.ID, "session.ineffective",
				ineffectiveTraceArgs(sess.SessionID, sess.Alias, "", "", sess.Persona, sess.IssueID, in.Body.EventCount)...)
		}

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
		go h.analyzeSessionChurn(p.ID, p.Slug, sess)

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

// computeIneffective is the IS-1100 conjunction. Returns true when the
// session ran more than ineffectiveSessionMinTurns AND produced none of the
// durable-mutation signals tracked via zdx_revisions/zdx_comments. v1 caveats
// (telemetry-only; promotion to a hard gate is a separate ticket):
//
//   - turnCount is the EventCount the client reported. One Claude "turn" =
//     one assistant message + tool roundtrip; EventCount is a coarse upper
//     bound (it counts all NDJSON events). Good enough to filter out trivially
//     short sessions, which is all we need for v1.
//   - zdx_questions has no session_id column yet (route columns from IS-1091
//     not on main), so questions are NOT counted as a signal in v1.
//   - "commits attached" is approximated via issue-close revisions; we do not
//     join zdx_issues.completed_in_sha here because the close revision already
//     fires the same not-ineffective path.
//   - Comment attribution is approximated by (author_alias, created_at) since
//     zdx_comments lacks a session_id column.
func computeIneffective(ctx context.Context, q *db.Queries, projectID int32, sess db.GetClaudeSessionBySessionIDRow, turnCount int32) bool {
	if turnCount <= ineffectiveSessionMinTurns {
		return false
	}
	sigs, err := q.CountSessionEffectiveSignals(ctx, db.CountSessionEffectiveSignalsParams{
		ProjectID: projectID,
		SessionID: sess.SessionID,
	})
	if err != nil {
		return false
	}
	if sigs.TotalRevisions > 0 {
		return false
	}
	if sess.Alias != "" {
		comments, err := q.CountSessionLongCommentsByAlias(ctx, db.CountSessionLongCommentsByAliasParams{
			ProjectID:   projectID,
			AuthorAlias: sess.Alias,
			CreatedAt:   sess.CreatedAt,
		})
		if err == nil && comments > 0 {
			return false
		}
	}
	return true
}

// ── Shared helpers ─────────────────────────────────────────────────────────

// getOrCreateAgentSession looks up a session by (project_id, session_id) and
// creates one if it doesn't exist. The second return is true when this call
// created the row.
func (h *Handler) getOrCreateAgentSession(ctx context.Context, projectID int32, sessionID, issueID, alias, title string, todoID int32, persona string) (db.CreateClaudeSessionRow, bool, error) {
	todo := pgtype.Int4{}
	if todoID > 0 {
		todo = pgtype.Int4{Int32: todoID, Valid: true}
	}
	if persona == "" {
		persona = "dev"
	}
	sess, err := h.Q.CreateClaudeSession(ctx, db.CreateClaudeSessionParams{
		ProjectID: projectID,
		IssueID:   issueID,
		SessionID: sessionID,
		Title:     title,
		Alias:     alias,
		TodoID:    todo,
		Persona:   persona,
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

	if !IsInProjectScope(ctx, slug) {
		http.Error(w, `{"title":"Forbidden","status":403,"detail":"project not in token scope"}`, http.StatusForbidden)
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

// analyzeSessionChurn runs after a session closes. If the session was working
// a todo that has been claimed multiple times without resolution, it files a
// high-priority blocker issue so the churn is visible before more sessions burn.
func (h *Handler) analyzeSessionChurn(projectID int32, slug string, sess db.GetClaudeSessionBySessionIDRow) {
	// Auto-file is gated off — same reason as maybeAutoFileBlockIssue: a broken todo
	// source (e.g. read:comments before IS-677) fans out one Agent churn issue per
	// affected target, which themselves regenerate as comment targets and recurse.
	// Re-enable with AUTO_FILE_AGENT_FAILURES=true.
	if os.Getenv("AUTO_FILE_AGENT_FAILURES") != "true" {
		return
	}
	if !sess.TodoID.Valid || sess.TodoID.Int32 == 0 {
		return
	}
	ctx := WithSource(context.Background(), "churn-analysis")

	todo, err := h.Q.GetTodoByID(ctx, sess.TodoID.Int32)
	if err != nil || todo.ID == 0 {
		return
	}

	// Count how many times this todo has been claimed
	claimCount, err := h.Q.CountReservationsForTodo(ctx, db.CountReservationsForTodoParams{
		ProjectID: projectID,
		TodoID:    sess.TodoID.Int32,
	})
	if err != nil || claimCount < 3 {
		return // not enough churn to act on
	}

	// Already has a reference issue — churn was already surfaced
	if todo.ReferenceIssueID != "" {
		return
	}

	// Already blocked — cycle detection already caught it
	if todo.Blocked {
		return
	}

	// File a high-priority churn issue
	p, err := h.Q.GetProjectByID(ctx, projectID)
	if err != nil {
		return
	}
	issueID, err := h.Q.NextIssueID(ctx)
	if err != nil {
		return
	}

	sourceRef := ""
	if todo.IssueRef != "" {
		sourceRef = fmt.Sprintf(" (source: %s)", todo.IssueRef)
	}

	title := fmt.Sprintf("Agent churn: todo %q claimed %d times without resolution", todo.Key, claimCount)
	issueCtx := fmt.Sprintf(
		"Todo %q [%s] has been claimed by %d agent sessions without being resolved. "+
			"Each session burns tokens and time without progress. "+
			"This is higher priority than the source work%s because continued churn wastes resources.\n\n"+
			"Investigate why agents cannot complete this todo:\n"+
			"- Is the todo description clear enough for an agent to act on?\n"+
			"- Does the required CLI command or capability exist?\n"+
			"- Is there a missing pattern that would guide the agent?\n"+
			"- Should this todo be restructured, split, or marked interactive-only?\n\n"+
			"When this issue is closed the todo's reference issue clears and it re-enters the queue.",
		todo.Key, todo.Kind, claimCount, sourceRef,
	)

	issue, err := h.Q.CreateIssue(ctx, db.CreateIssueParams{
		ID:        issueID,
		ProjectID: p.ID,
		Title:     title,
		Context:   issueCtx,
		IssueType: "impl",
		Status:    "open",
		Priority:  "1", // higher than source issue
	})
	if err != nil {
		return
	}

	_ = h.Q.AppendIssueWork(ctx, db.AppendIssueWorkParams{
		IssueID: issue.ID,
		Agent:   "system",
		Note: fmt.Sprintf("[auto-filed] Post-session churn analysis: todo %q claimed %d times across agent sessions. "+
			"Last session: %s.", todo.Key, claimCount, sess.SessionID),
	})
	_ = h.Q.SetTodoReferenceIssue(ctx, db.SetTodoReferenceIssueParams{
		ProjectID:        projectID,
		Key:              todo.Key,
		ReferenceIssueID: issue.ID,
	})
}
