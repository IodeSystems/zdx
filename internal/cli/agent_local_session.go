package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// sessionLog writes Claude-compatible JSONL envelopes for a local-LLM session
// and (optionally) streams the file to /api/dx/claude/sessions/ingest/stream so
// events render in the sessions/agents UI in real time.
type sessionLog struct {
	sid      string
	path     string
	cwd      string
	f        *os.File
	mu       sync.Mutex
	parent   string
	streamCh chan struct{}
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
	if s.streamCh != nil {
		close(s.streamCh)
		s.streamCh = nil
	}
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

// StartStream starts tailing the JSONL file and POSTing to
// /api/dx/claude/sessions/ingest/stream. Returns a cancel func.
func (s *sessionLog) StartStream(rc remoteConfig, issueID, alias string) func() {
	if !rc.valid() {
		return func() {}
	}
	done := make(chan struct{})
	s.streamCh = done

	go func() {
		for i := 0; i < 60; i++ {
			if fi, err := os.Stat(s.path); err == nil && fi.Size() > 0 {
				break
			}
			select {
			case <-done:
				return
			case <-time.After(200 * time.Millisecond):
			}
		}

		f, err := os.Open(s.path)
		if err != nil {
			return
		}
		defer f.Close()

		u := fmt.Sprintf("%s/api/dx/claude/sessions/ingest/stream?slug=%s&session_id=%s&issue_id=%s&alias=%s&agent_id=%s&agent_type=local",
			rc.url,
			url.QueryEscape(rc.slug),
			url.QueryEscape(s.sid),
			url.QueryEscape(issueID),
			url.QueryEscape(alias),
			url.QueryEscape("local"))

		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			reader := bufio.NewReader(f)
			for {
				select {
				case <-done:
					return
				default:
				}
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					_, _ = pw.Write(line)
				}
				if err != nil {
					time.Sleep(200 * time.Millisecond)
					continue
				}
			}
		}()

		req, _ := http.NewRequest("POST", u, pr)
		req.Header.Set("X-Api-Key", rc.key)
		req.Header.Set("Content-Type", "application/x-ndjson")
		client := &http.Client{Timeout: 0}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
		s.streamCh = nil
	}
}
