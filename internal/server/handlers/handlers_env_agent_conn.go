package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/envagentconn"
)

// EnvAgentStore is the narrow surface the env-agent connect handler needs.
// *db.Queries satisfies it in prod; tests pass a mock so unit tests don't
// require a real Postgres.
type EnvAgentStore interface {
	GetProjectBySlug(ctx context.Context, slug string) (db.ZdxProject, error)
	GetEnvironment(ctx context.Context, arg db.GetEnvironmentParams) (db.GetEnvironmentRow, error)
	UpsertEnvAgentHeartbeat(ctx context.Context, arg db.UpsertEnvAgentHeartbeatParams) (db.ZdxEnvAgent, error)
}

// EnvAgentRegisterMsg is the first frame sent by dx-envd after upgrading.
//
// ProjectSlug + EnvName together resolve to a zdx_environments row. The
// admin-only auth gate (IS-1227) means we trust the daemon to declare its
// project; IS-1228 lands per-env scoped tokens that will tighten this.
type EnvAgentRegisterMsg struct {
	AgentID        string `json:"agent_id"`
	ProjectSlug    string `json:"project_slug"`
	EnvName        string `json:"env_name"`
	Hostname       string `json:"hostname"`
	Version        string `json:"version"`
	OS             string `json:"os"`
	DeployedCommit string `json:"deployed_commit"`
	UptimeSecs     int64  `json:"uptime_secs"`
}

// EnvAgentHeartbeatMsg is the recurring liveness frame from dx-envd.
// Unknown frame types are silently dropped so older servers don't break newer
// daemons.
type EnvAgentHeartbeatMsg struct {
	Type           string `json:"type"`
	Version        string `json:"version"`
	OS             string `json:"os"`
	DeployedCommit string `json:"deployed_commit"`
	UptimeSecs     int64  `json:"uptime_secs"`
}

// HandleEnvAgentConnect upgrades to WS, reads the handshake, resolves the
// environment, and runs the heartbeat read loop. The persistent zdx_env_agents
// row is upserted on registration and on every heartbeat so `dx env list`
// (and IS-1230's deploy routing) can see env liveness even across daemon
// restarts.
//
// Store may be nil — without a DB we still accept and ack the connection so
// unit tests can drive the dispatch path without a real Postgres.
func (h *Handler) HandleEnvAgentConnect(reg *envagentconn.Registry, store EnvAgentStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			InsecureSkipVerify: true, // API-key auth happens in apiKeyMiddleware
		})
		if err != nil {
			log.Printf("env-agent connect: websocket accept: %v", err)
			return
		}

		ctx := r.Context()

		// TODO(IS-1228): when per-env scoped tokens land, drop the admin-only
		// gate and authorize against the token's env scope instead.
		role := UserRoleFromContext(ctx)
		if role != "admin" {
			conn.Close(websocket.StatusPolicyViolation, "env-agent connect requires an admin token") //nolint:errcheck
			return
		}

		_, msg, err := conn.Read(ctx)
		if err != nil {
			conn.Close(websocket.StatusProtocolError, "read handshake failed") //nolint:errcheck
			return
		}

		var hello EnvAgentRegisterMsg
		if err := json.Unmarshal(msg, &hello); err != nil || hello.AgentID == "" || hello.ProjectSlug == "" || hello.EnvName == "" {
			conn.Close(websocket.StatusProtocolError, "invalid handshake") //nolint:errcheck
			return
		}

		// Honor token project_scope when set; an unscoped admin token can
		// connect any env. Mirrors authorizeAgentRegister's scope check.
		scope := ProjectScopeFromContext(ctx)
		if len(scope) > 0 {
			allowed := false
			for _, s := range scope {
				if s == hello.ProjectSlug {
					allowed = true
					break
				}
			}
			if !allowed {
				conn.Close(websocket.StatusPolicyViolation, "token not in project scope") //nolint:errcheck
				return
			}
		}

		var envID int32
		var envName string = hello.EnvName
		if store != nil {
			p, err := store.GetProjectBySlug(ctx, hello.ProjectSlug)
			if err != nil {
				log.Printf("env-agent connect: lookup project %q: %v", hello.ProjectSlug, err)
				conn.Close(websocket.StatusPolicyViolation, "unknown project") //nolint:errcheck
				return
			}
			env, err := store.GetEnvironment(ctx, db.GetEnvironmentParams{ProjectID: p.ID, Name: hello.EnvName})
			if err != nil {
				log.Printf("env-agent connect: lookup env %q/%q: %v", hello.ProjectSlug, hello.EnvName, err)
				conn.Close(websocket.StatusPolicyViolation, "unknown environment") //nolint:errcheck
				return
			}
			envID = env.ID
			envName = env.Name

			if _, err := store.UpsertEnvAgentHeartbeat(ctx, db.UpsertEnvAgentHeartbeatParams{
				EnvID:          envID,
				AgentID:        hello.AgentID,
				Hostname:       hello.Hostname,
				Version:        hello.Version,
				Os:             hello.OS,
				DeployedCommit: hello.DeployedCommit,
				UptimeSecs:     hello.UptimeSecs,
			}); err != nil {
				log.Printf("env-agent connect: upsert heartbeat %s: %v", hello.AgentID, err)
				conn.Close(websocket.StatusInternalError, "persist failed") //nolint:errcheck
				return
			}
		}

		c := &envagentconn.Conn{
			EnvID:          envID,
			EnvName:        envName,
			AgentID:        hello.AgentID,
			Hostname:       hello.Hostname,
			Version:        hello.Version,
			OS:             hello.OS,
			DeployedCommit: hello.DeployedCommit,
			UptimeSecs:     hello.UptimeSecs,
			ConnectedAt:    time.Now(),
			WS:             conn,
		}
		if err := reg.Register(c); err != nil {
			conn.Close(websocket.StatusPolicyViolation, err.Error()) //nolint:errcheck
			return
		}
		defer reg.Unregister(hello.AgentID)

		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now().UTC()})
		if err := conn.Write(ctx, websocket.MessageText, ack); err != nil {
			log.Printf("env-agent connect: write ack: %v", err)
			return
		}

		log.Printf("env-agent %s connected (env=%s/%s host=%s)", hello.AgentID, hello.ProjectSlug, hello.EnvName, hello.Hostname)

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				status := websocket.CloseStatus(err)
				if status == websocket.StatusNormalClosure {
					log.Printf("env-agent %s shutdown gracefully", hello.AgentID)
				} else {
					log.Printf("env-agent %s disconnected: %v", hello.AgentID, err)
				}
				return
			}
			h.dispatchEnvAgentMessage(ctx, store, envID, hello.AgentID, data)
		}
	}
}

// dispatchEnvAgentMessage routes one daemon-sent frame to its handler. Unknown
// types are silently dropped so older servers don't break newer daemons.
func (h *Handler) dispatchEnvAgentMessage(ctx context.Context, store EnvAgentStore, envID int32, agentID string, data []byte) {
	var env struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return
	}
	switch env.Type {
	case "heartbeat":
		var hb EnvAgentHeartbeatMsg
		if err := json.Unmarshal(data, &hb); err != nil {
			log.Printf("env-agent %s: malformed heartbeat: %v", agentID, err)
			return
		}
		if store == nil {
			return
		}
		if _, err := store.UpsertEnvAgentHeartbeat(ctx, db.UpsertEnvAgentHeartbeatParams{
			EnvID:          envID,
			AgentID:        agentID,
			Version:        hb.Version,
			Os:             hb.OS,
			DeployedCommit: hb.DeployedCommit,
			UptimeSecs:     hb.UptimeSecs,
		}); err != nil {
			log.Printf("env-agent %s: upsert heartbeat: %v", agentID, err)
		}
	}
}
