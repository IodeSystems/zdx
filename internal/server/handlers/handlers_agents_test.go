package handlers

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// stubRegistry implements AgentConnRegistry for testing.
type stubRegistry struct {
	connected map[string]time.Time
}

func (s *stubRegistry) IsConnected(agentID string) bool {
	_, ok := s.connected[agentID]
	return ok
}

func (s *stubRegistry) ConnectedAt(agentID string) time.Time {
	return s.connected[agentID]
}

func newStubRegistry(ids ...string) *stubRegistry {
	m := make(map[string]time.Time, len(ids))
	for _, id := range ids {
		m[id] = time.Now()
	}
	return &stubRegistry{connected: m}
}

func makeAgentRow(id, status string) db.ZdxAgent {
	return db.ZdxAgent{
		ID:            id,
		Status:        status,
		LastHeartbeat: pgtype.Timestamptz{Valid: false},
		CreatedAt:     pgtype.Timestamptz{Valid: false},
	}
}

func TestAgentItemFromConnectionState(t *testing.T) {
	reg := newStubRegistry("agent-A")

	// Connected agent
	item := agentItemFrom(makeAgentRow("agent-A", "active"), reg)
	if item.ConnectionState != "connected" {
		t.Errorf("agent-A ConnectionState = %q, want connected", item.ConnectionState)
	}
	if item.ConnectedAt == "" {
		t.Error("agent-A ConnectedAt should be non-empty when connected")
	}

	// Disconnected agent (not in registry)
	item = agentItemFrom(makeAgentRow("agent-B", "active"), reg)
	if item.ConnectionState != "disconnected" {
		t.Errorf("agent-B ConnectionState = %q, want disconnected", item.ConnectionState)
	}
	if item.ConnectedAt != "" {
		t.Errorf("agent-B ConnectedAt should be empty when disconnected, got %q", item.ConnectedAt)
	}

	// Paused agent in registry — status wins over registry presence
	reg2 := newStubRegistry("agent-C")
	item = agentItemFrom(makeAgentRow("agent-C", "paused"), reg2)
	if item.ConnectionState != "paused" {
		t.Errorf("agent-C (paused, in registry) ConnectionState = %q, want paused", item.ConnectionState)
	}

	// Paused agent not in registry
	item = agentItemFrom(makeAgentRow("agent-D", "paused"), reg2)
	if item.ConnectionState != "paused" {
		t.Errorf("agent-D (paused, not in registry) ConnectionState = %q, want paused", item.ConnectionState)
	}

	// nil registry
	item = agentItemFrom(makeAgentRow("agent-E", "active"), nil)
	if item.ConnectionState != "disconnected" {
		t.Errorf("agent-E (nil reg) ConnectionState = %q, want disconnected", item.ConnectionState)
	}
}
