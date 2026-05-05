package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/mcpcmd"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/llm"
)

// ── OpenCode AgentAdapter ─────────────────────────────────────────────────

func init() {
	RegisterProvider("opencode", func(opts ProviderOpts) (AgentProvider, error) {
		llmCfg := opts.LLMLocal
		if opts.Model != "" {
			llmCfg.Model = opts.Model
		}
		// Pull endpoint + api_key + (when --model unset) model from the
		// server's admin/llm-configs for the chosen complexity tier. This
		// lets opencode work against a remote LLM without local llm_local
		// settings — matches the legacy RunE behavior.
		if opts.Complexity != "" {
			if serverCfg := resolveLLMConfigFromServer(opts.RC, opts.Complexity); serverCfg.BaseURL != "" {
				llmCfg.BaseURL = serverCfg.BaseURL
				if serverCfg.APIKey != "" {
					llmCfg.APIKey = serverCfg.APIKey
				}
				if opts.Model == "" {
					llmCfg.Model = serverCfg.Model
				}
			}
		}
		return &opencodeAdapter{
			rc:         opts.RC,
			llmCfg:     llmCfg,
			maxTurns:   opts.MaxTurns, // 0 = unlimited; matches opencode CLI default
			seedPrompt: opts.SeedPrompt,
		}, nil
	})
}

// opencodeAdapter implements AgentAdapter for the OpenCode agent loop.
// It drives an in-process chat-completions loop with MCP tools, writing
// Claude-compatible JSONL to .zdx/agent/opencode/<sid>.jsonl. Session
// state is persisted as JSON in .zdx/state/opencode/<sid>.json for resume.
type opencodeAdapter struct {
	rc         remoteConfig // populated by callers that need ResolveModel; safe to leave zero otherwise
	llmCfg     config.LLMLocal
	maxTurns   int
	seedPrompt string

	doneCh chan struct{}
	runErr error

	toolNames map[string]string
}

func (a *opencodeAdapter) Provider() string { return "opencode" }

// ResolveModel maps a complexity tier to a concrete model name by walking the
// server's admin LLM config in priority order. Falls back to the adapter's
// llmCfg.Model when the server is unreachable or no slot matches.
func (a *opencodeAdapter) ResolveModel(ctx context.Context, complexity string) (string, error) {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return "", err
	}
	resolved := applyComplexityModel(ctx, a.llmCfg, tier)
	return resolved.Model, nil
}

func (a *opencodeAdapter) Start(ctx context.Context, sid, issueID, alias string) (string, error) {
	if alias != "" {
		os.Setenv("DX_AUTHOR_ALIAS", alias)
	}
	root, err := cli.GitRepoRoot()
	if err != nil {
		return "", fmt.Errorf("dx agent opencode must run inside a git repo: %w", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "dx-agent-opencode",
		Version: "0.1.0",
	}, nil)
	mcpcmd.RegisterFSTools(srv, root)
	mcpcmd.RegisterShellTools(srv, root)

	dispCtx, dispCancel := context.WithCancel(ctx)
	disp, err := newLocalDispatcher(dispCtx, srv)
	if err != nil {
		dispCancel()
		return "", err
	}

	sessLog, err := newOpenCodeSessionLog(sid, issueID, root)
	if err != nil {
		disp.Close()
		dispCancel()
		return "", err
	}

	saveOpenCodeState(sid, issueID, alias, a.llmCfg.Model, root)

	system := opencodeSystemPrompt(alias, issueID)
	user := a.seedPrompt
	if user == "" {
		if issueID != "" {
			user = fmt.Sprintf("Work the vertical for %s. Use run_bash to invoke `dx` CLI commands (issue show, comment add, todo dev start/done, ...) for project state, and filesystem tools to implement code. Stop when the issue is closed.", issueID)
		} else {
			user = "Use run_bash to call `dx todo solo` to pick the next item, then work it. Stop when idle."
		}
	}

	fmt.Printf("── session %s  issue=%s  model=%s ──\n", sid, issueID, a.llmCfg.Model)

	a.doneCh = make(chan struct{})
	go func() {
		defer close(a.doneCh)
		defer disp.Close()
		defer dispCancel()
		defer sessLog.Close()

		cs := newOpenCodeChatSession(a.llmCfg, disp, sessLog, a.maxTurns)
		a.runErr = cs.Run(ctx, system, user)
	}()

	return sessLog.path, nil
}

func (a *opencodeAdapter) Wait() (int, error) {
	<-a.doneCh
	if a.runErr != nil {
		return 1, a.runErr
	}
	return 0, nil
}

func (a *opencodeAdapter) SubagentDir(_ string) string { return "" }

func (a *opencodeAdapter) ParseLine(line []byte, agentID string) (AgentEvent, error) {
	return (&claudeAdapter{}).ParseLine(line, agentID)
}

func (a *opencodeAdapter) RenderEvent(eventJSON []byte) string {
	if a.toolNames == nil {
		a.toolNames = map[string]string{}
	}
	return renderSessionEvent(eventJSON, a.toolNames)
}

// ── Session log ────────────────────────────────────────────────────────────

type opencodeSessionLog struct {
	sid    string
	path   string
	cwd    string
	f      *os.File
	mu     sync.Mutex
	parent string
}

func newOpenCodeSessionLog(sid, issueID, cwd string) (*opencodeSessionLog, error) {
	dir := ".zdx/agent/opencode"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, sid+".jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &opencodeSessionLog{sid: sid, path: path, cwd: cwd, f: f}, nil
}

func (s *opencodeSessionLog) Close() error {
	return s.f.Close()
}

func (s *opencodeSessionLog) writeEvent(ev map[string]any) error {
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
	ev["entrypoint"] = "dx-agent-opencode"
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

func (s *opencodeSessionLog) UserText(text string) error {
	return s.writeEvent(map[string]any{
		"type": "user",
		"message": map[string]any{
			"role":    "user",
			"content": text,
		},
	})
}

func (s *opencodeSessionLog) AssistantText(text string, usage map[string]any) error {
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

func (s *opencodeSessionLog) AssistantWithToolUse(text string, toolUses []map[string]any, usage map[string]any) error {
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

func (s *opencodeSessionLog) ToolResult(toolUseID, text string, isError bool) error {
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

func (s *opencodeSessionLog) AITitle(title string) error {
	return s.writeEvent(map[string]any{
		"type":  "ai-title",
		"title": title,
		"message": map[string]any{
			"role":    "assistant",
			"content": title,
		},
	})
}

// ── Chat session loop ─────────────────────────────────────────────────────

type opencodeChatSession struct {
	client     *llm.Client
	dispatcher *localDispatcher
	log        *opencodeSessionLog
	tools      []llm.ToolDef
	maxTurns   int
	timeout    time.Duration
}

func newOpenCodeChatSession(llmCfg config.LLMLocal, disp *localDispatcher, log *opencodeSessionLog, maxTurns int) *opencodeChatSession {
	client := llm.New(llm.Config{
		Type:   "openai",
		URL:    llmCfg.BaseURL,
		Model:  llmCfg.Model,
		APIKey: llmCfg.APIKey,
	})
	return &opencodeChatSession{
		client:     client,
		dispatcher: disp,
		log:        log,
		tools:      disp.OpenAIFunctions(),
		maxTurns:   maxTurns,
		timeout:    time.Duration(llmCfg.TimeoutSeconds) * time.Second,
	}
}

func (cs *opencodeChatSession) Run(ctx context.Context, system, user string) error {
	_ = cs.log.AITitle(cli.Truncate(user, 80))
	_ = cs.log.UserText(user)

	msgs := []llm.ChatMsg{}
	if system != "" {
		msgs = append(msgs, llm.ChatMsg{Role: "system", Content: system})
	}
	msgs = append(msgs, llm.ChatMsg{Role: "user", Content: user})

	for turn := 0; cs.maxTurns == 0 || turn < cs.maxTurns; turn++ {
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
			fmt.Printf("→ tool: %s(%s)\n", tc.Function.Name, cli.Truncate(tc.Function.Arguments, 200))
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

// ── System prompt ─────────────────────────────────────────────────────────

func opencodeSystemPrompt(alias, issueID string) string {
	var b strings.Builder
	b.WriteString("You are dx agent opencode, an autonomous developer operating inside a git repo.\n\n")
	b.WriteString("Available tool categories:\n")
	b.WriteString("  - filesystem tools: read_file, write_file, edit_file, list_dir, glob, grep.\n")
	b.WriteString("  - shell: run_bash. Invoke `dx` CLI for project state — e.g. `dx issue show IS-N`, `dx comment add`, `dx todo dev start/done`, `dx feature show`, `dx pattern search`, `dx question add`. Run `dx --help` for the full tree.\n\n")
	b.WriteString("Operating rules:\n")
	b.WriteString("  - Prefer `dx` CLI calls for project state (never re-derive from the filesystem alone).\n")
	b.WriteString("  - Edit minimally; read surrounding context before writing.\n")
	b.WriteString("  - After code edits, verify with run_bash (go build, tests, linters as appropriate).\n")
	b.WriteString("  - When the vertical is done, run `dx issue close` to finish.\n")
	b.WriteString("  - Stop emitting tool_calls when you are finished; a final text reply ends the session.\n\n")
	if alias != "" {
		b.WriteString("Your agent alias is: " + alias + ".\n")
	}
	if issueID != "" {
		b.WriteString("Current issue: " + issueID + ".\n")
	}
	return b.String()
}

// ── Session state persistence (JSON input/output) ─────────────────────────

type opencodeSessionState struct {
	Sid       string `json:"sid"`
	IssueID   string `json:"issue_id"`
	Alias     string `json:"alias"`
	Model     string `json:"model"`
	RepoRoot  string `json:"repo_root"`
	StartedAt string `json:"started_at"`
}

func saveOpenCodeState(sid, issueID, alias, model, repoRoot string) {
	dir := ".zdx/state/opencode"
	_ = os.MkdirAll(dir, 0o755)
	st := opencodeSessionState{
		Sid:       sid,
		IssueID:   issueID,
		Alias:     alias,
		Model:     model,
		RepoRoot:  repoRoot,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(dir, sid+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

func loadOpenCodeState(sid string) *opencodeSessionState {
	path := filepath.Join(".zdx", "state", "opencode", sid+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var st opencodeSessionState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil
	}
	return &st
}

// ── DX binary helper ──────────────────────────────────────────────────────

func runOpenCodeDx(args ...string) (string, error) {
	dxPath, err := os.Executable()
	if err != nil {
		dxPath = "dx"
	}
	out, err := exec.Command(dxPath, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
