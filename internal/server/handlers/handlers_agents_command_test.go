package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/iodesystems/zdx-go/internal/db"
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

// mockStatusUpdater records UpdateAgentStatus calls. A nil-but-typed value is
// the right way to assert "no calls were made" — see TestAgentCommandDrainNoStatusUpdate.
type mockStatusUpdater struct {
	calls []db.UpdateAgentStatusParams
}

func (m *mockStatusUpdater) UpdateAgentStatus(_ context.Context, arg db.UpdateAgentStatusParams) error {
	m.calls = append(m.calls, arg)
	return nil
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

// TestAgentCommandPauseUpdatesStatus verifies the pause path persists status="paused"
// so dx agent list (IS-819) reflects the new state. This is the load-bearing
// piece IS-820 promises — a regression here silently breaks visibility.
func TestAgentCommandPauseUpdatesStatus(t *testing.T) {
	mc := &mockCommander{}
	su := &mockStatusUpdater{}
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{AgentCommander: mc, AgentStatusUpdater: su}}
	h.registerAgentRoutes(api)

	resp := api.Post("/api/agents/abc123/command", map[string]string{"command": "pause"})
	if resp.Code != 204 {
		t.Fatalf("status = %d, want 204; body: %s", resp.Code, resp.Body)
	}
	if len(su.calls) != 1 {
		t.Fatalf("UpdateAgentStatus calls = %d, want 1", len(su.calls))
	}
	if got := su.calls[0]; got.ID != "abc123" || got.Status != "paused" {
		t.Errorf("UpdateAgentStatus arg = %+v, want {ID:abc123 Status:paused}", got)
	}
}

func TestAgentCommandResumeUpdatesStatus(t *testing.T) {
	mc := &mockCommander{}
	su := &mockStatusUpdater{}
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{AgentCommander: mc, AgentStatusUpdater: su}}
	h.registerAgentRoutes(api)

	resp := api.Post("/api/agents/abc123/command", map[string]string{"command": "resume"})
	if resp.Code != 204 {
		t.Fatalf("status = %d, want 204; body: %s", resp.Code, resp.Body)
	}
	if len(su.calls) != 1 {
		t.Fatalf("UpdateAgentStatus calls = %d, want 1", len(su.calls))
	}
	if got := su.calls[0]; got.ID != "abc123" || got.Status != "active" {
		t.Errorf("UpdateAgentStatus arg = %+v, want {ID:abc123 Status:active}", got)
	}
}

// TestAgentCommandDrainNoStatusUpdate asserts drain (and other non-pause/resume
// commands) do NOT touch agent status — drain is not in the statusFor map at
// handlers_agents.go and persisting "draining" here would race the DB-side
// drain workflow.
func TestAgentCommandDrainNoStatusUpdate(t *testing.T) {
	mc := &mockCommander{}
	su := &mockStatusUpdater{}
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{AgentCommander: mc, AgentStatusUpdater: su}}
	h.registerAgentRoutes(api)

	resp := api.Post("/api/agents/abc123/command", map[string]string{"command": "drain"})
	if resp.Code != 204 {
		t.Fatalf("status = %d, want 204; body: %s", resp.Code, resp.Body)
	}
	if len(su.calls) != 0 {
		t.Errorf("UpdateAgentStatus calls = %d, want 0; got %+v", len(su.calls), su.calls)
	}
}
