package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
