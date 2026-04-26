package agent

import (
	"context"
	"fmt"
	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/mcpcmd"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func agentLocalCmd() *cobra.Command {
	var loop bool
	var alias string
	var issue string
	var maxTurns int
	var complexity string
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Run local-LLM agent sessions with zdx integration",
		Long: `Run an OpenAI-compatible chat-completions loop against the configured
llm_local endpoint with tool-calling enabled. Registered tools span the
filesystem (read/write/edit/glob/grep/list_dir) and shell (run_bash); use
shell tools to invoke dx CLI commands directly. Every message, tool_use,
and tool_result is written as Claude-compatible JSONL to
.zdx/agent/local/<sid>.jsonl and streamed to the server for the
sessions/agents UI.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !validComplexity(complexity) {
				return fmt.Errorf("--complexity must be one of low|medium|high (got %q)", complexity)
			}
			global, _ := cmd.Flags().GetBool("global")
			global = global || config.IsGlobalMode()
			var cfg *config.Config
			if !global {
				cfg = config.Load()
			}
			var rc remoteConfig
			if global {
				if globalCfg := config.LoadGlobal(); globalCfg != nil {
					fmt.Fprintln(os.Stderr, "srcless mode: using ~/.zdx/config.yaml (no project config found)")
					rc = remoteConfig{
						url: globalCfg.Remote.URL,
						key: config.GlobalRemoteAPIKey(),
					}
				}
			} else {
				rc = remoteConfig{
					url:  cfg.RemoteURL(),
					slug: cfg.RemoteSlug(),
					key:  config.RemoteAPIKey(),
				}
			}
			llmCfg := cfg.ResolvedLLMLocal()
			llmCfg = applyComplexityModel(cmd.Context(), llmCfg, complexity)

			if loop {
				return runLocalLoop(cmd.Context(), rc, llmCfg, alias, maxTurns)
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			installReleaseOnSignal(rc, alias, "", nil, cancel)
			sid := uuid.New().String()
			return runLocalSession(ctx, rc, llmCfg, sid, issue, alias, "", maxTurns)
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 40, "cap on assistant turns per session")
	cmd.Flags().StringVar(&complexity, "complexity", "medium", "model slot to use: low|medium|high (from server admin/llm config)")
	return cmd
}

func validComplexity(c string) bool {
	switch c {
	case "low", "medium", "high":
		return true
	}
	return false
}

// applyComplexityModel overrides llmCfg.Model with the matching slot from the
// server's zdx_llm_configs (set via admin/llm). Walks configs in priority
// order and uses the first non-empty slot for the requested complexity; on
// any error the local config's Model is kept so the agent still has a
// working default.
func applyComplexityModel(ctx context.Context, llmCfg config.LLMLocal, complexity string) config.LLMLocal {
	c, err := cli.DefaultClient()
	if err != nil {
		return llmCfg
	}
	resp, err := c.ListLlmConfigsWithResponse(ctx)
	if err != nil || resp == nil || resp.JSON200 == nil || resp.JSON200.Configs == nil {
		return llmCfg
	}
	for _, cfg := range *resp.JSON200.Configs {
		var picked string
		switch complexity {
		case "low":
			picked = cfg.ModelLow
		case "high":
			picked = cfg.ModelHigh
		default:
			picked = cfg.ModelMedium
		}
		if picked != "" {
			llmCfg.Model = picked
			return llmCfg
		}
	}
	return llmCfg
}

// runLocalSession wraps the local-LLM chat loop in a localAdapter and drives
// it through the shared RunLifecycle runner. Event tailing, WS streaming,
// and session close are all owned by RunLifecycle.
func runLocalSession(ctx context.Context, rc remoteConfig, llmCfg config.LLMLocal, sid, issueID, alias, seedPrompt string, maxTurns int) error {
	adapter := &localAdapter{
		llmCfg:     llmCfg,
		maxTurns:   maxTurns,
		seedPrompt: seedPrompt,
	}
	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "local-cli", 0)
	return err
}

// ── Local AgentAdapter ────────────────────────────────────────────────────

// localAdapter implements AgentAdapter for the in-process local-LLM loop.
// The chat loop writes Claude-compatible JSONL to
// .zdx/agent/local/<sid>.jsonl; the shared runner tails that file, so no
// bespoke tailer or stream POST is needed here.
type localAdapter struct {
	llmCfg     config.LLMLocal
	maxTurns   int
	seedPrompt string

	doneCh chan struct{}
	runErr error

	toolNames map[string]string
}

func (a *localAdapter) Provider() string { return "local" }

func (a *localAdapter) Start(ctx context.Context, sid, issueID, alias string) (string, error) {
	// Set author alias so agent-posted comments are tagged and excluded
	// from unread-comment queries (prevents self-review loops).
	if alias != "" {
		os.Setenv("DX_AUTHOR_ALIAS", alias)
	}
	root, err := cli.GitRepoRoot()
	if err != nil {
		return "", fmt.Errorf("dx agent local must run inside a git repo: %w", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "dx-agent-local",
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

	sessLog, err := newSessionLog(sid, issueID, root)
	if err != nil {
		disp.Close()
		dispCancel()
		return "", err
	}

	system := localSystemPrompt(alias, issueID)
	user := a.seedPrompt
	if user == "" {
		if issueID != "" {
			user = fmt.Sprintf("Work the vertical for %s. Use run_bash to invoke `dx` CLI commands (issue show, comment add, todo dev start/done, ...) for project state, and filesystem tools to implement code. Stop when the issue is closed.", issueID)
		} else {
			user = "Use run_bash to call `dx todo solo` to pick the next item, then work it. Stop when idle."
		}
	}

	fmt.Printf("── session %s  issue=%s  model=%s ──\n", sid, issueID, a.llmCfg.Model)
	cs := newChatSession(a.llmCfg, disp, sessLog, a.maxTurns)

	a.doneCh = make(chan struct{})
	go func() {
		defer close(a.doneCh)
		defer disp.Close()
		defer dispCancel()
		defer sessLog.Close()
		a.runErr = cs.Run(ctx, system, user)
	}()

	return sessLog.path, nil
}

func (a *localAdapter) Wait() (int, error) {
	<-a.doneCh
	if a.runErr != nil {
		return 1, a.runErr
	}
	return 0, nil
}

func (a *localAdapter) SubagentDir(_ string) string { return "" }

func (a *localAdapter) ParseLine(line []byte, agentID string) (AgentEvent, error) {
	// Local transcripts use the same Claude-compatible JSONL envelope, so
	// the Claude parser is a natural fit.
	return (&claudeAdapter{}).ParseLine(line, agentID)
}

func (a *localAdapter) RenderEvent(eventJSON []byte) string {
	if a == nil {
		return ""
	}
	// Mirror the Claude renderer so .zdx/logs/agent.log reads uniformly
	// across providers; tool-name state lives on a per-adapter map so tool
	// results can be correlated with their originating tool_use.
	if a.toolNames == nil {
		a.toolNames = map[string]string{}
	}
	return renderSessionEvent(eventJSON, a.toolNames)
}

// runLocalLoop mirrors agent_claude.runLoop: poll solo, run a session per pick,
// repeat. Session state is persisted so SIGINT releases claimed tasks.
func runLocalLoop(parentCtx context.Context, rc remoteConfig, llmCfg config.LLMLocal, alias string, maxTurns int) error {
	stateFile := ".zdx/cache/local-agent-state"
	logFile := ".zdx/logs/local-agent.log"
	_ = os.MkdirAll(".zdx/logs", 0o755)
	_ = os.MkdirAll(".zdx/cache", 0o755)

	logf, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if logf != nil {
		defer logf.Close()
	}

	logfn := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		line := fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), msg)
		fmt.Print(line)
		if logf != nil {
			_, _ = logf.WriteString(line)
		}
	}

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	installReleaseOnSignal(rc, alias, stateFile, logfn, cancel)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		todo, err := runDxTodoSolo("")
		if err != nil || todo == "" {
			logfn("idle; sleeping 60s")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(60 * time.Second):
				continue
			}
		}
		logfn("todo solo:\n%s", todo)
		issueID := extractIssueID(todo)
		sid := uuid.New().String()
		_ = os.WriteFile(stateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)

		logfn("── SESSION START  session=%s  issue=%s ──", sid, issueID)
		start := time.Now()
		seed := fmt.Sprintf("Here is the current todo pick from `dx todo solo`:\n\n%s\n\nWork this vertical: resolve triage/decompose/comment/dev items for the referenced issue, then close it. Use dx tools for project ops and filesystem/shell tools for code changes. Stop when the issue is closed or blocked.", todo)
		err = runLocalSession(ctx, rc, llmCfg, sid, issueID, alias, seed, maxTurns)
		if err != nil {
			logfn("session error: %v", err)
		}
		logfn("── SESSION END    session=%s  duration=%s ──", sid, time.Since(start).Truncate(time.Second))
		_ = os.Remove(stateFile)
	}
}

func localSystemPrompt(alias, issueID string) string {
	var b strings.Builder
	b.WriteString("You are dx agent local, a local-LLM autonomous developer operating inside a git repo.\n\n")
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
