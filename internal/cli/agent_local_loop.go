package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/llm"
)

// chatSession orchestrates a single run of the local-LLM agent: it feeds a
// seed prompt, iterates assistant turns dispatching tool_calls until the
// model returns without further tool invocations or max-turns is hit.
type chatSession struct {
	client     *llm.Client
	dispatcher *localDispatcher
	log        *sessionLog
	tools      []llm.ToolDef
	maxTurns   int
	timeout    time.Duration
}

func newChatSession(llmCfg config.LLMLocal, disp *localDispatcher, log *sessionLog, maxTurns int) *chatSession {
	client := llm.New(llm.Config{
		Type:   "openai",
		URL:    llmCfg.BaseURL,
		Model:  llmCfg.Model,
		APIKey: llmCfg.APIKey,
	})
	return &chatSession{
		client:     client,
		dispatcher: disp,
		log:        log,
		tools:      disp.OpenAIFunctions(),
		maxTurns:   maxTurns,
		timeout:    time.Duration(llmCfg.TimeoutSeconds) * time.Second,
	}
}

// Run feeds system + user prompts and loops until the model stops emitting
// tool_calls or maxTurns is exceeded. Every message, tool_use, and tool_result
// is recorded to the session log.
func (cs *chatSession) Run(ctx context.Context, system, user string) error {
	_ = cs.log.AITitle(truncate(user, 80))
	_ = cs.log.UserText(user)

	msgs := []llm.ChatMsg{}
	if system != "" {
		msgs = append(msgs, llm.ChatMsg{Role: "system", Content: system})
	}
	msgs = append(msgs, llm.ChatMsg{Role: "user", Content: user})

	for turn := 0; turn < cs.maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		resp, err := cs.client.ChatCompletion(ctx, &llm.ChatRequest{
			Messages:   msgs,
			Tools:      cs.tools,
			ToolChoice: "auto",
			Timeout:    cs.timeout,
		})
		if err != nil {
			return fmt.Errorf("turn %d: %w", turn, err)
		}

		usage := map[string]any{
			"input_tokens":  resp.Usage.PromptTokens,
			"output_tokens": resp.Usage.CompletionTokens,
		}

		if len(resp.ToolCalls) == 0 {
			_ = cs.log.AssistantText(resp.Content, usage)
			if strings.TrimSpace(resp.Content) != "" {
				fmt.Println(resp.Content)
			}
			return nil
		}

		toolUses := make([]map[string]any, 0, len(resp.ToolCalls))
		for _, tc := range resp.ToolCalls {
			var input any
			if tc.Function.Arguments != "" {
				_ = json.Unmarshal([]byte(tc.Function.Arguments), &input)
			}
			if input == nil {
				input = map[string]any{}
			}
			toolUses = append(toolUses, map[string]any{
				"id":    tc.ID,
				"name":  tc.Function.Name,
				"input": input,
			})
		}
		_ = cs.log.AssistantWithToolUse(resp.Content, toolUses, usage)

		msgs = append(msgs, llm.ChatMsg{
			Role:      "assistant",
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		for _, tc := range resp.ToolCalls {
			fmt.Printf("→ tool: %s(%s)\n", tc.Function.Name, truncate(tc.Function.Arguments, 200))
			toolCtx, cancel := context.WithTimeout(ctx, cs.timeout)
			result, isErr, dispErr := cs.dispatcher.Dispatch(toolCtx, tc.Function.Name, tc.Function.Arguments)
			cancel()
			if dispErr != nil {
				result = dispErr.Error()
				isErr = true
			}
			_ = cs.log.ToolResult(tc.ID, result, isErr)
			msgs = append(msgs, llm.ChatMsg{
				Role:       "tool",
				ToolCallID: tc.ID,
				Name:       tc.Function.Name,
				Content:    result,
			})
		}
	}

	_ = cs.log.AssistantText(fmt.Sprintf("(stopped: max-turns=%d exceeded)", cs.maxTurns), nil)
	return fmt.Errorf("max turns (%d) exceeded", cs.maxTurns)
}
