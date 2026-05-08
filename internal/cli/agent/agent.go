package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/doctor"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// AgentCmd is the parent for every agent verb (run/loop/serve/lifecycle).
// The execution-mode flags — --container, --worktree, --branch, --complexity
// — are persistent so any verb inherits them without redeclaration. That
// keeps the grammar coherent: the verb says *what* (single task vs loop vs
// managed daemon); the persistent adjectives say *how* (in a container or
// on the host, on which worktree/branch, at what model tier).
//
// Defaults are "auto" / "docker" — the system picks. Operators only specify
// these flags when they want to override.
//
// Phase-1 scope (today): the flags are accepted and parsed. --container=docker
// and --complexity=auto take their full effect (loop dispatches to container
// mode by default; complexity resolves through the existing provider chain).
// --worktree and --branch parse and reject non-"auto" values until Stream 1
// (per-issue branch lifecycle, IS-1048) lands.
func AgentCmd() *cobra.Command {
	var provider, alias, issue, model, complexity, container, worktree, branch, mcpContainer string
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run a single agent session (use `dx agent loop` for the work loop)",
		Long: `Run an agent session against the configured project. The provider is
selected via --provider; complexity-to-model resolution is delegated to the
provider, so dx agent --provider=claude --complexity=high picks claude-opus-4-7,
while --provider=openai --complexity=high picks the project's configured
high-tier model from admin/llm-configs (and dispatches via OpenAI-compatible
chat-completions to whatever endpoint that config points at — local llama-swap,
hosted OpenAI, OpenRouter, etc.). The legacy --provider=opencode and
--provider=local names remain accepted aliases.

Execution environment is selected via --container=docker|local (default
docker). Container mode runs the agent inside MCP-slot containers with an
isolated worktree per slot; local mode runs claude directly on the host
against the operator's working tree.

For the long-running work loop, use ` + "`dx agent loop --provider=X`" + `.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When invoked with no flags, show usage rather than mint tokens.
			if provider == "" && len(args) == 0 {
				return cmd.Help()
			}
			mode, err := NormalizeContainer(container)
			if err != nil {
				return err
			}
			if err := rejectUnimplementedExplicit("worktree", worktree); err != nil {
				return err
			}
			if err := rejectUnimplementedExplicit("branch", branch); err != nil {
				return err
			}
			tier, terr := NormalizeComplexity(complexity)
			if terr != nil {
				return terr
			}
			opts, oerr := loadManagedOptsFromCmd(cmd, provider, alias, issue, model, tier, 0, mcpContainer)
			if oerr != nil {
				return oerr
			}
			if mode != ContainerDocker {
				// Local execution. enforceContainerExecution gates on
				// DX_AGENT_FORCE_CONTAINER (spec 117) — operators can refuse
				// host execution at the env level even when --container=local.
				if err := enforceContainerExecution(false); err != nil {
					return err
				}
			}
			executor, eerr := PickExecutor(mode, provider, opts)
			if eerr != nil {
				return eerr
			}
			return DispatchSingle(cmd.Context(), provider, opts, executor)
		},
	}
	// Persistent identity/role flags — inherited by every subcommand so the
	// grammar is "dx agent --provider=X --alias=Y <verb>" regardless of verb.
	cmd.PersistentFlags().StringVar(&provider, "provider", "", "agent provider: "+providerNames()+" (required for managed run)")
	cmd.PersistentFlags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.PersistentFlags().StringVar(&model, "model", "", "explicit model name (overrides --complexity)")
	cmd.PersistentFlags().StringVar(&complexity, "complexity", "auto", "model tier: auto|low|medium|high (auto = high or config-derived; resolved by the provider)")

	// Persistent execution-environment flags — the *how*, orthogonal to the
	// verb. Defaults are "auto" / "docker"; operators override when they want
	// host execution or pinned worktree/branch.
	cmd.PersistentFlags().StringVar(&container, "container", "docker", "execution environment: docker (default) | local")
	cmd.PersistentFlags().StringVar(&worktree, "worktree", "auto", "worktree path (auto = system picks/creates per claim) — non-auto pending IS-1048")
	cmd.PersistentFlags().StringVar(&branch, "branch", "auto", "git branch (auto = derived per claim) — non-auto pending IS-1048")

	// Single-session-only flags.
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().StringVar(&mcpContainer, "mcp-container", "", "dispatch tool calls through dx-agent --mcp-stdio running inside this container (opencode/local only)")

	cmd.AddCommand(agentLoopCmd(), agentConnectCmd(), agentStartCmd(), agentListCmd(), agentStopCmd(), agentReconnectCmd(), agentReleaseCmd(), agentSessionCmd(), agentPauseCmd(), agentResumeCmd(), agentDrainCmd(), agentBudgetCmd(), streamEvaluateCmd())
	return cmd
}

// agentConnectCmd registers the agent with the zdx server's pool, holds the
// WS connection alive, and (unless --idle) runs the work loop. The verb is
// the user-facing entry point for "make this agent visible in the server's
// /agents nav" — it covers both project-scoped and global-pool registration.
//
// With --global, the agent registers without a project binding; appears in
// /agents under "global" scope; receives no work until assigned (Stream-2
// follow-up). Without --global, registration inherits the project from
// .zdx/config.yaml just like `dx agent loop`.
//
// With --idle, registration completes but the work loop never starts. The
// agent stays connected (WS heartbeat alive, control channel open),
// waiting for the operator to dispatch work via the UI / control message.
//
// Default (no --idle) is "register and start working" — the user
// explicitly asked for this in the design conversation: "when an agent is
// connected, it should start working immediately unless started with
// connect --idle."
func agentConnectCmd() *cobra.Command {
	var idle bool
	var maxTurns int
	var keepContainer, chrome bool
	var mcpContainer string
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Register agent to the zdx server pool; start work loop unless --idle",
		Long: `Connect registers this agent in the server's agent pool (visible in
/agents nav) and starts the work loop by default. With --global registration
goes into the cross-project global pool; without it, the agent is project-
scoped and claims from this project's queue.

  dx agent connect --provider=claude              # project-scoped, working
  dx agent connect --provider=claude --idle       # project-scoped, idle
  dx agent connect --provider=claude --global     # global pool, idle until assigned

Honors all parent flags (--container, --worktree, --branch, --complexity,
--alias, --model). With --idle the work loop never runs — the WS stays
open so the UI can pause/resume/drain or eventually push work.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := cmd.Flag("provider").Value.String()
			alias := cmd.Flag("alias").Value.String()
			model := cmd.Flag("model").Value.String()
			complexity := cmd.Flag("complexity").Value.String()
			containerVal := cmd.Flag("container").Value.String()
			worktree := cmd.Flag("worktree").Value.String()
			branch := cmd.Flag("branch").Value.String()
			global, _ := cmd.Flags().GetBool("global")

			mode, err := NormalizeContainer(containerVal)
			if err != nil {
				return err
			}
			if err := rejectUnimplementedExplicit("worktree", worktree); err != nil {
				return err
			}
			if err := rejectUnimplementedExplicit("branch", branch); err != nil {
				return err
			}

			tier, err := NormalizeComplexity(complexity)
			if err != nil {
				return err
			}
			opts, err := loadManagedOptsFromCmd(cmd, provider, alias, "", model, tier, maxTurns, mcpContainer)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("chrome") {
				opts.Chrome = chrome
			}
			opts.KeepContainer = keepContainer
			// connect is one-agent-per-process by design: one WS handshake,
			// one row in /agents. Concurrency lives on `dx agent loop` for
			// the explicit fan-out opt-in; for N agents in the pool, run N
			// `dx agent connect` processes.
			opts.Concurrency = 1
			opts.Global = global
			opts.Idle = idle

			// Idle mode: just hold the WS daemon. No work loop, no slot
			// containers. The handshake with Idle=true tells the server
			// to surface the agent as paused/standby in /agents.
			if idle {
				return runIdleDaemon(cmd.Context(), provider, opts)
			}

			// Working mode: behave like `dx agent loop`. Executor is
			// chosen from --container; concurrency is hard-pinned to 1.
			if mode != ContainerDocker {
				if err := enforceContainerExecution(false); err != nil {
					return err
				}
			}
			executor, eerr := PickExecutor(mode, provider, opts)
			if eerr != nil {
				return eerr
			}
			return DispatchLoop(cmd.Context(), provider, opts, executor)
		},
	}
	cmd.Flags().Bool("global", false, "register into the server-wide global pool instead of this project's pool")
	cmd.Flags().BoolVar(&idle, "idle", false, "register but don't start the work loop; UI controls the agent")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 0, "cap on assistant turns per session (0 = unlimited; opencode/local only)")
	cmd.Flags().BoolVar(&keepContainer, "keep-container", false, "keep slot containers after exit (skip --rm; useful for debugging)")
	cmd.Flags().BoolVar(&chrome, "chrome", true, "pass --chrome to claude CLI (claude only; ignored otherwise)")
	cmd.Flags().StringVar(&mcpContainer, "mcp-container", "", "dispatch tool calls through dx-agent --mcp-stdio running inside this container (opencode/local only)")
	return cmd
}

// runIdleDaemon registers the agent via the WS daemon and blocks until ctx
// is cancelled. No work loop runs; the daemon's control channel is the
// only way the agent does anything once registered.
func runIdleDaemon(ctx context.Context, providerName string, opts ProviderOpts) error {
	if opts.RC.url == "" || opts.RC.key == "" {
		return fmt.Errorf("--idle requires a configured remote (RC.url + RC.key); set up ~/.zdx/config.yaml or run inside a project")
	}
	if opts.Alias == "" {
		opts.Alias = providerName + "-" + uuid.New().String()[:8]
	}
	holder := agentdaemon.NewLoopTaskHolder()
	startDaemon(ctx, providerName, opts, holder, nil)
	<-ctx.Done()
	return nil
}

// rejectUnimplementedExplicit refuses any value other than "auto" / empty for
// flags whose runtime support isn't wired yet. Surfaces the dependency so
// users learn the right path instead of seeing the flag silently ignored.
func rejectUnimplementedExplicit(flag, value string) error {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" || v == "auto" {
		return nil
	}
	return fmt.Errorf("--%s=%q not yet supported (tracked in IS-1048 — per-issue branch lifecycle); only --%s=auto is wired today", flag, value, flag)
}

// agentLoopCmd is the loop equivalent of dx agent. Dispatches through
// DispatchLoop (or DispatchContainerLoop with --container=docker) so
// providers that implement LoopProvider/ContainerProvider get their own
// runtime; plain providers get the universal RunManagedLoop.
//
// Identity/role/environment flags (--provider, --alias, --model,
// --complexity, --container, --worktree, --branch) are inherited from
// the parent. Loop-only flags (--max-turns, --keep-container,
// --concurrency, --chrome, --mcp-container) live here.
func agentLoopCmd() *cobra.Command {
	var mcpContainer string
	var maxTurns, concurrency int
	var keepContainer, chrome bool
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Loop: claim work, run a managed session per pick, repeat",
		Long: `Long-running loop that claims work via /api/dx/agent/claim and runs an
agent session per pick. Atomic claim/lease/release, churn backoff,
self-update re-exec, and crash-recovery release work for all providers.

Single agent, sequential task claims by default. With --container=docker
the loop's session runs inside a slot container (sleep infinity, /workspace
mounted, sandboxed); LLM lives on the host, tool calls dispatch into the
slot via docker exec dx-agent --mcp-stdio. Pass --container=local to run
claude directly on the host (debug / no-docker fallback).

For parallel work, run multiple ` + "`dx agent loop`" + ` processes — each is its
own visible alias and claim stream. The ` + "`--concurrency=N`" + ` flag is an
explicit opt-in to in-process fan-out (N slot containers under one parent
process); most operators should not need it. The agent.max_worktrees
config field is NOT consulted here — that's the project-wide
concurrent-agents ceiling read by ` + "`dx agent start`" + `.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			provider := cmd.Flag("provider").Value.String()
			alias := cmd.Flag("alias").Value.String()
			model := cmd.Flag("model").Value.String()
			complexity := cmd.Flag("complexity").Value.String()
			containerVal := cmd.Flag("container").Value.String()
			worktree := cmd.Flag("worktree").Value.String()
			branch := cmd.Flag("branch").Value.String()

			mode, err := NormalizeContainer(containerVal)
			if err != nil {
				return err
			}
			if err := rejectUnimplementedExplicit("worktree", worktree); err != nil {
				return err
			}
			if err := rejectUnimplementedExplicit("branch", branch); err != nil {
				return err
			}

			tier, err := NormalizeComplexity(complexity)
			if err != nil {
				return err
			}
			opts, err := loadManagedOptsFromCmd(cmd, provider, alias, "", model, tier, maxTurns, mcpContainer)
			if err != nil {
				return err
			}
			if cmd.Flags().Changed("chrome") {
				opts.Chrome = chrome
			}
			opts.KeepContainer = keepContainer
			opts.Concurrency = concurrency
			if mode != ContainerDocker {
				// Local execution. enforceContainerExecution still gates on
				// DX_AGENT_FORCE_CONTAINER (spec 117) — operators can refuse
				// host execution at the env level even when --container=local.
				if err := enforceContainerExecution(false); err != nil {
					return err
				}
			}
			executor, eerr := PickExecutor(mode, provider, opts)
			if eerr != nil {
				return eerr
			}
			return DispatchLoop(cmd.Context(), provider, opts, executor)
		},
	}
	cmd.Flags().IntVar(&maxTurns, "max-turns", 0, "cap on assistant turns per session (0 = unlimited; opencode/local only)")
	cmd.Flags().BoolVar(&keepContainer, "keep-container", false, "keep slot containers after exit (skip --rm; useful for debugging)")
	cmd.Flags().BoolVar(&chrome, "chrome", true, "pass --chrome to claude CLI (claude only; ignored otherwise)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, "in-process slot fan-out for --container=docker (default 1; for parallel work prefer running multiple `dx agent loop` processes)")
	cmd.Flags().StringVar(&mcpContainer, "mcp-container", "", "dispatch tool calls through dx-agent --mcp-stdio running inside this container (opencode/local only)")
	return cmd
}

// loadManagedOptsFromCmd centralizes the runtime + model-resolution dance
// so both dx agent and dx agent loop (and any legacy shims that route
// through the manager) build a populated ProviderOpts the same way.
//
// mcpContainer, when non-empty, becomes ProviderOpts.MCPCommand =
// {"docker", "exec", "-i", <container>, "dx-agent", "--mcp-stdio"} so
// opencode/local providers dispatch tools into the container instead of
// running their in-process MCP server. Empty means in-process tools.
func loadManagedOptsFromCmd(cmd *cobra.Command, provider, alias, issue, model, tier string, maxTurns int, mcpContainer string) (ProviderOpts, error) {
	rt, err := loadAgentRuntime(cmd)
	if err != nil {
		return ProviderOpts{}, err
	}

	resolved := model
	if resolved == "" {
		ctor, err := LookupProvider(provider)
		if err != nil {
			return ProviderOpts{}, err
		}
		ap, err := ctor(ProviderOpts{RC: rt.RC, AgentCfg: rt.AgentCfg, LLMLocal: rt.LLMLocal})
		if err != nil {
			return ProviderOpts{}, fmt.Errorf("resolve model: %w", err)
		}
		resolved, err = ap.ResolveModel(cmd.Context(), tier)
		if err != nil {
			return ProviderOpts{}, fmt.Errorf("resolve model (%s, complexity=%s): %w", provider, tier, err)
		}
	}

	var mcpCommand []string
	if mcpContainer != "" {
		mcpCommand = []string{"docker", "exec", "-i", mcpContainer, "dx-agent", "--mcp-stdio"}
	}

	return ProviderOpts{
		RC:         rt.RC,
		AgentCfg:   rt.AgentCfg,
		LLMLocal:   rt.LLMLocal,
		IssueID:    issue,
		Alias:      alias,
		Model:      resolved,
		Complexity: tier,
		Srcless:    rt.Srcless,
		WorkDir:    rt.WorkDir,
		Chrome:     true,
		MaxTurns:   maxTurns,
		MCPCommand: mcpCommand,
	}, nil
}

// agentRuntime holds the project + global config bits an agent command
// needs to dispatch a managed session or loop. Centralizes the global-vs-
// project loading dance so both the unified dx agent / dx agent loop
// commands and the legacy provider-specific subcommands resolve config
// the same way.
type agentRuntime struct {
	RC       remoteConfig
	AgentCfg config.AgentConfig
	LLMLocal config.LLMLocal
	Srcless  bool
	WorkDir  string // only set in srcless mode (from global agent config)
}

// loadAgentRuntime resolves config in this order: DX_GLOBAL=1 forces srcless;
// else project config (.zdx/config.yaml) if present; else fall back to
// ~/.zdx/config.yaml. Returns an error when no source is available.
//
// The cmd parameter is retained for future per-command flag reads (was used
// to read --global before that flag moved to `connect`-local).
func loadAgentRuntime(cmd *cobra.Command) (agentRuntime, error) {
	_ = cmd
	forceGlobal := config.IsGlobalMode()
	cfg := config.Load()

	if forceGlobal || cfg == nil {
		gc := config.LoadGlobal()
		if gc != nil {
			ga := gc.ResolvedGlobalAgent()
			return agentRuntime{
				RC:       remoteConfig{url: gc.Remote.URL, key: config.GlobalRemoteAPIKey()},
				AgentCfg: config.AgentConfig{ClaudeModel: ga.ClaudeModel, MaxWorktrees: ga.MaxWorktrees, LeaseMinutes: ga.LeaseMinutes},
				Srcless:  true,
				WorkDir:  ga.WorkDir,
			}, nil
		}
		if forceGlobal {
			return agentRuntime{}, fmt.Errorf("DX_GLOBAL set but ~/.zdx/config.yaml not found")
		}
	}

	if cfg == nil {
		return agentRuntime{}, fmt.Errorf("no project config found; run from a project root or set up ~/.zdx/config.yaml for srcless mode")
	}
	return agentRuntime{
		RC:       remoteConfig{url: cfg.RemoteURL(), slug: cfg.RemoteSlug(), key: config.RemoteAPIKey()},
		AgentCfg: cfg.ResolvedAgent(),
		LLMLocal: cfg.ResolvedLLMLocal(),
	}, nil
}

// runManagedFromFlags loads project + global config, resolves the model via
// the provider, and dispatches to RunManagedSession.
//
// The single-session path (dx agent --provider=…) was missed by e5ac0ad4 when
// it consolidated the legacy subcommands; enforceContainerExecution had only
// been threaded through dx agent loop's RunE, so DX_AGENT_FORCE_CONTAINER no
// longer covered single sessions even though spec 117 (every agent session
// runs in a container) applies to both. Apply the gate here too so the
// invariant holds across both entry points.
func runManagedFromFlags(ctx context.Context, cmd *cobra.Command, provider, alias, issue, model, complexity, mcpContainer string) error {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return err
	}
	opts, err := loadManagedOptsFromCmd(cmd, provider, alias, issue, model, tier, 0, mcpContainer)
	if err != nil {
		return err
	}
	if err := enforceContainerExecution(false); err != nil {
		return err
	}
	return RunManagedSession(ctx, provider, opts)
}

// agentSessionCmd groups session-scoped helpers used by shell wrappers to tag
// downstream dx calls with agent/session identifiers for server-side attribution.
func agentSessionCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "session", Short: "Agent session attribution helpers"}
	cmd.AddCommand(agentSessionBeginCmd())
	return cmd
}

func agentSessionBeginCmd() *cobra.Command {
	var agentID string
	cmd := &cobra.Command{
		Use:   "begin",
		Short: "Print export lines for ZDX_AGENT_ID and ZDX_SESSION_ID (use: eval $(dx agent session begin))",
		Long: `Emit shell export statements for ZDX_AGENT_ID and ZDX_SESSION_ID.

The CLI attaches these as X-ZDX-Agent-Id and X-ZDX-Session-Id headers on every
request, allowing the server to record which agent session made each status
change. Run this once at the start of an agent session:

  eval $(dx agent session begin --agent-id=abc123)
  dx agent claim --issue=IS-42   # all writes now carry session attribution

If --agent-id is omitted a random 8-char id is generated.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentID == "" {
				agentID = shortID()
			}
			sessionID := longSessionID()
			fmt.Printf("export ZDX_AGENT_ID=%s\n", agentID)
			fmt.Printf("export ZDX_SESSION_ID=%s\n", sessionID)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentID, "agent-id", "", "agent id (generated if empty)")
	return cmd
}

// longSessionID returns a 16-byte hex string suitable as a session identifier.
func longSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func shortID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func agentStartCmd() *cobra.Command {
	var issue, sessionID, taskGroup string
	var serverPort int32
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start a new agent (worktree + compose + register)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()
			cfg := config.Load()
			agentCfg := cfg.ResolvedAgent()

			// Check doctor prerequisites before committing to worktree/compose setup.
			state := &doctor.ProjectState{}
			doctor.DetectLocal(state)
			if pResp, err := c.GetProjectInfoWithResponse(cmd.Context(), &dxclient.GetProjectInfoParams{Slug: slug}); err == nil && pResp.JSON200 != nil {
				state.Classification = doctor.Classification(pResp.JSON200.Classification)
			}
			if agentCfg.LLMProvider == "claude" && !state.ClaudeInstalled {
				return fmt.Errorf("claude CLI not found in PATH; install it before using the claude agent provider")
			}
			if err := checkDockerRequirement(state, os.Stderr); err != nil {
				return err
			}

			// Check worktree slot availability.
			activeCount := countActiveWorktrees()
			if activeCount >= agentCfg.MaxWorktrees {
				return fmt.Errorf("no worktree slots available (%d/%d active)", activeCount, agentCfg.MaxWorktrees)
			}

			id := shortID()
			branchName := "agent/" + id
			if issue != "" {
				branchName += "/" + issue
			}

			repoRoot, err := cli.GitRepoRoot()
			if err != nil {
				return fmt.Errorf("not in a git repo: %w", err)
			}
			wtPath := filepath.Join(repoRoot, "agent", id)
			if issue != "" {
				wtPath = filepath.Join(repoRoot, "agent", id, issue)
			}

			if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
				return fmt.Errorf("mkdir: %w", err)
			}
			out, err := exec.Command("git", "worktree", "add", "-b", branchName, wtPath).CombinedOutput()
			if err != nil {
				return fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
			}
			fmt.Printf("worktree: %s (branch %s)\n", wtPath, branchName)

			composeProject := "zdx-agent-" + id

			// Use project-level compose file if it exists, otherwise generate default.
			composeFile := filepath.Join(wtPath, "docker-compose.agent.yaml")
			if projectCompose := agentCfg.ComposeFile; projectCompose != "" {
				if srcData, readErr := os.ReadFile(projectCompose); readErr == nil {
					// Project provides its own compose file — use it.
					if writeErr := os.WriteFile(composeFile, srcData, 0o644); writeErr != nil {
						return fmt.Errorf("write compose: %w", writeErr)
					}
				} else {
					// No project compose file — use built-in default.
					if writeErr := os.WriteFile(composeFile, []byte(defaultAgentCompose), 0o644); writeErr != nil {
						return fmt.Errorf("write compose: %w", writeErr)
					}
				}
			}

			out, err = exec.Command("docker", "compose", "-p", composeProject, "-f", composeFile, "up", "-d", "--wait").CombinedOutput()
			if err != nil {
				return fmt.Errorf("docker compose up: %s: %w", strings.TrimSpace(string(out)), err)
			}

			dbPort, err := discoverComposePort(composeProject, composeFile, "postgres", 5432)
			if err != nil {
				return fmt.Errorf("discover postgres port: %w", err)
			}
			dbURL := fmt.Sprintf("postgres://zdx:zdx@127.0.0.1:%d/zdx?sslmode=disable", dbPort)

			valkeyPort, err := discoverComposePort(composeProject, composeFile, "valkey", 6379)
			if err != nil {
				return fmt.Errorf("discover valkey port: %w", err)
			}
			valkeyURL := fmt.Sprintf("127.0.0.1:%d", valkeyPort)

			fmt.Printf("compose:  %s (db port %d, valkey port %d)\n", composeProject, dbPort, valkeyPort)

			resp, err := c.RegisterAgentWithResponse(cmd.Context(), dxclient.RegisterAgentRequest{
				Slug:           slug,
				Id:             id,
				SessionId:      sessionID,
				WorktreePath:   wtPath,
				WorktreeBranch: branchName,
				Pid:            int32(os.Getpid()),
				Status:         "active",
				TaskGroup:      taskGroup,
				ComposeProject: composeProject,
				ServerPort:     serverPort,
				DatabaseUrl:    dbURL,
				ValkeyUrl:      valkeyURL,
			})
			if err != nil {
				return fmt.Errorf("register: %w", err)
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return fmt.Errorf("register: %w", err)
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("register: empty response")
			}
			fmt.Printf("agent:    %s (registered)\n", resp.JSON200.Id)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID (e.g. IS-42)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session identifier")
	cmd.Flags().StringVar(&taskGroup, "task-group", "", "task group filter")
	cmd.Flags().Int32Var(&serverPort, "port", 0, "server port (auto-assigned if 0)")
	return cmd
}

// checkDockerRequirement returns an error for isolated classifications when
// Docker is unavailable, and emits a warning to stderr for non-isolated ones.
func checkDockerRequirement(state *doctor.ProjectState, stderr io.Writer) error {
	needsDocker := state.Classification == doctor.ClassService || state.Classification == doctor.ClassSaaS
	if !state.DockerAvailable {
		if needsDocker {
			return fmt.Errorf("docker daemon is not running; %s projects require docker for agent isolation", state.Classification)
		}
		fmt.Fprintf(stderr, "warning: docker not available — compose-based isolation will be unavailable\n")
	}
	return nil
}

func agentListCmd() *cobra.Command {
	var showTasks bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			ctx := cmd.Context()
			resp, err := c.ListAgentsWithResponse(ctx, &dxclient.ListAgentsParams{Slug: c.SlugOrDie()})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Agents == nil || len(*resp.JSON200.Agents) == 0 {
				fmt.Println("no agents")
				return nil
			}
			for _, a := range *resp.JSON200.Agents {
				fmt.Printf("%-10s %-12s %-8s pid=%-6d port=%-5d %s\n",
					a.Id, a.ConnectionState, a.Status, a.Pid, a.ServerPort, a.WorktreeBranch)
				if showTasks {
					taskResp, err := c.ListAgentTasksWithResponse(ctx, a.Id)
					if err != nil {
						fmt.Printf("  (tasks error: %v)\n", err)
						continue
					}
					if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
						fmt.Printf("  (tasks error: %v)\n", err)
						continue
					}
					if taskResp.JSON200 == nil || taskResp.JSON200.Tasks == nil {
						continue
					}
					for _, t := range *taskResp.JSON200.Tasks {
						fmt.Printf("  %-8s %-8s %s\n", t.Id, t.Status, cli.Truncate(t.Text, 60))
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&showTasks, "tasks", false, "also show claimed tasks per agent")
	return cmd
}

func agentStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <agent-id>",
		Short: "Gracefully stop an agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			ctx := cmd.Context()
			id := args[0]

			agentResp, err := c.GetAgentWithResponse(ctx, id)
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if err := c.CheckStatus(agentResp.StatusCode(), agentResp.Body); err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if agentResp.JSON200 == nil {
				return fmt.Errorf("get agent: empty response")
			}
			agent := *agentResp.JSON200

			taskResp, err := c.ListAgentTasksWithResponse(ctx, id)
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			if taskResp.JSON200 != nil && taskResp.JSON200.Tasks != nil {
				for _, t := range *taskResp.JSON200.Tasks {
					relResp, err := c.ReleaseTaskWithResponse(ctx, t.Id, dxclient.ReleaseTaskRequest{AgentId: id})
					if err != nil {
						fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.Id, err)
						continue
					}
					if err := c.CheckStatus(relResp.StatusCode(), relResp.Body); err != nil {
						fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.Id, err)
						continue
					}
					fmt.Printf("released task %s\n", t.Id)
				}
			}

			delResp, err := c.DeleteAgentWithResponse(ctx, id)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: delete agent: %v\n", err)
			} else if err := c.CheckStatus(delResp.StatusCode(), delResp.Body); err != nil {
				fmt.Fprintf(os.Stderr, "warn: delete agent: %v\n", err)
			} else {
				fmt.Printf("deleted agent %s\n", id)
			}

			if agent.ComposeProject != "" {
				out, err := exec.Command("docker", "compose", "-p", agent.ComposeProject, "down", "-v").CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: compose down: %s\n", strings.TrimSpace(string(out)))
				} else {
					fmt.Printf("compose down: %s\n", agent.ComposeProject)
				}
			}

			if agent.WorktreePath != "" {
				out, err := exec.Command("git", "worktree", "remove", "--force", agent.WorktreePath).CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: worktree remove: %s\n", strings.TrimSpace(string(out)))
				} else {
					fmt.Printf("worktree removed: %s\n", agent.WorktreePath)
				}
				if agent.WorktreeBranch != "" {
					_ = exec.Command("git", "branch", "-D", agent.WorktreeBranch).Run()
				}
			}

			return nil
		},
	}
}

func agentReconnectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reconnect <agent-id>",
		Short: "Reconnect a dead agent (re-register with new PID)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			ctx := cmd.Context()
			id := args[0]
			slug := c.SlugOrDie()

			agentResp, err := c.GetAgentWithResponse(ctx, id)
			if err != nil {
				return fmt.Errorf("agent %s not found (may have been reaped): %w", id, err)
			}
			if err := c.CheckStatus(agentResp.StatusCode(), agentResp.Body); err != nil {
				return fmt.Errorf("agent %s not found (may have been reaped): %w", id, err)
			}
			if agentResp.JSON200 == nil {
				return fmt.Errorf("agent %s not found", id)
			}
			agent := *agentResp.JSON200

			if agent.WorktreePath != "" {
				if _, err := os.Stat(agent.WorktreePath); err != nil {
					return fmt.Errorf("worktree %s does not exist — cannot resume", agent.WorktreePath)
				}
			}

			if agent.ComposeProject != "" {
				composeFile := filepath.Join(agent.WorktreePath, "docker-compose.agent.yaml")
				out, err := exec.Command("docker", "compose", "-p", agent.ComposeProject, "-f", composeFile, "up", "-d", "--wait").CombinedOutput()
				if err != nil {
					return fmt.Errorf("compose up: %s: %w", strings.TrimSpace(string(out)), err)
				}
				fmt.Printf("compose restarted: %s\n", agent.ComposeProject)

				dbPort, err := discoverComposePort(agent.ComposeProject, composeFile, "postgres", 5432)
				if err != nil {
					return fmt.Errorf("discover postgres port: %w", err)
				}
				agent.DatabaseUrl = fmt.Sprintf("postgres://zdx:zdx@127.0.0.1:%d/zdx?sslmode=disable", dbPort)

				valkeyPort, err := discoverComposePort(agent.ComposeProject, composeFile, "valkey", 6379)
				if err != nil {
					return fmt.Errorf("discover valkey port: %w", err)
				}
				agent.ValkeyUrl = fmt.Sprintf("127.0.0.1:%d", valkeyPort)
				fmt.Printf("ports:    db=%d valkey=%d\n", dbPort, valkeyPort)
			}

			regResp, err := c.RegisterAgentWithResponse(ctx, dxclient.RegisterAgentRequest{
				Slug:           slug,
				Id:             id,
				SessionId:      agent.SessionId,
				WorktreePath:   agent.WorktreePath,
				WorktreeBranch: agent.WorktreeBranch,
				Pid:            int32(os.Getpid()),
				Status:         "active",
				TaskGroup:      agent.TaskGroup,
				ComposeProject: agent.ComposeProject,
				ServerPort:     agent.ServerPort,
				DatabaseUrl:    agent.DatabaseUrl,
				ValkeyUrl:      agent.ValkeyUrl,
			})
			if err != nil {
				return fmt.Errorf("re-register: %w", err)
			}
			if err := c.CheckStatus(regResp.StatusCode(), regResp.Body); err != nil {
				return fmt.Errorf("re-register: %w", err)
			}
			fmt.Printf("resumed agent %s (new pid=%d)\n", id, os.Getpid())
			return nil
		},
	}
}

func agentReleaseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "release <agent-id>",
		Short: "Release agent resources (compose, worktree, tasks) without deleting DB record",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			ctx := cmd.Context()
			id := args[0]

			agentResp, err := c.GetAgentWithResponse(ctx, id)
			if err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if err := c.CheckStatus(agentResp.StatusCode(), agentResp.Body); err != nil {
				return fmt.Errorf("get agent: %w", err)
			}
			if agentResp.JSON200 == nil {
				return fmt.Errorf("get agent: empty response")
			}
			agent := *agentResp.JSON200

			taskResp, err := c.ListAgentTasksWithResponse(ctx, id)
			if err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			if err := c.CheckStatus(taskResp.StatusCode(), taskResp.Body); err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			if taskResp.JSON200 != nil && taskResp.JSON200.Tasks != nil {
				for _, t := range *taskResp.JSON200.Tasks {
					relResp, err := c.ReleaseTaskWithResponse(ctx, t.Id, dxclient.ReleaseTaskRequest{AgentId: id})
					if err != nil {
						fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.Id, err)
						continue
					}
					if err := c.CheckStatus(relResp.StatusCode(), relResp.Body); err != nil {
						fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.Id, err)
						continue
					}
					fmt.Printf("released task %s\n", t.Id)
				}
			}

			if agent.ComposeProject != "" {
				out, err := exec.Command("docker", "compose", "-p", agent.ComposeProject, "down", "-v").CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: compose down: %s\n", strings.TrimSpace(string(out)))
				} else {
					fmt.Printf("compose down: %s\n", agent.ComposeProject)
				}
			}

			if agent.WorktreePath != "" {
				out, err := exec.Command("git", "worktree", "remove", "--force", agent.WorktreePath).CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: worktree remove: %s\n", strings.TrimSpace(string(out)))
				} else {
					fmt.Printf("worktree removed: %s\n", agent.WorktreePath)
				}
				if agent.WorktreeBranch != "" {
					_ = exec.Command("git", "branch", "-D", agent.WorktreeBranch).Run()
				}
			}

			return nil
		},
	}
}

func agentPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <agent-id>",
		Short: "Pause a running agent (holds task lease, no new LLM turns)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sendAgentControl(cmd.Context(), cmd.OutOrStdout(), cli.MustClient(), args[0], "pause", "paused")
		},
	}
}

func agentResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <agent-id>",
		Short: "Resume a paused agent (re-enters task loop with existing session)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sendAgentControl(cmd.Context(), cmd.OutOrStdout(), cli.MustClient(), args[0], "resume", "resumed")
		},
	}
}

func agentDrainCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "drain <agent-id>",
		Short: "Drain an agent (finishes current task, releases lease, disconnects)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return sendAgentControl(cmd.Context(), cmd.OutOrStdout(), cli.MustClient(), args[0], "drain", "draining")
		},
	}
}

// sendAgentControl issues a WS control command (pause/resume/drain) for an
// agent via the server's send-command endpoint. On success it writes
// "<verb> agent <id>" to out. A 404 is rewritten as the operator-friendly
// "agent <id> not connected" hint so the caller does not see the raw HTTP
// status string.
func sendAgentControl(ctx context.Context, out io.Writer, c *cli.Client, id, command, successVerb string) error {
	resp, err := c.SendAgentCommandWithResponse(ctx, id, dxclient.SendAgentCommandRequest{Command: command})
	if err != nil {
		return err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return fmt.Errorf("agent %s not connected — server cannot deliver command", id)
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return err
	}
	fmt.Fprintf(out, "%s agent %s\n", successVerb, id)
	return nil
}

const defaultAgentCompose = `services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: zdx
      POSTGRES_USER: zdx
      POSTGRES_PASSWORD: zdx
    ports:
      - "127.0.0.1::5432"
    tmpfs:
      - /var/lib/postgresql/data:exec
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U zdx -d zdx"]
      interval: 1s
      timeout: 3s
      retries: 30
  valkey:
    image: valkey/valkey:8
    ports:
      - "127.0.0.1::6379"
    healthcheck:
      test: ["CMD-SHELL", "valkey-cli ping | grep -q PONG"]
      interval: 1s
      timeout: 3s
      retries: 30
`

// countActiveWorktrees counts git worktrees under agent/.
func countActiveWorktrees() int {
	out, err := exec.Command("git", "worktree", "list", "--porcelain").CombinedOutput()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") && strings.Contains(line, "/agent/") {
			count++
		}
	}
	return count
}

// ── Budget subcommand tree ────────────────────────────────────────────────

func agentBudgetCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "budget", Short: "Manage per-agent and per-project spend ceilings"}
	cmd.AddCommand(agentBudgetSetCmd(), agentBudgetShowCmd(), agentBudgetListCmd(), agentBudgetLiftCmd())
	return cmd
}

func agentBudgetSetCmd() *cobra.Command {
	var agentID string
	var projectID int32
	var tokens int64
	var cost float64
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set token/cost ceiling for an agent or project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentID == "" && projectID == 0 {
				return fmt.Errorf("provide --agent or --project")
			}
			if agentID != "" && projectID != 0 {
				return fmt.Errorf("provide only one of --agent or --project")
			}
			c := cli.MustClient()
			req := dxclient.SetAgentBudgetRequest{
				AgentId:      agentID,
				ProjectId:    projectID,
				TokenCeiling: tokens,
				CostCeiling:  cost,
			}
			resp, err := c.SetAgentBudgetWithResponse(cmd.Context(), req)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			b := resp.JSON200
			fmt.Printf("budget set: id=%d agent=%s project=%d tokens=%d cost=%.4f\n",
				b.Id, b.AgentId, b.ProjectId, b.TokenCeiling, b.CostCeiling)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentID, "agent", "", "agent ID")
	cmd.Flags().Int32Var(&projectID, "project", 0, "project ID")
	cmd.Flags().Int64Var(&tokens, "tokens", 0, "token ceiling (0 = unlimited)")
	cmd.Flags().Float64Var(&cost, "cost", 0, "cost ceiling in USD (0 = unlimited)")
	return cmd
}

func agentBudgetShowCmd() *cobra.Command {
	var agentID string
	var projectID int32
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show budget ceiling and current usage for an agent or project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if agentID == "" && projectID == 0 {
				return fmt.Errorf("provide --agent or --project")
			}
			c := cli.MustClient()
			params := &dxclient.GetAgentBudgetParams{}
			if agentID != "" {
				params.AgentId = &agentID
			}
			if projectID != 0 {
				params.ProjectId = &projectID
			}
			resp, err := c.GetAgentBudgetWithResponse(cmd.Context(), params)
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			b := resp.JSON200
			pause := ""
			if b.ActivePause {
				pause = " [PAUSED]"
			}
			fmt.Printf("budget id=%d%s\n", b.Id, pause)
			if b.AgentId != "" {
				fmt.Printf("  agent:   %s\n", b.AgentId)
			}
			if b.ProjectId != 0 {
				fmt.Printf("  project: %d\n", b.ProjectId)
			}
			fmt.Printf("  tokens:  %d / %d\n", b.TokensUsed, b.TokenCeiling)
			fmt.Printf("  cost:    $%.4f / $%.4f\n", b.CostUsed, b.CostCeiling)
			return nil
		},
	}
	cmd.Flags().StringVar(&agentID, "agent", "", "agent ID")
	cmd.Flags().Int32Var(&projectID, "project", 0, "project ID")
	return cmd
}

func agentBudgetListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all budget ceilings with current usage",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ListAgentBudgetsWithResponse(cmd.Context())
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Budgets == nil || len(*resp.JSON200.Budgets) == 0 {
				fmt.Println("no budgets set")
				return nil
			}
			fmt.Printf("%-6s %-14s %-10s %14s %14s %10s %8s\n",
				"ID", "AGENT/PROJECT", "SCOPE", "TOKENS USED", "TOKEN CEIL", "COST USED", "PAUSED")
			for _, b := range *resp.JSON200.Budgets {
				scope := "agent"
				who := b.AgentId
				if who == "" {
					scope = "project"
					who = fmt.Sprintf("%d", b.ProjectId)
				}
				paused := ""
				if b.ActivePause {
					paused = "yes"
				}
				fmt.Printf("%-6d %-14s %-10s %14d %14d %10.4f %8s\n",
					b.Id, cli.Truncate(who, 14), scope, b.TokensUsed, b.TokenCeiling, b.CostUsed, paused)
			}
			return nil
		},
	}
}

func agentBudgetLiftCmd() *cobra.Command {
	var liftedBy string
	cmd := &cobra.Command{
		Use:   "lift <agent-id>",
		Short: "Lift an active budget pause and resume the agent",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			req := dxclient.LiftAgentBudgetPauseRequest{LiftedBy: liftedBy}
			resp, err := c.LiftAgentBudgetPauseWithResponse(cmd.Context(), args[0], req)
			if err != nil {
				return err
			}
			if resp.StatusCode() == http.StatusNotFound {
				return fmt.Errorf("agent %s has no active budget pause", args[0])
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil {
				return fmt.Errorf("empty response")
			}
			fmt.Println(resp.JSON200.Message)
			return nil
		},
	}
	cmd.Flags().StringVar(&liftedBy, "by", "", "identity of the approver (defaults to 'operator')")
	return cmd
}

func discoverComposePort(project, composeFile, service string, containerPort int) (int, error) {
	out, err := exec.Command("docker", "compose", "-p", project, "-f", composeFile, "port", service, strconv.Itoa(containerPort)).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("docker compose port %s %d: %s: %w", service, containerPort, strings.TrimSpace(string(out)), err)
	}
	hostPort := strings.TrimSpace(string(out))
	parts := strings.Split(hostPort, ":")
	if len(parts) != 2 {
		return 0, fmt.Errorf("unexpected port output: %q", hostPort)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, fmt.Errorf("parse port %q: %w", parts[1], err)
	}
	return port, nil
}
