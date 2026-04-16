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
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Run local-LLM agent sessions with zdx integration",
		Long: `Run an OpenAI-compatible chat-completions loop against the configured
llm_local endpoint with tool-calling enabled. Registered tools span the dx
project API, the filesystem (read/write/edit/glob/grep/list_dir) and shell
(run_bash). Every message, tool_use, and tool_result is written as Claude-
compatible JSONL to .zdx/agent/local/<sid>.jsonl and streamed to the server
for the sessions/agents UI.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			rc := remoteConfig{
				url:  cfg.RemoteURL(),
				slug: cfg.RemoteSlug(),
				key:  config.RemoteAPIKey(),
			}
			llmCfg := cfg.ResolvedLLMLocal()

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
	return cmd
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
	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "local-cli")
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
	c, err := cli.DefaultClient()
	if err != nil {
		return "", err
	}
	root, err := cli.GitRepoRoot()
	if err != nil {
		return "", fmt.Errorf("dx agent local must run inside a git repo: %w", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "dx-agent-local",
		Version: "0.1.0",
	}, nil)
	mcpcmd.RegisterMCPTools(srv, c)
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
			user = fmt.Sprintf("Work the vertical for %s. Use dx tools to read, triage, decompose, and close tasks. Use filesystem/shell tools to implement code. Stop when the issue is closed.", issueID)
		} else {
			user = "Pick the next item from `todo_solo` and work it. Stop when idle."
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
	b.WriteString("  - dx project tools: issue_list, issue_add, issue_show, issue_close, todo_solo, todo_show, todo_tech_add, todo_dev_done, todo_owner_triage, comment_list, comment_add, feature_list, feature_show, pattern_search, question_search, question_add.\n")
	b.WriteString("  - filesystem tools: read_file, write_file, edit_file, list_dir, glob, grep.\n")
	b.WriteString("  - shell: run_bash.\n\n")
	b.WriteString("Operating rules:\n")
	b.WriteString("  - Prefer dx tools for project state (never re-derive from the filesystem alone).\n")
	b.WriteString("  - Edit minimally; read surrounding context before writing.\n")
	b.WriteString("  - After code edits, verify with run_bash (go build, tests, linters as appropriate).\n")
	b.WriteString("  - When the vertical is done, call issue_close to finish.\n")
	b.WriteString("  - Stop emitting tool_calls when you are finished; a final text reply ends the session.\n\n")
	if alias != "" {
		b.WriteString("Your agent alias is: " + alias + ".\n")
	}
	if issueID != "" {
		b.WriteString("Current issue: " + issueID + ".\n")
	}
	return b.String()
}
