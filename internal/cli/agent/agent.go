package agent

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

func AgentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Agent lifecycle management"}
	cmd.AddCommand(agentClaudeCmd(), agentLocalCmd(), agentStartCmd(), agentListCmd(), agentStopCmd(), agentReapCmd(), agentResumeCmd(), agentReleaseCmd(), agentSessionCmd())
	return cmd
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

			composeContent := `services:
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
			composeFile := filepath.Join(wtPath, "docker-compose.agent.yaml")
			if err := os.WriteFile(composeFile, []byte(composeContent), 0o644); err != nil {
				return fmt.Errorf("write compose: %w", err)
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

			var agent cli.AgentItem
			if err := c.Post("/api/agents/register", map[string]any{
				"slug":            slug,
				"id":              id,
				"session_id":      sessionID,
				"worktree_path":   wtPath,
				"worktree_branch": branchName,
				"pid":             os.Getpid(),
				"status":          "active",
				"task_group":      taskGroup,
				"compose_project": composeProject,
				"server_port":     serverPort,
				"database_url":    dbURL,
				"valkey_url":      valkeyURL,
			}, &agent); err != nil {
				return fmt.Errorf("register: %w", err)
			}
			fmt.Printf("agent:    %s (registered)\n", agent.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&issue, "issue", "", "issue ID (e.g. IS-42)")
	cmd.Flags().StringVar(&sessionID, "session", "", "session identifier")
	cmd.Flags().StringVar(&taskGroup, "task-group", "", "task group filter")
	cmd.Flags().Int32Var(&serverPort, "port", 0, "server port (auto-assigned if 0)")
	return cmd
}

func agentListCmd() *cobra.Command {
	var showTasks bool
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			var resp struct {
				Agents []cli.AgentItem `json:"agents"`
			}
			if err := c.Get("/api/agents/list", url.Values{"slug": {c.SlugOrDie()}}, &resp); err != nil {
				return err
			}
			if len(resp.Agents) == 0 {
				fmt.Println("no agents")
				return nil
			}
			for _, a := range resp.Agents {
				fmt.Printf("%-10s %-8s pid=%-6d port=%-5d %s  %s\n",
					a.ID, a.Status, a.Pid, a.ServerPort, a.WorktreeBranch, a.LastHeartbeat)
				if showTasks {
					var taskResp struct {
						Tasks []cli.AgentTaskItem `json:"tasks"`
					}
					if err := c.Get("/api/agents/"+a.ID+"/tasks", nil, &taskResp); err != nil {
						fmt.Printf("  (tasks error: %v)\n", err)
						continue
					}
					for _, t := range taskResp.Tasks {
						fmt.Printf("  %-8s %-8s %s\n", t.ID, t.Status, cli.Truncate(t.Text, 60))
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
			id := args[0]

			var agent cli.AgentItem
			if err := c.Get("/api/agents/"+id, nil, &agent); err != nil {
				return fmt.Errorf("get agent: %w", err)
			}

			var taskResp struct {
				Tasks []cli.AgentTaskItem `json:"tasks"`
			}
			if err := c.Get("/api/agents/"+id+"/tasks", nil, &taskResp); err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			for _, t := range taskResp.Tasks {
				if err := c.Post("/api/tasks/"+t.ID+"/release", map[string]any{"agent_id": id}, nil); err != nil {
					fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.ID, err)
				} else {
					fmt.Printf("released task %s\n", t.ID)
				}
			}

			if err := c.Delete("/api/agents/"+id, nil, nil); err != nil {
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
			var resp struct {
				Reaped []cli.AgentItem `json:"reaped"`
			}
			if err := c.Post("/api/agents/reap", map[string]any{"threshold_minutes": thresholdMin}, &resp); err != nil {
				return err
			}
			if len(resp.Reaped) == 0 {
				fmt.Println("no stale agents")
				return nil
			}
			for _, a := range resp.Reaped {
				fmt.Printf("reaped %s (pid=%d, last heartbeat %s)\n", a.ID, a.Pid, a.LastHeartbeat)
			}
			return nil
		},
	}
	cmd.Flags().Int32Var(&thresholdMin, "threshold", 5, "stale threshold in minutes")
	return cmd
}

func agentResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "resume <agent-id>",
		Short: "Resume a dead agent (re-register with new PID)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			id := args[0]
			slug := c.SlugOrDie()

			var agent cli.AgentItem
			if err := c.Get("/api/agents/"+id, nil, &agent); err != nil {
				return fmt.Errorf("agent %s not found (may have been reaped): %w", id, err)
			}

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

			var updated cli.AgentItem
			if err := c.Post("/api/agents/register", map[string]any{
				"slug":            slug,
				"id":              id,
				"session_id":      agent.SessionID,
				"worktree_path":   agent.WorktreePath,
				"worktree_branch": agent.WorktreeBranch,
				"pid":             os.Getpid(),
				"status":          "active",
				"task_group":      agent.TaskGroup,
				"compose_project": agent.ComposeProject,
				"server_port":     agent.ServerPort,
				"database_url":    agent.DatabaseUrl,
				"valkey_url":      agent.ValkeyUrl,
			}, &updated); err != nil {
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
			id := args[0]

			var agent cli.AgentItem
			if err := c.Get("/api/agents/"+id, nil, &agent); err != nil {
				return fmt.Errorf("get agent: %w", err)
			}

			var taskResp struct {
				Tasks []cli.AgentTaskItem `json:"tasks"`
			}
			if err := c.Get("/api/agents/"+id+"/tasks", nil, &taskResp); err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			for _, t := range taskResp.Tasks {
				if err := c.Post("/api/tasks/"+t.ID+"/release", map[string]any{"agent_id": id}, nil); err != nil {
					fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.ID, err)
				} else {
					fmt.Printf("released task %s\n", t.ID)
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
