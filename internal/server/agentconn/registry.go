package agentconn

import (
	"fmt"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Conn represents a single connected agent daemon.
type Conn struct {
	AgentID       string
	Hostname      string
	Pid           int32
	Capabilities  []string
	WorktreePath  string
	WorktreeBranch string
	ConnectedAt   time.Time
	WS            *websocket.Conn
}

// Registry holds live agent connections. Connection itself is the liveness
// signal — no heartbeat timestamps (spec 179).
type Registry struct {
	mu    sync.RWMutex
	conns map[string]*Conn
}

func NewRegistry() *Registry {
	return &Registry{conns: make(map[string]*Conn)}
}

// Register adds c to the registry. Returns an error if agent_id is already connected.
func (r *Registry) Register(c *Conn) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.conns[c.AgentID]; ok {
		return fmt.Errorf("agent %q already connected", c.AgentID)
	}
	r.conns[c.AgentID] = c
	return nil
}

// Unregister removes the agent with the given ID (no-op if not present).
func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.conns, agentID)
}

// Get returns the Conn for agentID if present.
func (r *Registry) Get(agentID string) (*Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.conns[agentID]
	return c, ok
}

// List returns a snapshot of all connected agents.
func (r *Registry) List() []*Conn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Conn, 0, len(r.conns))
	for _, c := range r.conns {
		out = append(out, c)
	}
	return out
}
