package cli

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func AgentCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "agent", Short: "Agent lifecycle management"}
	cmd.AddCommand(agentStartCmd(), agentListCmd(), agentStopCmd(), agentReapCmd(), agentResumeCmd(), agentReleaseCmd())
	return cmd
}

type agentItem struct {
	ID             string `json:"id"`
	ProjectID      int32  `json:"project_id"`
	SessionID      string `json:"session_id"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	Pid            int32  `json:"pid"`
	Status         string `json:"status"`
	TaskGroup      string `json:"task_group"`
	ComposeProject string `json:"compose_project"`
	ServerPort     int32  `json:"server_port"`
	DatabaseUrl    string `json:"database_url"`
	LastHeartbeat  string `json:"last_heartbeat"`
	CreatedAt      string `json:"created_at"`
}

type agentTaskItem struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Feature        string `json:"feature"`
	Status         string `json:"status"`
	Issue          string `json:"issue"`
	TaskGroup      string `json:"task_group"`
	ClaimedAt      string `json:"claimed_at"`
	LeaseExpiresAt string `json:"lease_expires_at"`
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
			c := mustClient()
			slug := c.SlugOrDie()

			id := shortID()
			branchName := "agent/" + id
			if issue != "" {
				branchName += "/" + issue
			}

			repoRoot, err := gitRepoRoot()
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
			if serverPort == 0 {
				serverPort = 7600 + int32(os.Getpid()%1000)
			}
			dbPort := 5432 + int32(os.Getpid()%1000)
			dbURL := fmt.Sprintf("postgres://zdx:zdx@127.0.0.1:%d/zdx?sslmode=disable", dbPort)

			composeContent := fmt.Sprintf(`services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: zdx
      POSTGRES_USER: zdx
      POSTGRES_PASSWORD: zdx
    ports:
      - "127.0.0.1:%d:5432"
    tmpfs:
      - /var/lib/postgresql/data:exec
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U zdx -d zdx"]
      interval: 1s
      timeout: 3s
      retries: 30
`, dbPort)
			composeFile := filepath.Join(wtPath, "docker-compose.agent.yaml")
			if err := os.WriteFile(composeFile, []byte(composeContent), 0o644); err != nil {
				return fmt.Errorf("write compose: %w", err)
			}

			out, err = exec.Command("docker", "compose", "-p", composeProject, "-f", composeFile, "up", "-d", "--wait").CombinedOutput()
			if err != nil {
				return fmt.Errorf("docker compose up: %s: %w", strings.TrimSpace(string(out)), err)
			}
			fmt.Printf("compose:  %s (db port %d)\n", composeProject, dbPort)

			var agent agentItem
			if err := c.post("/api/agents/register", map[string]any{
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
			c := mustClient()
			var resp struct {
				Agents []agentItem `json:"agents"`
			}
			if err := c.get("/api/agents/list", url.Values{"slug": {c.SlugOrDie()}}, &resp); err != nil {
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
						Tasks []agentTaskItem `json:"tasks"`
					}
					if err := c.get("/api/agents/"+a.ID+"/tasks", nil, &taskResp); err != nil {
						fmt.Printf("  (tasks error: %v)\n", err)
						continue
					}
					for _, t := range taskResp.Tasks {
						fmt.Printf("  %-8s %-8s %s\n", t.ID, t.Status, truncate(t.Text, 60))
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
			c := mustClient()
			id := args[0]

			var agent agentItem
			if err := c.get("/api/agents/"+id, nil, &agent); err != nil {
				return fmt.Errorf("get agent: %w", err)
			}

			var taskResp struct {
				Tasks []agentTaskItem `json:"tasks"`
			}
			if err := c.get("/api/agents/"+id+"/tasks", nil, &taskResp); err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			for _, t := range taskResp.Tasks {
				if err := c.post("/api/tasks/"+t.ID+"/release", map[string]any{"agent_id": id}, nil); err != nil {
					fmt.Fprintf(os.Stderr, "warn: release %s: %v\n", t.ID, err)
				} else {
					fmt.Printf("released task %s\n", t.ID)
				}
			}

			if err := c.doJSON("DELETE", "/api/agents/"+id, nil, nil); err != nil {
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
			c := mustClient()
			var resp struct {
				Reaped []agentItem `json:"reaped"`
			}
			if err := c.post("/api/agents/reap", map[string]any{"threshold_minutes": thresholdMin}, &resp); err != nil {
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
			c := mustClient()
			id := args[0]
			slug := c.SlugOrDie()

			var agent agentItem
			if err := c.get("/api/agents/"+id, nil, &agent); err != nil {
				return fmt.Errorf("agent %s not found (may have been reaped): %w", id, err)
			}

			if agent.WorktreePath != "" {
				if _, err := os.Stat(agent.WorktreePath); err != nil {
					return fmt.Errorf("worktree %s does not exist — cannot resume", agent.WorktreePath)
				}
			}

			if agent.ComposeProject != "" {
				out, err := exec.Command("docker", "compose", "-p", agent.ComposeProject, "up", "-d", "--wait").CombinedOutput()
				if err != nil {
					return fmt.Errorf("compose up: %s: %w", strings.TrimSpace(string(out)), err)
				}
				fmt.Printf("compose restarted: %s\n", agent.ComposeProject)
			}

			var updated agentItem
			if err := c.post("/api/agents/register", map[string]any{
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
			c := mustClient()
			id := args[0]

			var agent agentItem
			if err := c.get("/api/agents/"+id, nil, &agent); err != nil {
				return fmt.Errorf("get agent: %w", err)
			}

			var taskResp struct {
				Tasks []agentTaskItem `json:"tasks"`
			}
			if err := c.get("/api/agents/"+id+"/tasks", nil, &taskResp); err != nil {
				return fmt.Errorf("list tasks: %w", err)
			}
			for _, t := range taskResp.Tasks {
				if err := c.post("/api/tasks/"+t.ID+"/release", map[string]any{"agent_id": id}, nil); err != nil {
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

func gitRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// heartbeatLoop sends heartbeats until the stop channel is closed.
func heartbeatLoop(c *Client, agentID string, interval time.Duration, stop <-chan struct{}) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			_ = c.post("/api/agents/"+agentID+"/heartbeat", nil, nil)
		}
	}
}
