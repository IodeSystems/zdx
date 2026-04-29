package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_AgentHeartbeatLoopRefreshesTimestamp is the demo for spec 70 on
// feature dx-todo-agent: an agent running solo with --agent-id ticks the
// heartbeat loop, and each tick refreshes last_heartbeat for staleness
// detection.
//
// Production drives this via cli.HeartbeatLoop (interval=60s) which POSTs to
// /api/agents/{id}/heartbeat each tick. The demo records two explicit
// heartbeat POSTs to make the protocol legible, then verifies last_heartbeat
// advanced after each.
func TestDemoAPI_AgentHeartbeatLoopRefreshesTimestamp(t *testing.T) {
	rec := newApiRecorder(t, "agent-heartbeat-loop")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_agent_heartbeat_test.go", Note: "heartbeat-loop demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent_shared.go", Note: "HeartbeatLoop drives periodic heartbeat POSTs"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_agents.go", Note: "POST /api/agents/{id}/heartbeat handler"})
	rec.AddCoderef(coderef{FilePath: "queries/agents.sql", Note: "UpdateAgentHeartbeat sets last_heartbeat = NOW()"})
	t.Cleanup(rec.Save)

	const slug = "demo-agent-heartbeat"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Agent Heartbeat",
	}, nil))

	const agentID = "demo-heartbeat-agent"
	mustOK(t, rec.Do(http.MethodPost, "/api/agents/register", map[string]any{
		"slug":            slug,
		"id":              agentID,
		"session_id":      "demo-heartbeat-session",
		"worktree_path":   "",
		"worktree_branch": "",
		"pid":             0,
		"status":          "active",
		"task_group":      "",
		"compose_project": "",
		"server_port":     0,
		"database_url":    "",
		"valkey_url":      "",
	}, nil))

	initial := getAgentHeartbeatVia(t, rec, agentID)

	// Each HeartbeatLoop tick POSTs this endpoint; simulate one tick.
	// Sleep > 1s between samples because /api/agents/{id} formats timestamps
	// at second resolution (handlers.fmtTS → time.RFC3339).
	time.Sleep(1100 * time.Millisecond)
	heartbeatPath := fmt.Sprintf("/api/agents/%s/heartbeat", agentID)
	mustNoContent(t, rec.Do(http.MethodPost, heartbeatPath, nil, nil))

	afterFirst := getAgentHeartbeatVia(t, rec, agentID)
	if !afterFirst.After(initial) {
		t.Fatalf("first tick did not advance last_heartbeat: initial=%s after=%s",
			initial.Format(time.RFC3339), afterFirst.Format(time.RFC3339))
	}

	// Second tick: every subsequent fire keeps the agent alive for staleness checks.
	time.Sleep(1100 * time.Millisecond)
	mustNoContent(t, rec.Do(http.MethodPost, heartbeatPath, nil, nil))

	afterSecond := getAgentHeartbeatVia(t, rec, agentID)
	if !afterSecond.After(afterFirst) {
		t.Fatalf("second tick did not advance last_heartbeat: first=%s second=%s",
			afterFirst.Format(time.RFC3339), afterSecond.Format(time.RFC3339))
	}
}

func mustNoContent(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", resp.StatusCode)
	}
}

func getAgentHeartbeatVia(t *testing.T, rec *ApiDemoRecorder, agentID string) time.Time {
	t.Helper()
	var resp struct {
		LastHeartbeat string `json:"last_heartbeat"`
	}
	mustOK(t, rec.Do(http.MethodGet,
		fmt.Sprintf("/api/agents/%s", agentID), nil, &resp))
	ts, err := time.Parse(time.RFC3339, resp.LastHeartbeat)
	if err != nil {
		t.Fatalf("parse last_heartbeat %q: %v", resp.LastHeartbeat, err)
	}
	return ts
}
