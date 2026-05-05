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

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/doctor"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func AgentCmd() *cobra.Command {
	var provider, alias, issue, model, complexity string
	cmd := &cobra.Command{
		Use:   "agent",
		Short: "Run a single agent session (or use a subcommand for legacy flows)",
		Long: `Run an agent session against the configured project. The provider is
selected via --provider; complexity-to-model resolution is delegated to the
provider, so dx agent --provider=claude --complexity=high picks claude-opus-4-7,
while --provider=opencode --complexity=high picks the project's configured
high-tier model from admin/llm-configs.

Legacy single-purpose subcommands (dx agent claude, opencode, local) remain
available with their full feature surface (--container, --chrome, --max-turns,
etc). The unified ` + "`dx agent`" + ` form covers the common single-session path
end-to-end through the new provider registry.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// When invoked with no flags, show usage rather than mint tokens.
			if provider == "" && len(args) == 0 {
				return cmd.Help()
			}
			return runManagedFromFlags(cmd.Context(), cmd, provider, alias, issue, model, complexity)
		},
	}
	cmd.PersistentFlags().Bool("global", false, "force srcless mode using ~/.zdx/config.yaml instead of project config")
	cmd.Flags().StringVar(&provider, "provider", "", "agent provider: "+providerNames()+" (required for managed run)")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().StringVar(&model, "model", "", "explicit model name (overrides --complexity)")
	cmd.Flags().StringVar(&complexity, "complexity", DefaultComplexity, "model tier: low|medium|high (resolved by the provider)")
	cmd.AddCommand(agentClaudeCmd(), agentLocalCmd(), agentOpenCodeCmd(), agentLoopCmd(), agentStartCmd(), agentListCmd(), agentStopCmd(), agentReapCmd(), agentReconnectCmd(), agentReleaseCmd(), agentSessionCmd(), agentPauseCmd(), agentResumeCmd(), agentDrainCmd(), agentBudgetCmd())
	return cmd
}

// agentLoopCmd is the loop equivalent of dx agent: polls dx todo solo, runs a
// managed session per pick, repeats. Shares the same provider registry +
// manager scaffolding as the single-session form.
func agentLoopCmd() *cobra.Command {
	var provider, alias, model, complexity string
	var maxTurns int
	cmd := &cobra.Command{
		Use:   "loop",
		Short: "Loop: poll dx todo solo, run a managed session per pick, repeat",
		Long: `Long-running loop that claims work via dx todo solo and runs a managed
agent session per pick, sleeping 60s between idle ticks. Same provider
registry + complexity resolution as ` + "`dx agent`" + `; the loop wraps
RunManagedSession with signal handling and state-file checkpointing.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tier, err := NormalizeComplexity(complexity)
			if err != nil {
				return err
			}
			opts, err := loadManagedOptsFromCmd(cmd, provider, alias, "", model, tier, maxTurns)
			if err != nil {
				return err
			}
			return RunManagedLoop(cmd.Context(), provider, opts)
		},
	}
	cmd.Flags().StringVar(&provider, "provider", "", "agent provider: "+providerNames()+" (required)")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&model, "model", "", "explicit model name (overrides --complexity)")
	cmd.Flags().StringVar(&complexity, "complexity", DefaultComplexity, "model tier: low|medium|high (resolved by the provider)")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 0, "cap on assistant turns per session (0 = unlimited; opencode/local only)")
	return cmd
}

// loadManagedOptsFromCmd centralizes the project-config + global-mode +
// model-resolution dance so both `dx agent` and `dx agent loop` (and the
// upcoming legacy shims) build a populated ProviderOpts the same way.
func loadManagedOptsFromCmd(cmd *cobra.Command, provider, alias, issue, model, tier string, maxTurns int) (ProviderOpts, error) {
	global, _ := cmd.Flags().GetBool("global")
	global = global || config.IsGlobalMode()

	var rc remoteConfig
	var agentCfg config.AgentConfig
	var llmLocal config.LLMLocal
	var srcless bool
	if global {
		gc := config.LoadGlobal()
		if gc != nil {
			ga := gc.ResolvedGlobalAgent()
			rc = remoteConfig{url: gc.Remote.URL, key: config.GlobalRemoteAPIKey()}
			agentCfg = config.AgentConfig{ClaudeModel: ga.ClaudeModel, MaxWorktrees: ga.MaxWorktrees, LeaseMinutes: ga.LeaseMinutes}
			srcless = true
		}
	} else {
		cfg := config.Load()
		if cfg == nil {
			return ProviderOpts{}, fmt.Errorf("no project config found; run from a project root or pass --global")
		}
		rc = remoteConfig{url: cfg.RemoteURL(), slug: cfg.RemoteSlug(), key: config.RemoteAPIKey()}
		agentCfg = cfg.ResolvedAgent()
		llmLocal = cfg.ResolvedLLMLocal()
	}

	resolved := model
	if resolved == "" {
		ctor, err := LookupProvider(provider)
		if err != nil {
			return ProviderOpts{}, err
		}
		ap, err := ctor(ProviderOpts{RC: rc, AgentCfg: agentCfg, LLMLocal: llmLocal})
		if err != nil {
			return ProviderOpts{}, fmt.Errorf("resolve model: %w", err)
		}
		resolved, err = ap.ResolveModel(cmd.Context(), tier)
		if err != nil {
			return ProviderOpts{}, fmt.Errorf("resolve model (%s, complexity=%s): %w", provider, tier, err)
		}
	}

	return ProviderOpts{
		RC:         rc,
		AgentCfg:   agentCfg,
		LLMLocal:   llmLocal,
		IssueID:    issue,
		Alias:      alias,
		Model:      resolved,
		Complexity: tier,
		Srcless:    srcless,
		Chrome:     true,
		MaxTurns:   maxTurns,
	}, nil
}

// runManagedFromFlags loads project + global config, resolves the model via
// the provider, and dispatches to RunManagedSession.
func runManagedFromFlags(ctx context.Context, cmd *cobra.Command, provider, alias, issue, model, complexity string) error {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return err
	}
	opts, err := loadManagedOptsFromCmd(cmd, provider, alias, issue, model, tier, 0)
	if err != nil {
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
  dx todo solo --issue=IS-42   # all writes now carry session attribution

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

func agentReapCmd() *cobra.Command {
	var thresholdMin int32
	cmd := &cobra.Command{
		Use:   "reap",
		Short: "Reap stale agents (heartbeat expired)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			resp, err := c.ReapAgentsWithResponse(cmd.Context(), dxclient.ReapAgentsRequest{ThresholdMinutes: thresholdMin})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Reaped == nil || len(*resp.JSON200.Reaped) == 0 {
				fmt.Println("no stale agents")
				return nil
			}
			for _, a := range *resp.JSON200.Reaped {
				fmt.Printf("reaped %s (pid=%d, last heartbeat %s)\n", a.Id, a.Pid, a.LastHeartbeat)
			}
			return nil
		},
	}
	cmd.Flags().Int32Var(&thresholdMin, "threshold", 5, "stale threshold in minutes")
	return cmd
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
