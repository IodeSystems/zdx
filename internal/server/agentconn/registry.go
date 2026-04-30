package agentconn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

// Conn represents a single connected agent daemon.
type Conn struct {
	AgentID        string
	Hostname       string
	Pid            int32
	Capabilities   []string
	WorktreePath   string
	WorktreeBranch string
	ConnectedAt    time.Time
	WS             *websocket.Conn
}

// Registry holds live agent connections. Connection itself is the liveness
// signal — no heartbeat timestamps (spec 179).
type Registry struct {
	mu           sync.RWMutex
	conns        map[string]*Conn
	onDisconnect func(agentID string)
}

// NewRegistry creates a Registry. onDisconnect is called from Unregister with
// the agent ID; pass nil if no callback is needed.
func NewRegistry(onDisconnect func(agentID string)) *Registry {
	return &Registry{conns: make(map[string]*Conn), onDisconnect: onDisconnect}
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

// Unregister removes the agent with the given ID and fires the OnDisconnect
// callback if one was set. No-op if the agent is not present.
func (r *Registry) Unregister(agentID string) {
	r.mu.Lock()
	_, present := r.conns[agentID]
	delete(r.conns, agentID)
	r.mu.Unlock()
	if present && r.onDisconnect != nil {
		r.onDisconnect(agentID)
	}
}

// Get returns the Conn for agentID if present.
func (r *Registry) Get(agentID string) (*Conn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.conns[agentID]
	return c, ok
}

// SendAgentCommand writes raw JSON data to the named agent's WS connection.
// Returns an error if the agent is not currently connected.
func (r *Registry) SendAgentCommand(ctx context.Context, agentID string, data []byte) error {
	c, ok := r.Get(agentID)
	if !ok {
		return fmt.Errorf("agent %q not connected", agentID)
	}
	return c.WS.Write(ctx, websocket.MessageText, data)
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
