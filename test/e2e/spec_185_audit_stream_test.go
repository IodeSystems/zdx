package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// TestSpec185_AuditEventStream is the canonical demo for spec 185 on feature
// agent-managed-worker (issue IS-605):
//
//	Given an agent executing a turn, when the daemon emits tool calls, file
//	edits, and shell commands, then it streams these events to dx-server over
//	the connection so the server records a live audit trail attributable to
//	the agent_id and task.
//
// The audit trail must be:
//   - persisted (rows queryable via GET /api/dx/claude/sessions/{id}/events
//     with the right event_type, agent_id, and payload),
//   - attributable (each row carries the originating agent_id, and the parent
//     session is linked to the originating todo_id),
//   - and live (each event is broadcast as agent.event on the session's WS
//     channel as soon as the server persists it).
//
// The test drives the existing audit ingestion path: the daemon-side equivalent
// is dx agent claude / local, which posts NDJSON batches of {seq, event_type,
// event_json, agent_id} to /api/dx/agent/sessions/{sid}/events. The server
// persists each line to zdx_claude_events and publishes agent.event on
// project:<slug>:claude:<sid>.
func TestSpec185_AuditEventStream(t *testing.T) {
	requiresAPI(t)

	d := NewApiDriver(t, "e2e-spec-185-audit", "Spec 185 Audit Stream")
	sc := Given(d).
		TriagedIssue("Audit stream coverage", "spec 185 demo source", 2).
		Build()
	issueID := sc.Issues[0]

	const (
		agentID   = "spec185-agent-1"
		sessionID = "spec185-session-uuid"
	)

	// Claim a todo from the queue to obtain a valid zdx_todos.id.
	// zdx_claude_sessions.todo_id FKs to zdx_todos, not zdx_tasks.
	claimedTodo, claimStatus := soloClaimNext(t, d.Slug, agentID)
	if claimStatus != http.StatusOK {
		t.Fatalf("soloClaimNext: want 200, got %d", claimStatus)
	}
	todoID := claimedTodo.ID

	// Register the agent so the audit row's agent_id resolves to a known
	// project agent — this is the precondition IS-600 Phase 1 establishes.
	mustOK(t, apiDo(t, http.MethodPost, "/api/agents/register",
		map[string]any{
			"slug":            d.Slug,
			"id":              agentID,
			"session_id":      sessionID,
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

	// Create the agent session linked to the originating todo (taskID).
	// Attribution flows: events → session.todo_id → task → issue.
	var session struct {
		ID        int64  `json:"id"`
		SessionID string `json:"session_id"`
		Created   bool   `json:"created"`
	}
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/agent/sessions/%s/create?slug=%s", sessionID, d.Slug),
		map[string]any{
			"provider":   "claude",
			"alias":      "spec185-audit",
			"trigger":    "manual",
			"title":      "spec 185 audit run",
			"issue_id":   fmt.Sprintf("IS-%d", issueID),
			"agent_id":   agentID,
			"agent_type": "claude",
			"todo_id":    todoID,
		}, &session))
	if !session.Created {
		t.Fatalf("expected fresh session, got created=false (sid=%q)", session.SessionID)
	}

	// Subscribe to the session WS channel BEFORE streaming events, otherwise
	// the publish can race ahead of the subscriber.
	channel := fmt.Sprintf("project:%s:claude:%s", d.Slug, sessionID)
	var signResp struct {
		Token string `json:"token"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/ws/sign",
		map[string]string{"channel": channel}, &signResp))
	if signResp.Token == "" {
		t.Fatal("ws sign returned empty token")
	}

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	subPayload, _ := json.Marshal(map[string]string{
		"type":  "subscribe",
		"token": signResp.Token,
	})
	if err := conn.Write(ctx, websocket.MessageText, subPayload); err != nil {
		t.Fatalf("ws write subscribe: %v", err)
	}

	// First inbound frame is the subscribed ack.
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read subscribed ack: %v", err)
	}
	var ack struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
	}
	if err := json.Unmarshal(ackData, &ack); err != nil {
		t.Fatalf("unmarshal subscribed ack: %v", err)
	}
	if ack.Type != "subscribed" || ack.Channel != channel {
		t.Fatalf("subscribed ack: type=%q channel=%q want subscribed/%s", ack.Type, ack.Channel, channel)
	}

	// Three audit-event types the spec calls out: tool calls, file edits,
	// shell commands. Each event_type carries a distinguishable payload so
	// we can verify it round-trips through both WS and DB.
	events := []struct {
		seq       int32
		eventType string
		payload   map[string]any
	}{
		{1, "tool_call", map[string]any{"tool": "Bash", "input": "ls -la"}},
		{2, "file_edit", map[string]any{"path": "internal/server/handlers/handlers_agent_sessions.go", "lines_added": 3, "lines_removed": 1}},
		{3, "shell_cmd", map[string]any{"cmd": "go build ./...", "exit_code": 0}},
	}

	// Build NDJSON body — one line per event — and POST it to the audit
	// ingestion endpoint. agent_id is set on every line so each row in
	// zdx_claude_events is attributable.
	var body bytes.Buffer
	for _, ev := range events {
		evJSON, err := json.Marshal(ev.payload)
		if err != nil {
			t.Fatalf("marshal payload for %s: %v", ev.eventType, err)
		}
		line, err := json.Marshal(map[string]any{
			"seq":        ev.seq,
			"event_type": ev.eventType,
			"event_json": json.RawMessage(evJSON),
			"agent_id":   agentID,
			"agent_type": "claude",
		})
		if err != nil {
			t.Fatalf("marshal ndjson line: %v", err)
		}
		body.Write(line)
		body.WriteByte('\n')
	}

	url := fmt.Sprintf("%s/api/dx/agent/sessions/%s/events?slug=%s", srv.URL, sessionID, d.Slug)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("X-Api-Key", srv.AdminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /events: status=%d body=%s", resp.StatusCode, string(raw))
	}
	var ingest struct {
		Persisted int `json:"persisted"`
		Skipped   int `json:"skipped"`
		MaxSeq    int `json:"max_seq"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ingest); err != nil {
		t.Fatalf("decode ingest response: %v", err)
	}
	if ingest.Persisted != len(events) {
		t.Errorf("persisted: want %d, got %d (skipped=%d)", len(events), ingest.Persisted, ingest.Skipped)
	}
	if ingest.MaxSeq != int(events[len(events)-1].seq) {
		t.Errorf("max_seq: want %d, got %d", events[len(events)-1].seq, ingest.MaxSeq)
	}

	// ── Liveness — events must be published on the session WS channel ──
	// Collect agent.event frames keyed by event_type until we have all three.
	wsByType := map[string]json.RawMessage{}
	for len(wsByType) < len(events) {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("ws read (got %d/%d): %v", len(wsByType), len(events), err)
		}
		var msg struct {
			Type    string          `json:"type"`
			Channel string          `json:"channel"`
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal ws frame: %v", err)
		}
		if msg.Type != "message" || msg.Channel != channel {
			continue
		}
		if msg.Event != "agent.event" {
			// claude.event is published in parallel as a legacy mirror — ignore it.
			continue
		}
		var p struct {
			SessionID string          `json:"session_id"`
			SessionPK int64           `json:"session_pk"`
			Seq       int32           `json:"seq"`
			EventType string          `json:"event_type"`
			AgentID   string          `json:"agent_id"`
			EventJSON json.RawMessage `json:"event_json"`
		}
		if err := json.Unmarshal(msg.Payload, &p); err != nil {
			t.Fatalf("unmarshal agent.event payload: %v", err)
		}
		if p.SessionID != sessionID {
			t.Errorf("agent.event session_id: want %q got %q", sessionID, p.SessionID)
		}
		if p.SessionPK != session.ID {
			t.Errorf("agent.event session_pk: want %d got %d", session.ID, p.SessionPK)
		}
		if p.AgentID != agentID {
			t.Errorf("agent.event[%s] agent_id: want %q got %q", p.EventType, agentID, p.AgentID)
		}
		wsByType[p.EventType] = p.EventJSON
	}

	for _, ev := range events {
		got, ok := wsByType[ev.eventType]
		if !ok {
			t.Errorf("ws: missing event_type %q", ev.eventType)
			continue
		}
		var gotPayload map[string]any
		if err := json.Unmarshal(got, &gotPayload); err != nil {
			t.Errorf("ws[%s]: unmarshal payload: %v", ev.eventType, err)
			continue
		}
		for k, v := range ev.payload {
			if !equalJSONScalar(gotPayload[k], v) {
				t.Errorf("ws[%s].%s: want %v got %v", ev.eventType, k, v, gotPayload[k])
			}
		}
	}

	// ── Persistence — rows must be queryable with attribution intact ──
	var listed struct {
		Events []struct {
			Seq       int32           `json:"seq"`
			EventType string          `json:"event_type"`
			EventJSON json.RawMessage `json:"event_json"`
			AgentID   string          `json:"agent_id"`
		} `json:"events"`
		Total int64 `json:"total"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/claude/sessions/%d/events?slug=%s&limit=50", session.ID, d.Slug),
		nil, &listed))
	if int(listed.Total) != len(events) {
		t.Errorf("DB total: want %d, got %d", len(events), listed.Total)
	}
	dbByType := map[string]json.RawMessage{}
	dbAgents := map[string]string{}
	for _, row := range listed.Events {
		dbByType[row.EventType] = row.EventJSON
		dbAgents[row.EventType] = row.AgentID
	}
	for _, ev := range events {
		raw, ok := dbByType[ev.eventType]
		if !ok {
			t.Errorf("DB: missing row for event_type=%q", ev.eventType)
			continue
		}
		if dbAgents[ev.eventType] != agentID {
			t.Errorf("DB[%s] agent_id: want %q got %q", ev.eventType, agentID, dbAgents[ev.eventType])
		}
		var gotPayload map[string]any
		if err := json.Unmarshal(raw, &gotPayload); err != nil {
			t.Errorf("DB[%s]: unmarshal payload: %v", ev.eventType, err)
			continue
		}
		for k, v := range ev.payload {
			if !equalJSONScalar(gotPayload[k], v) {
				t.Errorf("DB[%s].%s: want %v got %v", ev.eventType, k, v, gotPayload[k])
			}
		}
	}

	// ── Task attribution — the session is linked to the originating todo ──
	var sess struct {
		ID     int64 `json:"id"`
		TodoID int32 `json:"todo_id"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/claude/sessions/%d?slug=%s", session.ID, d.Slug),
		nil, &sess))
	if sess.TodoID != todoID {
		t.Errorf("session.todo_id: want %d, got %d", todoID, sess.TodoID)
	}
}

// equalJSONScalar compares an actual decoded JSON scalar (string, float64,
// bool) against a Go literal used in the test's expected payload (string,
// int, float64, bool). JSON numerics decode as float64 even when the source
// was an integer, so we coerce both sides for comparison.
func equalJSONScalar(got, want any) bool {
	switch w := want.(type) {
	case int:
		if g, ok := got.(float64); ok {
			return g == float64(w)
		}
	case float64:
		if g, ok := got.(float64); ok {
			return g == w
		}
	case string:
		if g, ok := got.(string); ok {
			return g == w
		}
	case bool:
		if g, ok := got.(bool); ok {
			return g == w
		}
	}
	return false
}
