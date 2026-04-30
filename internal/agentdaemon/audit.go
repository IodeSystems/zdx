package agentdaemon

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"time"
)

// AgentConn abstracts the daemon's persistent WS connection for audit event emission.
type AgentConn interface {
	SendMsg(ctx context.Context, data []byte) error
}

// tapClaudeOutput wraps r in a reader that intercepts Claude JSONL and emits
// an audit event over conn for each tool_use block. All bytes pass through
// unchanged. Each send runs in a goroutine so the pipe never blocks the
// subprocess stdout reader.
func tapClaudeOutput(conn AgentConn, agentID, sessionID, turnID string, r io.Reader) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		sc := bufio.NewScanner(io.TeeReader(r, pw))
		sc.Buffer(make([]byte, 1<<20), 1<<20) // claude payloads can be large
		for sc.Scan() {
			line := sc.Bytes()
			cp := make([]byte, len(line))
			copy(cp, line)
			go emitAuditEventsFromLine(conn, agentID, sessionID, turnID, cp)
		}
		if err := sc.Err(); err != nil {
			pw.CloseWithError(err)
		}
	}()
	return pr
}

func emitAuditEventsFromLine(conn AgentConn, agentID, sessionID, turnID string, line []byte) {
	var evt struct {
		Type    string `json:"type"`
		Message struct {
			Content []struct {
				Type  string          `json:"type"`
				Name  string          `json:"name"`
				Input json.RawMessage `json:"input"`
			} `json:"content"`
		} `json:"message"`
	}
	if json.Unmarshal(line, &evt) != nil || evt.Type != "assistant" {
		return
	}
	for _, c := range evt.Message.Content {
		if c.Type != "tool_use" {
			continue
		}
		eventType := ClassifyToolUse(c.Name)
		payload, err := json.Marshal(map[string]any{
			"tool":  c.Name,
			"input": c.Input,
		})
		if err != nil {
			continue
		}
		msg, err := json.Marshal(map[string]any{
			"type":       "agent.audit_event",
			"agent_id":   agentID,
			"session_id": sessionID,
			"turn_id":    turnID,
			"event_type": eventType,
			"payload":    json.RawMessage(payload),
		})
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := conn.SendMsg(ctx, msg); err != nil {
			log.Printf("audit: send %s: %v", eventType, err)
		}
		cancel()
	}
}

// ClassifyToolUse returns the audit event_type for a Claude tool name.
func ClassifyToolUse(name string) string {
	switch name {
	case "Write", "Edit", "MultiEdit", "NotebookEdit":
		return "file_edit"
	case "Bash":
		return "shell_cmd"
	default:
		return "tool_call"
	}
}
