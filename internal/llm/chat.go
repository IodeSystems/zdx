package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ToolDef describes a tool available to the model in OpenAI chat/completions
// function-calling format.
type ToolDef struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ToolCall is a tool invocation emitted by the model.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatMsg is an OpenAI chat/completions message with optional tool-calling fields.
type ChatMsg struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	Name       string     `json:"name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

// ChatRequest is the input to ChatCompletion.
type ChatRequest struct {
	Messages   []ChatMsg
	Tools      []ToolDef
	ToolChoice string
	MaxTokens  int
	Timeout    time.Duration
}

// ChatResponse is the model's reply. Content is text; ToolCalls are structured
// tool invocations the caller must dispatch and feed back as tool messages.
type ChatResponse struct {
	Content      string
	ToolCalls    []ToolCall
	FinishReason string
	Usage        ChatUsage
}

type ChatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletion issues an OpenAI-compatible chat/completions request with
// optional tool definitions and returns the structured response.
func (c *Client) ChatCompletion(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	u := strings.TrimRight(c.cfg.URL, "/") + "/v1/chat/completions"

	body := map[string]any{
		"model":    c.cfg.Model,
		"messages": req.Messages,
	}
	if len(req.Tools) > 0 {
		body["tools"] = req.Tools
		if req.ToolChoice != "" {
			body["tool_choice"] = req.ToolChoice
		} else {
			body["tool_choice"] = "auto"
		}
	}
	if req.MaxTokens > 0 {
		body["max_tokens"] = req.MaxTokens
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	timeout := req.Timeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(cctx, http.MethodPost, u, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	client := &http.Client{Timeout: timeout + 5*time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("llm: chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody struct {
			Error struct{ Message string } `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&errBody)
		return nil, fmt.Errorf("llm: chat %s: %s", resp.Status, errBody.Error.Message)
	}

	var parsed struct {
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Role      string     `json:"role"`
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
		Usage ChatUsage `json:"usage"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("llm: chat decode: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return nil, fmt.Errorf("llm: chat returned no choices")
	}
	ch := parsed.Choices[0]
	return &ChatResponse{
		Content:      ch.Message.Content,
		ToolCalls:    ch.Message.ToolCalls,
		FinishReason: ch.FinishReason,
		Usage:        parsed.Usage,
	}, nil
}

// StreamChatCompletion POSTs to /v1/chat/completions with stream:true, parses
// SSE chunks, invokes onDelta for each content delta, and returns the
// accumulated assistant content. Provider-side session continuity is achieved
// by sending the full message history; OpenAI-compatible endpoints do not
// expose a session/thread identifier on streamed responses.
func (c *Client) StreamChatCompletion(ctx context.Context, messages []ChatMessage, onDelta func(string)) (string, error) {
	u := strings.TrimRight(c.cfg.URL, "/") + "/v1/chat/completions"

	body, err := json.Marshal(map[string]any{
		"model":    c.cfg.Model,
		"messages": messages,
		"stream":   true,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	client := &http.Client{Timeout: 0}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm: stream %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}

	var acc strings.Builder
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimPrefix(line, "data: ")
				if payload == "[DONE]" {
					return acc.String(), nil
				}
				var chunk struct {
					Choices []struct {
						Delta struct {
							Content string `json:"content"`
						} `json:"delta"`
					} `json:"choices"`
				}
				if jerr := json.Unmarshal([]byte(payload), &chunk); jerr != nil {
					continue
				}
				for _, ch := range chunk.Choices {
					if ch.Delta.Content != "" {
						acc.WriteString(ch.Delta.Content)
						if onDelta != nil {
							onDelta(ch.Delta.Content)
						}
					}
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return acc.String(), nil
			}
			return acc.String(), fmt.Errorf("llm: stream read: %w", err)
		}
	}
}
