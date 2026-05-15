// Package envagentconn tracks live dx-envd WS connections — the per-env
// daemon scaffolded in IS-1233. Mirrors internal/server/agentconn but keyed
// on (env_id, agent_id) instead of project_slug, and intentionally leaner:
// deploy / schema-dump command channels live in IS-1230 / IS-1232.
package envagentconn

import (
	"fmt"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Conn represents a single connected env-agent daemon. The WS itself is the
// liveness signal (matches agentconn's spec-179 model); persistence of
// last-seen lives in zdx_env_agents (upserted on every heartbeat).
type Conn struct {
	EnvID          int32
	EnvName        string
	AgentID        string
	Hostname       string
	Version        string
	OS             string
	DeployedCommit string
	UptimeSecs     int64
	ConnectedAt    time.Time
	WS             *websocket.Conn
}

// Registry holds live env-agent connections keyed by agent_id.
type Registry struct {
	mu    sync.RWMutex
	conns map[string]*Conn
}

// New returns an empty Registry.
func New() *Registry {
	return &Registry{conns: make(map[string]*Conn)}
}

// Register adds c. Returns an error if agent_id is already connected.
func (r *Registry) Register(c *Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.conns[c.AgentID]; ok {
		return fmt.Errorf("env-agent %q already connected", c.AgentID)
	}
	r.conns[c.AgentID] = c
	return nil
}

// Unregister removes the agent with the given ID. No-op if absent.
func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	delete(r.conns, agentID)
	r.mu.Unlock()
}

// Get returns the Conn for agentID if present.
func (r *Registry) Get(agentID string) (*Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.conns[agentID]
	return c, ok
}

// List returns a snapshot of all connected env-agents.
func (r *Registry) List() []*Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Conn, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, c)
	}
	return out
}
