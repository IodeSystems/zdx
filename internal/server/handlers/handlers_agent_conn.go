package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/agentconn"
)

// AgentRegisterMsg is the first frame sent by the daemon after upgrading.
type AgentRegisterMsg struct {
	AgentID        string   `json:"agent_id"`
	Hostname       string   `json:"hostname"`
	Pid            int32    `json:"pid"`
	Capabilities   []string `json:"capabilities"`
	WorktreePath   string   `json:"worktree_path"`
	WorktreeBranch string   `json:"worktree_branch"`
}

// HandleAgentConnect upgrades the HTTP connection to WebSocket, reads the
// registration handshake, and holds the connection as the liveness signal.
// q may be nil (e.g. in tests without a DB); DB updates are skipped when nil.
func HandleAgentConnect(registry *agentconn.Registry, q *db.Queries) http.HandlerFunc {
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

		c := &agentconn.Conn{
			AgentID:        reg.AgentID,
			Hostname:       reg.Hostname,
			Pid:            reg.Pid,
			Capabilities:   reg.Capabilities,
			WorktreePath:   reg.WorktreePath,
			WorktreeBranch: reg.WorktreeBranch,
			ConnectedAt:    time.Now(),
			WS:             conn,
		}
		if err := registry.Register(c); err != nil {
			conn.Close(websocket.StatusPolicyViolation, err.Error()) //nolint:errcheck
			return
		}
		defer registry.Unregister(reg.AgentID)

		// Clear disconnect_at and restore active status on reconnect.
		if q != nil {
			if err := q.MarkAgentConnected(ctx, reg.AgentID); err != nil {
				log.Printf("agent connect: mark connected %s: %v", reg.AgentID, err)
			}
		}

		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now().UTC()})
		if err := conn.Write(ctx, websocket.MessageText, ack); err != nil {
			log.Printf("agent connect: write ack: %v", err)
			return
		}

		log.Printf("agent %s connected (host=%s pid=%d)", reg.AgentID, reg.Hostname, reg.Pid)

		// Hold connection until the client disconnects or ctx is cancelled.
		// The connection itself is the liveness signal (spec 179).
		for {
			_, _, err := conn.Read(ctx)
			if err != nil {
				status := websocket.CloseStatus(err)
				if status == websocket.StatusNormalClosure {
					// Check for our graceful-shutdown reason.
					log.Printf("agent %s shutdown gracefully", reg.AgentID)
				} else {
					log.Printf("agent %s disconnected: %v", reg.AgentID, err)
				}
				return
			}
		}
	}
}
