package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/mcpcmd"
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
			tier, err := NormalizeComplexity(complexity)
			if err != nil {
				return err
			}
			opts, err := loadManagedOptsFromCmd(cmd, "local", alias, issue, "", tier, maxTurns)
			if err != nil {
				return err
			}
			if loop {
				return RunManagedLoop(cmd.Context(), "local", opts)
			}
			return RunManagedSession(cmd.Context(), "local", opts)
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 40, "cap on assistant turns per session")
	cmd.Flags().StringVar(&complexity, "complexity", DefaultComplexity, "model slot to use: low|medium|high (from server admin/llm config)")
	return cmd
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

func init() {
	RegisterProvider("local", func(opts ProviderOpts) (AgentProvider, error) {
		llmCfg := opts.LLMLocal
		if opts.Model != "" {
			llmCfg.Model = opts.Model
		}
		// Mirror opencode: pull full endpoint + api_key from admin/llm-configs
		// for the chosen tier so dx agent --provider=local works against a
		// remote LLM without project-local llm_local settings.
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
		maxTurns := opts.MaxTurns
		if maxTurns == 0 {
			maxTurns = 40 // matches the local CLI default
		}
		return &localAdapter{
			llmCfg:     llmCfg,
			maxTurns:   maxTurns,
			seedPrompt: opts.SeedPrompt,
		}, nil
	})
}

func (a *localAdapter) Provider() string { return "local" }

// ResolveModel maps a complexity tier to a concrete model name by walking the
// server's admin LLM config in priority order. Falls back to the adapter's
// llmCfg.Model when the server is unreachable or no slot matches.
func (a *localAdapter) ResolveModel(ctx context.Context, complexity string) (string, error) {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return "", err
	}
	resolved := applyComplexityModel(ctx, a.llmCfg, tier)
	return resolved.Model, nil
}

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
