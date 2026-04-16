package cli

import (
	"context"
	"fmt"
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
			installReleaseOnSignal(rc, alias, "", nil)
			sid := uuid.New().String()
			return runLocalSession(cmd.Context(), rc, llmCfg, sid, issue, alias, "", maxTurns)
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 40, "cap on assistant turns per session")
	return cmd
}

// runLocalSession runs one chat session against llmCfg for the given seed prompt.
// Either issueID or seedPrompt must be non-empty; when both empty it reads from stdin.
func runLocalSession(ctx context.Context, rc remoteConfig, llmCfg config.LLMLocal, sid, issueID, alias, seedPrompt string, maxTurns int) error {
	c, err := DefaultClient()
	if err != nil {
		return err
	}
	root, err := gitRepoRoot()
	if err != nil {
		return fmt.Errorf("dx agent local must run inside a git repo: %w", err)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "dx-agent-local",
		Version: "0.1.0",
	}, nil)
	registerMCPTools(srv, c)
	RegisterFSTools(srv, root)
	RegisterShellTools(srv, root)

	dispCtx, dispCancel := context.WithCancel(ctx)
	defer dispCancel()
	disp, err := newLocalDispatcher(dispCtx, srv)
	if err != nil {
		return err
	}
	defer disp.Close()

	log, err := newSessionLog(sid, issueID, root)
	if err != nil {
		return err
	}
	defer log.Close()

	streamCancel := log.StartStream(rc, issueID, alias)
	defer streamCancel()

	system := localSystemPrompt(alias, issueID)
	user := seedPrompt
	if user == "" {
		if issueID != "" {
			user = fmt.Sprintf("Work the vertical for %s. Use dx tools to read, triage, decompose, and close tasks. Use filesystem/shell tools to implement code. Stop when the issue is closed.", issueID)
		} else {
			user = "Pick the next item from `todo_solo` and work it. Stop when idle."
		}
	}

	fmt.Printf("── session %s  issue=%s  model=%s ──\n", sid, issueID, llmCfg.Model)
	cs := newChatSession(llmCfg, disp, log, maxTurns)
	return cs.Run(ctx, system, user)
}

// runLocalLoop mirrors agent_claude.runLoop: poll solo, run a session per pick,
// repeat. Session state is persisted so SIGINT releases claimed tasks.
func runLocalLoop(ctx context.Context, rc remoteConfig, llmCfg config.LLMLocal, alias string, maxTurns int) error {
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

	installReleaseOnSignal(rc, alias, stateFile, logfn)

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
