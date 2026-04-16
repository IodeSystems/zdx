package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// sessionLog writes Claude-compatible JSONL envelopes for a local-LLM session
// to .zdx/agent/local/<sid>.jsonl. The shared RunLifecycle runner is
// responsible for tailing that file and streaming events to the server;
// this type only handles the write side.
type sessionLog struct {
	sid    string
	path   string
	cwd    string
	f      *os.File
	mu     sync.Mutex
	parent string
}

func newSessionLog(sid, issueID, cwd string) (*sessionLog, error) {
	dir := filepath.Join(".zdx", "agent", "local")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, sid+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &sessionLog{sid: sid, path: path, cwd: cwd, f: f}, nil
}

func (s *sessionLog) Close() error {
	return s.f.Close()
}

func (s *sessionLog) writeEvent(ev map[string]any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	u := uuid.New().String()
	ev["uuid"] = u
	if s.parent != "" {
		ev["parentUuid"] = s.parent
	} else {
		ev["parentUuid"] = nil
	}
	ev["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
	ev["sessionId"] = s.sid
	ev["cwd"] = s.cwd
	ev["isSidechain"] = false
	ev["userType"] = "external"
	ev["entrypoint"] = "dx-agent-local"
	s.parent = u
	buf, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if _, err := s.f.Write(append(buf, '\n')); err != nil {
		return err
	}
	return s.f.Sync()
}

// UserText records a user-role text message.
func (s *sessionLog) UserText(text string) error {
	return s.writeEvent(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": text,
		},
	})
}

// AssistantText records an assistant-role text message (no tool calls).
func (s *sessionLog) AssistantText(text string, usage map[string]any) error {
	content := []any{map[string]any{"type": "text", "text": text}}
	return s.writeEvent(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":    "assistant",
			"content": content,
			"usage":   usage,
		},
	})
}

// AssistantWithToolUse records an assistant turn that included tool calls.
// toolUses is a list of {id,name,input} maps matching Claude's shape.
func (s *sessionLog) AssistantWithToolUse(text string, toolUses []map[string]any, usage map[string]any) error {
	var content []any
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	for _, tu := range toolUses {
		content = append(content, map[string]any{
			"type":  "tool_use",
			"id":    tu["id"],
			"name":  tu["name"],
			"input": tu["input"],
		})
	}
	return s.writeEvent(map[string]any{
		"type": "assistant",
		"message": map[string]any{
			"role":    "assistant",
			"content": content,
			"usage":   usage,
		},
	})
}

// ToolResult records a tool_result returned to the model (wrapped as a user turn).
func (s *sessionLog) ToolResult(toolUseID, text string, isError bool) error {
	return s.writeEvent(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role": "user",
			"content": []any{map[string]any{
				"tool_use_id": toolUseID,
				"type":        "tool_result",
				"content":     text,
				"is_error":    isError,
			}},
		},
	})
}

// AITitle records a synthetic title event (type=ai-title) used by the UI to
// label the session.
func (s *sessionLog) AITitle(title string) error {
	return s.writeEvent(map[string]any{
		"type":  "ai-title",
		"title": title,
		"message": map[string]any{
			"role":    "assistant",
			"content": title,
		},
	})
}
