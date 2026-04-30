package agentdaemon

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// stubConn records messages sent via SendMsg.
type stubConn struct {
	mu   sync.Mutex
	msgs [][]byte
}

func (s *stubConn) SendMsg(_ context.Context, data []byte) error {
	s.mu.Lock()
	s.msgs = append(s.msgs, append([]byte(nil), data...))
	s.mu.Unlock()
	return nil
}

func (s *stubConn) received() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([][]byte, len(s.msgs))
	copy(cp, s.msgs)
	return cp
}

func TestClassifyToolUse(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"Write", "file_edit"},
		{"Edit", "file_edit"},
		{"MultiEdit", "file_edit"},
		{"NotebookEdit", "file_edit"},
		{"Bash", "shell_cmd"},
		{"Read", "tool_call"},
		{"Grep", "tool_call"},
		{"Agent", "tool_call"},
		{"", "tool_call"},
	}
	for _, tc := range cases {
		got := ClassifyToolUse(tc.name)
		if got != tc.want {
			t.Errorf("ClassifyToolUse(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestTapClaudeOutput_EmitsAuditEvents(t *testing.T) {
	conn := &stubConn{}
	const (
		agentID   = "agent-1"
		sessionID = "sess-abc"
		turnID    = "turn-1"
	)

	// Craft a JSONL assistant event containing two tool_use blocks.
	assistantLine := mustMarshal(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"content": []any{
				map[string]any{
					"type":  "tool_use",
					"name":  "Bash",
					"id":    "id-1",
					"input": map[string]any{"command": "go build ./..."},
				},
				map[string]any{
					"type":  "tool_use",
					"name":  "Write",
					"id":    "id-2",
					"input": map[string]any{"file_path": "foo.go"},
				},
			},
		},
	})

	// Non-tool line that should produce no audit events.
	textLine := mustMarshal(map[string]any{"type": "system", "subtype": "init"})

	input := strings.NewReader(string(assistantLine) + "\n" + string(textLine) + "\n")
	reader := tapClaudeOutput(conn, agentID, sessionID, turnID, input)

	// Consume the reader (pipe must be drained for the goroutine to finish).
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	// Pass-through: output should equal input.
	wantOut := string(assistantLine) + "\n" + string(textLine) + "\n"
	if string(out) != wantOut {
		t.Errorf("passthrough mismatch: got %q want %q", string(out), wantOut)
	}

	// Wait for goroutines to deliver their audit messages.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(conn.received()) >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	msgs := conn.received()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 audit events, got %d", len(msgs))
	}

	// Decode each message and verify fields.
	byType := map[string]map[string]any{}
	for _, raw := range msgs {
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("unmarshal audit msg: %v", err)
		}
		typ, _ := m["event_type"].(string)
		byType[typ] = m
	}

	for _, tc := range []struct {
		eventType string
		wantTool  string
	}{
		{"shell_cmd", "Bash"},
		{"file_edit", "Write"},
	} {
		m, ok := byType[tc.eventType]
		if !ok {
			t.Errorf("missing audit event for event_type=%q", tc.eventType)
			continue
		}
		if m["type"] != "agent.audit_event" {
			t.Errorf("[%s] type: got %q want agent.audit_event", tc.eventType, m["type"])
		}
		if m["agent_id"] != agentID {
			t.Errorf("[%s] agent_id: got %q want %q", tc.eventType, m["agent_id"], agentID)
		}
		if m["session_id"] != sessionID {
			t.Errorf("[%s] session_id: got %q want %q", tc.eventType, m["session_id"], sessionID)
		}
		if m["turn_id"] != turnID {
			t.Errorf("[%s] turn_id: got %q want %q", tc.eventType, m["turn_id"], turnID)
		}
		// payload is nested JSON; re-decode
		payloadRaw, _ := json.Marshal(m["payload"])
		var payload map[string]any
		if err := json.Unmarshal(payloadRaw, &payload); err != nil {
			t.Fatalf("[%s] payload unmarshal: %v", tc.eventType, err)
		}
		if payload["tool"] != tc.wantTool {
			t.Errorf("[%s] payload.tool: got %q want %q", tc.eventType, payload["tool"], tc.wantTool)
		}
	}
}

func TestTapClaudeOutput_NonAssistantLines_NoEvents(t *testing.T) {
	conn := &stubConn{}
	lines := strings.Join([]string{
		mustMarshalStr(map[string]any{"type": "system", "subtype": "init"}),
		mustMarshalStr(map[string]any{"type": "user", "message": map[string]any{"role": "user"}}),
		`not json at all`,
	}, "\n") + "\n"

	reader := tapClaudeOutput(conn, "ag", "sid", "tid", strings.NewReader(lines))
	if _, err := io.ReadAll(reader); err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // let any goroutines run
	if len(conn.received()) != 0 {
		t.Errorf("expected 0 audit events, got %d", len(conn.received()))
	}
}

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

func mustMarshalStr(v any) string { return string(mustMarshal(v)) }
