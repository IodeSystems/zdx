package handlers

import (
	"context"
	"encoding/json"
	"testing"
)

// recordingBroker captures every Publish* call so tests can assert that
// audit-event dispatch fired on the right channel with the right payload.
type recordingBroker struct {
	auditCalls []auditCall
}

type auditCall struct {
	AgentID string
	Payload any
}

func (r *recordingBroker) Publish(channel, eventType string, payload any) {}
func (r *recordingBroker) PublishIssue(slug, issueID, eventType string, payload any) {
}
func (r *recordingBroker) PublishTask(slug, taskID, eventType string, payload any) {
}
func (r *recordingBroker) PublishClaudeEvent(slug, sessionID, eventType string, payload any) {
}
func (r *recordingBroker) PublishClaudeSessionLifecycle(slug, sessionID, eventType string, payload any) {
}
func (r *recordingBroker) PublishAgentSessionLifecycle(slug, sessionID, eventType string, payload any) {
}
func (r *recordingBroker) PublishAuditEvent(agentID string, payload any) {
	r.auditCalls = append(r.auditCalls, auditCall{AgentID: agentID, Payload: payload})
}

func TestDispatchAgentMessage_AuditEventPublishes(t *testing.T) {
	br := &recordingBroker{}
	h := &Handler{Deps: &Deps{Broker: br}}

	// Q is nil — the audit-channel publish must fire regardless of DB
	// availability so operators see the live signal even before /create
	// has been called.
	frame := map[string]any{
		"type":       "agent.audit_event",
		"agent_id":   "agent-1",
		"session_id": "sess-abc",
		"turn_id":    "turn-001",
		"event_type": "tool_call",
		"payload":    map[string]any{"tool": "Bash", "input": "ls"},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		t.Fatalf("marshal frame: %v", err)
	}

	h.dispatchAgentMessage(context.Background(), "agent-1", data)

	if len(br.auditCalls) != 1 {
		t.Fatalf("audit publishes: want 1, got %d", len(br.auditCalls))
	}
	call := br.auditCalls[0]
	if call.AgentID != "agent-1" {
		t.Errorf("audit agent_id: want %q got %q", "agent-1", call.AgentID)
	}
	pm, ok := call.Payload.(map[string]any)
	if !ok {
		t.Fatalf("payload type: want map[string]any got %T", call.Payload)
	}
	if pm["session_id"] != "sess-abc" {
		t.Errorf("session_id: %v", pm["session_id"])
	}
	if pm["turn_id"] != "turn-001" {
		t.Errorf("turn_id: %v", pm["turn_id"])
	}
	if pm["event_type"] != "tool_call" {
		t.Errorf("event_type: %v", pm["event_type"])
	}
	inner, ok := pm["payload"].(map[string]any)
	if !ok {
		t.Fatalf("payload.payload type: %T", pm["payload"])
	}
	if inner["tool"] != "Bash" || inner["input"] != "ls" {
		t.Errorf("inner payload: %v", inner)
	}
}

func TestDispatchAgentMessage_FillsAgentIDFromConnection(t *testing.T) {
	br := &recordingBroker{}
	h := &Handler{Deps: &Deps{Broker: br}}

	frame := map[string]any{
		"type":       "agent.audit_event",
		"session_id": "sess-xyz",
		"event_type": "shell_cmd",
		"payload":    map[string]any{"cmd": "go test"},
	}
	data, _ := json.Marshal(frame)

	h.dispatchAgentMessage(context.Background(), "conn-agent", data)

	if len(br.auditCalls) != 1 {
		t.Fatalf("audit publishes: want 1, got %d", len(br.auditCalls))
	}
	if br.auditCalls[0].AgentID != "conn-agent" {
		t.Errorf("audit agent_id: want %q got %q", "conn-agent", br.auditCalls[0].AgentID)
	}
}

func TestDispatchAgentMessage_UnknownTypeIgnored(t *testing.T) {
	br := &recordingBroker{}
	h := &Handler{Deps: &Deps{Broker: br}}

	data, _ := json.Marshal(map[string]any{"type": "agent.something_new"})
	h.dispatchAgentMessage(context.Background(), "agent-1", data)

	if len(br.auditCalls) != 0 {
		t.Fatalf("unknown type fired %d audit publishes; want 0", len(br.auditCalls))
	}
}

func TestDispatchAgentMessage_MalformedDropped(t *testing.T) {
	br := &recordingBroker{}
	h := &Handler{Deps: &Deps{Broker: br}}

	h.dispatchAgentMessage(context.Background(), "agent-1", []byte("not json"))

	if len(br.auditCalls) != 0 {
		t.Fatalf("malformed frame fired %d audit publishes; want 0", len(br.auditCalls))
	}
}

func TestAuthorizeAgentRegister(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		scope          []string
		projectSlug    string
		wantReject     bool
		wantDeprecated bool
	}{
		{name: "global with admin unscoped — allowed", role: "admin", scope: nil, projectSlug: "", wantReject: false, wantDeprecated: false},
		{name: "global with admin scoped — deprecated", role: "admin", scope: []string{"foo"}, projectSlug: "", wantReject: false, wantDeprecated: true},
		{name: "global with non-admin unscoped — deprecated", role: "user", scope: nil, projectSlug: "", wantReject: false, wantDeprecated: true},
		{name: "global with non-admin scoped — deprecated", role: "user", scope: []string{"foo"}, projectSlug: "", wantReject: false, wantDeprecated: true},
		{name: "project-scoped registering matching slug — allowed", role: "user", scope: []string{"foo"}, projectSlug: "foo", wantReject: false, wantDeprecated: false},
		{name: "project-scoped registering different slug — rejected", role: "user", scope: []string{"foo"}, projectSlug: "bar", wantReject: true, wantDeprecated: false},
		{name: "unscoped admin registering project — allowed", role: "admin", scope: nil, projectSlug: "foo", wantReject: false, wantDeprecated: false},
		{name: "unscoped non-admin registering project — allowed", role: "user", scope: nil, projectSlug: "foo", wantReject: false, wantDeprecated: false},
		{name: "multi-scope key registering an in-scope slug — allowed", role: "user", scope: []string{"foo", "bar"}, projectSlug: "bar", wantReject: false, wantDeprecated: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			reason, deprecation := authorizeAgentRegister(tc.role, tc.scope, tc.projectSlug, "agent-x")
			rejected := reason != ""
			deprecated := deprecation != ""
			if rejected != tc.wantReject {
				t.Errorf("reject: want %v got %v (reason=%q)", tc.wantReject, rejected, reason)
			}
			if deprecated != tc.wantDeprecated {
				t.Errorf("deprecation: want %v got %v (msg=%q)", tc.wantDeprecated, deprecated, deprecation)
			}
		})
	}
}
