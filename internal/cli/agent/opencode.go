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
	"github.com/iodesystems/zdx-go/internal/cli/agent/tracelog"
	"github.com/iodesystems/zdx-go/internal/cli/mcpcmd"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/llm"
)

// ── OpenCode AgentAdapter ─────────────────────────────────────────────────

func init() {
	// Register one constructor under three names: "openai" is canonical,
	// "opencode" and "local" remain as aliases for back-compat (the previous
	// design split these into separate providers; they were always doing
	// the same thing — chat-completions against an OpenAI-compatible
	// endpoint with our in-process MCP tools).
	ctor := func(opts ProviderOpts) (AgentProvider, error) {
		llmCfg := opts.LLMLocal
		if opts.Model != "" {
			llmCfg.Model = opts.Model
		}
		// Pull endpoint + api_key + (when --model unset) model from the
		// server's admin/llm-configs for the chosen complexity tier. Lets
		// the operator point the agent at a remote LLM without local
		// llm_local settings.
		if opts.Complexity != "" {
			if serverCfg := resolveLLMConfigFromServer(opts.RC, opts.Complexity); serverCfg.BaseURL != "" {
				llmCfg.BaseURL = serverCfg.BaseURL
				if serverCfg.APIKey != "" {
					llmCfg.APIKey = serverCfg.APIKey
				}
				if opts.Model == "" {
					llmCfg.Model = serverCfg.Model
				}
				if serverCfg.TimeoutSeconds > 0 {
					llmCfg.TimeoutSeconds = serverCfg.TimeoutSeconds
				}
			}
		}
		return &opencodeAdapter{
			rc:            opts.RC,
			llmCfg:        llmCfg,
			maxTurns:      opts.MaxTurns, // 0 = unlimited
			seedPrompt:    opts.SeedPrompt,
			mcpCommand:    opts.MCPCommand,
			complexity:    opts.Complexity,
			persona:       opts.Persona,
			spinThreshold: opts.SpinLockThreshold,
			tlog:          opts.TraceLog,
		}, nil
	}
	RegisterProvider("openai", ctor)
	RegisterProvider("opencode", ctor) // deprecated alias
	RegisterProvider("local", ctor)    // deprecated alias
}

// opencodeAdapter (kept under its legacy name for now to minimize the diff;
// it actually implements the unified `openai` provider — chat-completions
// against any OpenAI-compatible endpoint, with our in-process MCP tools).
// Drives an in-process chat-completions loop, writing Claude-compatible
// JSONL to .zdx/agent/openai/<sid>.jsonl. Session state is persisted as
// JSON in .zdx/state/openai/<sid>.json for resume.
type opencodeAdapter struct {
	rc            remoteConfig // populated by callers that need ResolveModel; safe to leave zero otherwise
	llmCfg        config.LLMLocal
	maxTurns      int
	seedPrompt    string
	mcpCommand    []string // when non-empty, dispatch tools via newRemoteDispatcher (dev-container mode)
	complexity    string   // canonical tier — surfaced in setup events for review/escalation telemetry
	persona       string   // role tier (NormalizePersona-d); empty defaults to "dev" (IS-1096)
	spinThreshold int      // IS-1116 spin-lock detector threshold; ≤0 disables
	tlog          *tracelog.Logger

	doneCh chan struct{}
	runErr error
	cs     *opencodeChatSession // captured post-Start so AbortInfo can surface spin-lock state

	toolNames map[string]string
}

func (a *opencodeAdapter) Provider() string { return "openai" }

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
	setupStart := time.Now()
	if a.tlog != nil {
		a.tlog.Info("setup.start",
			"provider", "openai",
			"sid", sid,
			"alias", alias,
			"issue_id", issueID,
			"model", a.llmCfg.Model,
			"complexity", a.complexity,
			"transport", a.dispatcherTransport(),
			"endpoint", a.llmCfg.BaseURL)
	}

	if alias != "" {
		os.Setenv("DX_AUTHOR_ALIAS", alias)
	}
	// Inject agent issue ID so dx test --escalate can auto-file blockers
	// scoped to the right issue (parity with the legacy local provider).
	if issueID != "" {
		os.Setenv("DX_AGENT_ISSUE", issueID)
	}
	root, err := cli.GitRepoRoot()
	if err != nil {
		return "", fmt.Errorf("dx agent openai must run inside a git repo: %w", err)
	}

	dispCtx, dispCancel := context.WithCancel(ctx)
	disp, err := a.buildDispatcher(dispCtx, root)
	if err != nil {
		dispCancel()
		return "", err
	}
	if a.tlog != nil {
		a.tlog.Info("mcp.attach",
			"sid", sid,
			"transport", a.dispatcherTransport(),
			"tools_count", len(disp.tools),
			"tools", disp.toolNames())
	}

	sessLog, err := newOpenCodeSessionLog(sid, issueID, root)
	if err != nil {
		disp.Close()
		dispCancel()
		return "", err
	}

	saveOpenCodeState(sid, issueID, alias, a.llmCfg.Model, root)

	system := opencodeSystemPrompt(alias, issueID, a.persona)
	user := a.seedPrompt
	if user == "" {
		if issueID != "" {
			user = fmt.Sprintf("Work the vertical for %s. Use run_bash to invoke `dx` CLI commands (issue show, comment add, todo dev start/done, ...) for project state, and filesystem tools to implement code. Stop when the issue is closed.", issueID)
		} else {
			// This branch should be unreachable: `dx agent` (single-session)
			// now hard-errors when --issue is empty (see agent.go RunE), and
			// the loop builds an explicit seed from the claimed todo. Keep
			// a sane fallback rather than silently inventing work — and do
			// NOT reference a phantom `dx agent claim` CLI command (the
			// previous fallback led the agent on a 100+ turn confused run
			// trying to invoke a subcommand that has never existed).
			user = "No issue or seed provided. Exit without action — this is a harness bug; the caller should have supplied --issue or a seed prompt."
		}
	}

	fmt.Printf("── session %s  issue=%s  model=%s ──\n", sid, issueID, a.llmCfg.Model)

	if a.tlog != nil {
		a.tlog.Info("setup.done",
			"sid", sid,
			"took_ms", time.Since(setupStart).Milliseconds())
	}

	a.doneCh = make(chan struct{})
	a.cs = newOpenCodeChatSession(a.llmCfg, disp, sessLog, a.maxTurns, a.spinThreshold)
	go func() {
		defer close(a.doneCh)
		defer disp.Close()
		defer dispCancel()
		defer sessLog.Close()

		a.runErr = a.cs.Run(ctx, system, user)
	}()

	return sessLog.path, nil
}

// dispatcherTransport returns the human-readable transport label used in
// setup events. "stdio" when an external MCP subprocess is configured (dev
// container mode); "in-process" when tools are registered directly into the
// host process's MCP server.
func (a *opencodeAdapter) dispatcherTransport() string {
	if len(a.mcpCommand) > 0 {
		return "stdio"
	}
	return "in-process"
}

// buildDispatcher returns the MCP dispatcher Start should use. Default is
// the in-process server (root = the repo's GitRepoRoot). When mcpCommand
// is set, dispatches through a remote MCP server spawned via that argv —
// the host process runs the chat loop, the subprocess executes tools.
func (a *opencodeAdapter) buildDispatcher(ctx context.Context, root string) (*localDispatcher, error) {
	if len(a.mcpCommand) > 0 {
		return newRemoteDispatcher(ctx, a.mcpCommand, a.persona)
	}
	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "dx-agent-opencode",
		Version: "0.1.0",
	}, nil)
	mcpcmd.RegisterFSTools(srv, root)
	mcpcmd.RegisterShellTools(srv, root)
	mcpcmd.RegisterOutlineTools(srv, root)
	return newLocalDispatcher(ctx, srv, a.persona)
}

func (a *opencodeAdapter) Wait() (int, error) {
	<-a.doneCh
	if a.runErr != nil {
		return 1, a.runErr
	}
	return 0, nil
}

// AbortInfo implements AbortReporter — RunLifecycle calls this post-Wait
// and forwards any non-nil reason to close-agent-session so server-side
// telemetry (TK-1791) emits session.aborted_spin_lock.
func (a *opencodeAdapter) AbortInfo() *AbortInfo {
	if a.cs == nil {
		return nil
	}
	return a.cs.abort
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
	dir := ".zdx/agent/openai"
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

// writeCompaction appends a tombstone.applied or compaction.applied record
// referencing an original event by UUID. The original event is never
// rewritten — compactions are attached, not destructive.
func (s *opencodeSessionLog) writeCompaction(rec SessionEvent) error {
	ev := map[string]any{
		"type":              rec.Type,
		"strategy":          rec.Strategy,
		"compacted_content": rec.CompactedContent,
	}
	if rec.EventID != "" {
		ev["event_id"] = rec.EventID
	}
	if rec.OriginalSize > 0 {
		ev["original_size"] = rec.OriginalSize
	}
	return s.writeEvent(ev)
}

// ── Chat session loop ─────────────────────────────────────────────────────

type opencodeChatSession struct {
	client     *llm.Client
	summarizer *llm.Client
	dispatcher *localDispatcher
	log        *opencodeSessionLog
	tools      []llm.ToolDef
	maxTurns   int
	timeout    time.Duration

	// Context-window awareness for the 3-phase compaction ladder.
	// nCtx is probed from the model's /props endpoint at session start;
	// 0 means probe failed and compaction is disabled. phase is
	// 1=raw, 2=tombstone, 3=compact.
	nCtx  int
	phase int

	// IS-1116 spin-lock detector. Trips when the most recent N tool_calls
	// are bit-identical (tool + canonical args). When set, abort holds the
	// reason the adapter surfaces back to RunLifecycle via AbortReporter.
	spin  *spinTracker
	abort *AbortInfo
}

func newOpenCodeChatSession(llmCfg config.LLMLocal, disp *localDispatcher, log *opencodeSessionLog, maxTurns, spinThreshold int) *opencodeChatSession {
	client := llm.New(llm.Config{
		Type:   "openai",
		URL:    llmCfg.BaseURL,
		Model:  llmCfg.Model,
		APIKey: llmCfg.APIKey,
	})
	return &opencodeChatSession{
		client:     client,
		summarizer: client,
		dispatcher: disp,
		log:        log,
		tools:      disp.OpenAIFunctions(),
		maxTurns:   maxTurns,
		timeout:    time.Duration(llmCfg.TimeoutSeconds) * time.Second,
		phase:      1,
		spin:       newSpinTracker(spinThreshold),
	}
}

func (cs *opencodeChatSession) Run(ctx context.Context, system, user string) error {
	_ = cs.log.AITitle(cli.Truncate(user, 80))
	_ = cs.log.UserText(user)

	// Probe the model server for n_ctx so the compaction ladder has a real
	// budget to reason against. Failure is non-fatal — compaction simply
	// stays disabled and the session behaves like before.
	if props, err := cs.client.FetchServerProps(ctx); err == nil && props != nil && props.NCtx > 0 {
		cs.nCtx = props.NCtx
	} else {
		cs.nCtx = defaultNCtx
	}

	msgs, err := cs.rehydrate(system, user)
	if err != nil {
		return fmt.Errorf("initial hydrate: %w", err)
	}

	for turn := 0; cs.maxTurns == 0 || turn < cs.maxTurns; turn++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Force single tool_call per turn. Local chat templates (e.g.
		// llama.cpp jinja for Qwen3) abort the server when cumulative
		// tool_result payload in a single request exceeds ~50KB; serializing
		// tool calls keeps each turn's request bounded by one tool result
		// rather than the fan-out sum.
		serial := false
		chatStart := time.Now()
		fmt.Fprintf(os.Stderr, "→ llm: turn=%d msgs=%d tools=%d timeout=%s\n",
			turn, len(msgs), len(cs.tools), cs.timeout)
		resp, err := cs.client.ChatCompletion(ctx, &llm.ChatRequest{
			Messages:          msgs,
			Tools:             cs.tools,
			ToolChoice:        "auto",
			Timeout:           cs.timeout,
			ParallelToolCalls: &serial,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "← llm: turn=%d failed after %s: %v\n",
				turn, time.Since(chatStart).Truncate(time.Millisecond), err)
			// Record a final assistant event so the failure is visible in the
			// session JSONL — otherwise the log dangles after the last
			// tool_result and operators have no fingerprint of what crashed.
			_ = cs.log.AssistantText(fmt.Sprintf("(stopped: chat error on turn %d: %v)", turn, err), nil)
			return fmt.Errorf("turn %d: %w", turn, err)
		}
		fmt.Fprintf(os.Stderr, "← llm: turn=%d ok in %s (in=%d out=%d)\n",
			turn, time.Since(chatStart).Truncate(time.Millisecond),
			resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

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

			// IS-1116 spin-lock detection. Push the agent-issued payload
			// (not the dispatcher's result) so internal retries in the tool
			// client don't masquerade as agent churn. Trip → set abort
			// metadata on the session and bail out before Dispatch so we
			// don't burn another tool turn on the same stuck call.
			if cs.spin.enabled() {
				digest := argsDigest(tc.Function.Arguments)
				cs.spin.Push(tc.Function.Name, digest)
				if entry, count, ok := cs.spin.Tripped(); ok {
					cs.abort = &AbortInfo{
						Reason: "spin_lock",
						SpinLock: &SpinLockInfo{
							Tool:        entry.tool,
							ArgsDigest:  entry.digest,
							RepeatCount: int32(count),
							LastTurn:    int32(turn),
						},
					}
					fmt.Fprintf(os.Stderr, "[spin-lock] turn=%d tool=%s repeat=%d digest=%s — aborting session\n",
						turn, entry.tool, count, entry.digest)
					_ = cs.log.AssistantText(fmt.Sprintf("(stopped: spin-lock — tool %s repeated %d turns with identical args)", entry.tool, count), nil)
					return nil
				}
			}

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

		// 3-phase compaction ladder. Pressure is computed against the
		// last response's prompt_tokens + a reservation for the next
		// completion. Phase advances on threshold breach (60% standard,
		// 80% panic). Each transition writes compaction records to the
		// log and re-hydrates msgs[] from the post-compaction state.
		if next, ok := cs.shouldCompact(resp.Usage.PromptTokens); ok {
			rebuilt, err := cs.compactAndRehydrate(ctx, next, system, user)
			if err != nil {
				_ = cs.log.AssistantText(fmt.Sprintf("(compaction error on turn %d: %v)", turn, err), nil)
				return fmt.Errorf("compaction turn %d: %w", turn, err)
			}
			msgs = rebuilt
			cs.phase = next
		}
	}

	_ = cs.log.AssistantText(fmt.Sprintf("(stopped: max-turns=%d exceeded)", cs.maxTurns), nil)
	return fmt.Errorf("max turns (%d) exceeded", cs.maxTurns)
}

// rehydrate reads the JSONL session log and reconstructs msgs[] using the
// default hydration strategy (most recent compaction wins per event).
func (cs *opencodeChatSession) rehydrate(system, user string) ([]llm.ChatMsg, error) {
	events, err := readSessionEvents(cs.log.path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return hydrate(events, system, user, HydrationStrategy{}), nil
}

// shouldCompact wraps CheckThreshold for the session loop. It clamps panic
// returns to "advance one phase" (matching the existing ratchet semantics)
// and reports whether any compaction action is needed.
func (cs *opencodeChatSession) shouldCompact(promptTokens int) (int, bool) {
	if cs.phase >= int(PhaseCompact) {
		return cs.phase, false
	}
	next := CheckThreshold(promptTokens, defaultResponseReserveTokens, cs.nCtx, Phase(cs.phase))
	if next == PhasePanic || int(next) > cs.phase {
		return cs.phase + 1, true
	}
	return cs.phase, false
}

// compactAndRehydrate writes the records for the target phase to the log and
// rebuilds msgs[]. Phase 2 = tombstone large tool_results; phase 3 = LLM
// summary of the dropped middle, with the verbatim tail preserved.
func (cs *opencodeChatSession) compactAndRehydrate(ctx context.Context, targetPhase int, system, user string) ([]llm.ChatMsg, error) {
	events, err := readSessionEvents(cs.log.path)
	if err != nil {
		return nil, fmt.Errorf("read session log: %w", err)
	}

	var records []SessionEvent
	switch targetPhase {
	case 2:
		records = applyTombstone(events, HydrationStrategy{})
	case 3:
		records, err = applyCompaction(ctx, cs.summarizer, events, HydrationStrategy{})
		if err != nil {
			return nil, err
		}
	}

	for _, r := range records {
		if werr := cs.log.writeCompaction(r); werr != nil {
			return nil, fmt.Errorf("write compaction record: %w", werr)
		}
	}

	return cs.rehydrate(system, user)
}

// ── System prompt ─────────────────────────────────────────────────────────

func opencodeSystemPrompt(alias, issueID, persona string) string {
	var b strings.Builder
	if block, err := PersonaPrompt(persona); err == nil && block != "" {
		b.WriteString(block)
		b.WriteString("\n\n")
	}
	b.WriteString("You are dx agent (openai), an autonomous developer operating inside a git repo.\n\n")
	b.WriteString("Available tool categories:\n")
	b.WriteString("  - filesystem tools: read_file, write_file, edit_file, list_dir, glob, grep.\n")
	b.WriteString("  - structural outline: `outline` (backends: Go, TS/TSX/JS/JSX). Prefer this over read_file when probing what a file or package exposes. Pass `lod` (0=files-only, 1=decl names, 2=signatures (default), 3=full) and `json_path` (e.g. 'files/0/decls') to drill in progressively. Function bodies are never included.\n")
	b.WriteString("  - shell: run_bash. Invoke `dx` CLI for project state — e.g. `dx issue show IS-N`, `dx comment add`, `dx todo dev start/done`, `dx feature show`, `dx pattern search`, `dx question add`. Run `dx --help` for the full tree.\n\n")
	b.WriteString("Tooling:\n")
	b.WriteString("  - The `dx` binary is available on PATH at /workspace/bin. You can call `dx` directly from run_bash without locating it first.\n\n")
	b.WriteString("Operating rules:\n")
	b.WriteString("  - Prefer `dx` CLI calls for project state (never re-derive from the filesystem alone).\n")
	b.WriteString("  - Edit minimally; read surrounding context before writing.\n")
	b.WriteString("  - After code edits, verify with run_bash (go build, tests, linters as appropriate).\n")
	b.WriteString("  - When the vertical is done, run `dx issue close` to finish.\n")
	b.WriteString("  - Stop emitting tool_calls when you are finished; a final text reply ends the session.\n\n")
	b.WriteString("Worker contract — do NOT edit generated files. These are owned by the merge-train and regenerated on top of your intent diff. Editing them by hand creates merge-train conflicts and lint drift failures, and edits will be lost in the regen step.\n")
	b.WriteString("  - internal/db/*.sql.go             — sqlc output; edit queries/*.sql instead and run `~/go/bin/sqlc generate`.\n")
	b.WriteString("  - internal/db/*.sql.metaquery.go   — sqlc-metaquery output; suppress with `-- metaquery: off` in the .sql file, never edit the .go.\n")
	b.WriteString("  - internal/db/querier.go           — sqlc output (interface). Same fix as *.sql.go.\n")
	b.WriteString("  - internal/db/models.go            — sqlc output (row structs). Edit schema/queries, regen.\n")
	b.WriteString("  - internal/dxclient/models.gen.go  — oapi-codegen output; edit handlers + openapi.json (regen via `make gen-dxclient` or `dx ui gen-api`).\n")
	b.WriteString("  - internal/dxclient/openapi.json   — committed but only as intent; regenerate via `dx ui gen-api` after handler changes.\n")
	b.WriteString("  - ui/src/api.gen.ts                — openapi-typescript output; regen via `dx ui gen-api`.\n")
	b.WriteString("  - schema/shipped.sql               — pg_dump snapshot; owned by `bin/regen-schema` (merge-train) or `bin/db migrate`. Never hand-edit.\n")
	b.WriteString("  If a metaquery/sqlc/ts lint advisory points at a generated file, fix it at the source (the .sql, the handler, or the suppression comment) — do NOT patch the .gen.go/.gen.ts.\n\n")
	if alias != "" {
		b.WriteString("Your agent alias is: " + alias + ".\n")
	}
	if issueID != "" {
		b.WriteString("Current issue: " + issueID + ".\n\n")
		b.WriteString("TEST-FAILURE PROTOCOL:\n")
		b.WriteString("  Before committing, run: dx test --classify-preexisting --escalate\n")
		b.WriteString("  If preexisting failures are detected, the --escalate flag will auto-file\n")
		b.WriteString("  a deduplicated blocker issue. Do NOT attempt to fix preexisting failures.\n")
		b.WriteString("  If REGRESSION FAILURES are shown (caused by your diff), fix them first.\n\n")
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
	dir := ".zdx/state/openai"
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
	path := filepath.Join(".zdx", "state", "openai", sid+".json")
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
