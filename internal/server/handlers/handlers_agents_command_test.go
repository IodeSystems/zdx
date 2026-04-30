package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

// mockCommander records SendAgentCommand calls and optionally returns an error.
type mockCommander struct {
	lastAgentID string
	lastData    []byte
	err         error
}

func (m *mockCommander) SendAgentCommand(_ context.Context, agentID string, data []byte) error {
	m.lastAgentID = agentID
	m.lastData = data
	return m.err
}

func TestAgentCommandPause(t *testing.T) {
	mc := &mockCommander{}
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{AgentCommander: mc}}
	h.registerAgentRoutes(api)

	resp := api.Post("/api/agents/abc123/command", map[string]string{"command": "pause"})
	if resp.Code != 204 {
		t.Fatalf("status = %d, want 204; body: %s", resp.Code, resp.Body)
	}
	if mc.lastAgentID != "abc123" {
		t.Errorf("agentID = %q, want %q", mc.lastAgentID, "abc123")
	}
	var msg map[string]string
	if err := json.Unmarshal(mc.lastData, &msg); err != nil {
		t.Fatalf("unmarshal sent msg: %v", err)
	}
	if msg["type"] != "pause" {
		t.Errorf("msg type = %q, want %q", msg["type"], "pause")
	}
}

func TestAgentCommandUnknownAgent(t *testing.T) {
	mc := &mockCommander{err: errors.New("not connected")}
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{AgentCommander: mc}}
	h.registerAgentRoutes(api)

	resp := api.Post("/api/agents/unknown/command", map[string]string{"command": "pause"})
	if resp.Code != 404 {
		t.Fatalf("status = %d, want 404; body: %s", resp.Code, resp.Body)
	}
}
