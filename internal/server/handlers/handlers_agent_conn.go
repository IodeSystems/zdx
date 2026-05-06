package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/agentconn"
)

// AgentRegisterMsg is the first frame sent by the daemon after upgrading.
//
// ProjectSlug is the project the daemon is registering under. Empty string
// means the agent is registering into the server-wide global pool — visible
// in the top-level /agents nav, not bound to any one project. The slug
// must match a real project when non-empty.
//
// Idle reports the agent's initial work-loop state. False = start
// claiming work immediately (today's `dx agent loop` behavior). True =
// register but stay passive; operator activates via UI / control message.
// Mostly relevant for global agents (no project queue to claim from
// without an assignment), but also useful for project-scoped agents that
// want to show up as "ready, waiting" before being told to do anything.
type AgentRegisterMsg struct {
	AgentID        string   `json:"agent_id"`
	Hostname       string   `json:"hostname"`
	Pid            int32    `json:"pid"`
	Capabilities   []string `json:"capabilities"`
	WorktreePath   string   `json:"worktree_path"`
	WorktreeBranch string   `json:"worktree_branch"`
	ProjectSlug    string   `json:"project_slug,omitempty"`
	Idle           bool     `json:"idle,omitempty"`
}

// HandleAgentConnect upgrades the HTTP connection to WebSocket, reads the
// registration handshake, and holds the connection as the liveness signal.
// Subsequent frames are dispatched by message type — currently only
// "agent.audit_event" is consumed; unknown types are ignored so older servers
// don't break newer daemons.
//
// Q may be nil (e.g. in tests without a DB); DB updates and persistence are
// skipped when nil. Broker may also be nil for tests.
func (h *Handler) HandleAgentConnect(registry *agentconn.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // API-key auth happens in middleware
		})
		if err != nil {
			log.Printf("agent connect: websocket accept: %v", err)
			return
		}

		ctx := r.Context()

		_, msg, err := conn.Read(ctx)
		if err != nil {
			conn.Close(websocket.StatusProtocolError, "read handshake failed") //nolint:errcheck
			return
		}

		var reg AgentRegisterMsg
		if err := json.Unmarshal(msg, &reg); err != nil || reg.AgentID == "" {
			conn.Close(websocket.StatusProtocolError, "invalid handshake") //nolint:errcheck
			return
		}

		// Authorize the registration against the token already validated by
		// apiKeyMiddleware (role + project_scope are stamped on ctx). Global
		// registrations want an admin-tier token (role=admin, unscoped).
		// Project-scoped registrations must fall inside the token's scope.
		// Transitional: globals authenticated by a project token are
		// accepted but logged as deprecated so operators can migrate
		// without surprise breakage.
		role := UserRoleFromContext(ctx)
		scope := ProjectScopeFromContext(ctx)
		if reason, deprecation := authorizeAgentRegister(role, scope, reg.ProjectSlug, reg.AgentID); reason != "" {
			log.Printf("agent connect: %s rejected: %s", reg.AgentID, reason)
			conn.Close(websocket.StatusPolicyViolation, reason) //nolint:errcheck
			return
		} else if deprecation != "" {
			log.Printf("agent connect: %s", deprecation)
		}

		c := &agentconn.Conn{
			AgentID:        reg.AgentID,
			Hostname:       reg.Hostname,
			Pid:            reg.Pid,
			Capabilities:   reg.Capabilities,
			WorktreePath:   reg.WorktreePath,
			WorktreeBranch: reg.WorktreeBranch,
			ProjectSlug:    reg.ProjectSlug,
			Idle:           reg.Idle,
			ConnectedAt:    time.Now(),
			WS:             conn,
		}
		if err := registry.Register(c); err != nil {
			conn.Close(websocket.StatusPolicyViolation, err.Error()) //nolint:errcheck
			return
		}
		defer registry.Unregister(reg.AgentID)

		// Persist (upsert) the agent row. Until phase 2 the connect path
		// only registered in-memory; without a DB row, /api/agents and
		// assign/unassign couldn't see global-pool agents. Project-scoped
		// upserts go through RegisterAgent (sets originally_global=false on
		// insert); empty ProjectSlug routes to RegisterGlobalAgent
		// (originally_global=true, project_id NULL). originally_global is
		// immutable on upsert in either query.
		if h.Q != nil {
			if reg.ProjectSlug == "" {
				if _, err := h.Q.RegisterGlobalAgent(ctx, db.RegisterGlobalAgentParams{
					ID:             reg.AgentID,
					SessionID:      "",
					WorktreePath:   reg.WorktreePath,
					WorktreeBranch: reg.WorktreeBranch,
					Pid:            reg.Pid,
					Status:         "active",
					Idle:           reg.Idle,
				}); err != nil {
					log.Printf("agent connect: register global %s: %v", reg.AgentID, err)
				}
			} else {
				p, err := h.Q.GetProjectBySlug(ctx, reg.ProjectSlug)
				if err != nil {
					log.Printf("agent connect: lookup project %q for %s: %v", reg.ProjectSlug, reg.AgentID, err)
				} else {
					if _, err := h.Q.RegisterAgent(ctx, db.RegisterAgentParams{
						ID:             reg.AgentID,
						ProjectID:      pgtype.Int4{Int32: p.ID, Valid: true},
						SessionID:      "",
						WorktreePath:   reg.WorktreePath,
						WorktreeBranch: reg.WorktreeBranch,
						Pid:            reg.Pid,
						Status:         "active",
					}); err != nil {
						log.Printf("agent connect: register project %s: %v", reg.AgentID, err)
					} else if err := h.Q.SetAgentIdle(ctx, db.SetAgentIdleParams{ID: reg.AgentID, Idle: reg.Idle}); err != nil {
						log.Printf("agent connect: set idle %s: %v", reg.AgentID, err)
					}
				}
			}
			if err := h.Q.MarkAgentConnected(ctx, reg.AgentID); err != nil {
				log.Printf("agent connect: mark connected %s: %v", reg.AgentID, err)
			}
		}

		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now().UTC()})
		if err := conn.Write(ctx, websocket.MessageText, ack); err != nil {
			log.Printf("agent connect: write ack: %v", err)
			return
		}

		log.Printf("agent %s connected (host=%s pid=%d)", reg.AgentID, reg.Hostname, reg.Pid)

		// Dispatch incoming frames by type. The connection itself remains the
		// liveness signal (spec 179) — Read errors close the loop.
		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				status := websocket.CloseStatus(err)
				if status == websocket.StatusNormalClosure {
					log.Printf("agent %s shutdown gracefully", reg.AgentID)
				} else {
					log.Printf("agent %s disconnected: %v", reg.AgentID, err)
				}
				return
			}
			h.dispatchAgentMessage(ctx, reg.AgentID, data)
		}
	}
}

// authorizeAgentRegister gates the WS handshake against the token's role +
// project_scope. Returns ("", "") when the registration is allowed without
// remarks. Returns ("", deprecation) when the call should proceed but a
// log line should fire (transitional path for globals authenticated with a
// project-scoped or non-admin token). Returns (reason, "") when the WS
// must be closed with a policy-violation — used today for project-scoped
// tokens registering against a different project. The plan to flip the
// global/non-admin path from deprecation-log to hard-reject lives in
// docs/plan.md (phase 3).
func authorizeAgentRegister(role string, scope []string, projectSlug, agentID string) (reason, deprecation string) {
	if projectSlug == "" {
		// Global registration. Admin-tier = role=admin AND unscoped (the
		// same definition handlers_admin_tokens.go uses for /api/admin/).
		if role != "admin" || len(scope) > 0 {
			return "", fmt.Sprintf("DEPRECATED: agent %s registering global with non-admin token (role=%q, scoped=%t); a future release will require an admin-tier token", agentID, role, len(scope) > 0)
		}
		return "", ""
	}
	// Project-scoped registration. Empty scope = unrestricted (admin or a
	// generic CLI key); non-empty scope must contain the registering slug.
	if len(scope) == 0 {
		return "", ""
	}
	for _, s := range scope {
		if s == projectSlug {
			return "", ""
		}
	}
	return "token not in project scope", ""
}

// AgentAuditEventMsg is the daemon-sent payload for tool-call/file-edit/shell
// audit events streamed over the persistent agent connection.
type AgentAuditEventMsg struct {
	Type      string          `json:"type"`
	AgentID   string          `json:"agent_id"`
	SessionID string          `json:"session_id"`
	TurnID    string          `json:"turn_id"`
	EventType string          `json:"event_type"`
	Payload   json.RawMessage `json:"payload"`
}

// dispatchAgentMessage routes one daemon-sent frame to its handler. Unknown
// types are silently dropped so older servers don't break newer daemons.
func (h *Handler) dispatchAgentMessage(ctx context.Context, connAgentID string, data []byte) {
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	switch env.Type {
	case "agent.audit_event":
		var msg AgentAuditEventMsg
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("agent %s: malformed audit event: %v", connAgentID, err)
			return
		}
		if msg.AgentID == "" {
			msg.AgentID = connAgentID
		}
		h.handleAgentAuditEvent(ctx, msg)
	}
}

// handleAgentAuditEvent persists one audit event to zdx_claude_events (when a
// matching session exists) and broadcasts it on the per-session and per-agent
// WS channels so operators see live activity. DB failures are logged but never
// disconnect the agent — the live stream is the load-bearing signal.
func (h *Handler) handleAgentAuditEvent(ctx context.Context, msg AgentAuditEventMsg) {
	auditPayload := map[string]any{
		"agent_id":   msg.AgentID,
		"session_id": msg.SessionID,
		"turn_id":    msg.TurnID,
		"event_type": msg.EventType,
		"payload":    rawOrEmpty(msg.Payload),
	}
	if h.Broker != nil {
		h.Broker.PublishAuditEvent(msg.AgentID, auditPayload)
	}

	if h.Q == nil || msg.SessionID == "" {
		return
	}

	agent, err := h.Q.GetAgent(ctx, msg.AgentID)
	if err != nil {
		log.Printf("audit: lookup agent %s: %v", msg.AgentID, err)
		return
	}
	// Global-pool agents (ProjectID NULL) have no project to attribute
	// audit events to. Drop silently — the agent's WS audit-channel
	// broadcast above already gave the operator the live signal.
	if !agent.ProjectID.Valid {
		return
	}
	project, err := h.Q.GetProjectByID(ctx, agent.ProjectID.Int32)
	if err != nil {
		log.Printf("audit: lookup project %d: %v", agent.ProjectID.Int32, err)
		return
	}
	sess, err := h.Q.GetClaudeSessionBySessionID(ctx, db.GetClaudeSessionBySessionIDParams{
		ProjectID: project.ID,
		SessionID: msg.SessionID,
	})
	if err != nil {
		// No matching session yet (e.g. /create wasn't called): drop silently.
		// Re-emitting on the audit channel above already covers liveness.
		return
	}

	maxSeq, err := h.Q.GetMaxClaudeEventSeq(ctx, sess.ID)
	if err != nil {
		log.Printf("audit: max seq for session %d: %v", sess.ID, err)
		return
	}
	eventBytes := []byte(rawOrEmptyBytes(msg.Payload))
	h.ingestAgentEvent(ctx, project.Slug, sess.SessionID, sess.ID, maxSeq+1, msg.EventType, eventBytes, msg.AgentID, "", "")
}

// rawOrEmpty returns msg.Payload as a parsed value when it's a JSON object,
// otherwise as a RawMessage (or empty object for nil). The audit channel
// payload prefers a structured value so subscribers can JSON-walk it.
func rawOrEmpty(raw json.RawMessage) any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var parsed any
	if err := json.Unmarshal(raw, &parsed); err == nil {
		return parsed
	}
	return raw
}

// rawOrEmptyBytes returns the bytes of msg.Payload, or `{}` when empty so the
// jsonb column never receives an invalid value.
func rawOrEmptyBytes(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return []byte("{}")
	}
	return []byte(raw)
}
