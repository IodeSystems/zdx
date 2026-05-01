package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/cli/agent"
)

func main() {
	server := flag.String("server", "http://localhost:7600", "dx-server base URL")
	agentID := flag.String("agent-id", "", "agent identifier (default: hostname-pid)")
	worktree := flag.String("worktree", ".", "worktree path")
	apiKey := flag.String("api-key", "", "API key (env ZDX_API_KEY)")
	flag.Parse()

	if *apiKey == "" {
		*apiKey = os.Getenv("ZDX_API_KEY")
	}
	if *apiKey == "" {
		log.Fatal("--api-key or ZDX_API_KEY required")
	}

	if *agentID == "" {
		hostname, _ := os.Hostname()
		*agentID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	hostname, _ := os.Hostname()
	branch := gitBranch(*worktree)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("dx-agent starting: id=%s server=%s", *agentID, *server)

	ctrlCh := make(chan agentdaemon.ControlMsg, 16)
	d := &agentdaemon.Daemon{
		ServerURL:      *server,
		AgentID:        *agentID,
		APIKey:         *apiKey,
		WorktreePath:   *worktree,
		WorktreeBranch: branch,
		Hostname:       hostname,
		Pid:            int32(os.Getpid()),
		Capabilities:   []string{"claude", "local"},
		ControlCh:      ctrlCh,
		// No-op TaskHolder until IS-602 wires in the real session manager.
		Holder: agentdaemon.NoopHolder(),
	}

	// Consume control signals. Resume spawns a fresh Take goroutine using the
	// session/issue captured at pause time; pause is informational only (the
	// daemon's hold loop already prevents new LLM turns).
	go func() {
		for msg := range ctrlCh {
			switch msg.Type {
			case "pause":
				log.Printf("pause signal: session=%s issue=%s", msg.SessionID, msg.IssueID)
			case "resume":
				log.Printf("resume signal: session=%s issue=%s — spawning Take", msg.SessionID, msg.IssueID)
				go func(m agentdaemon.ControlMsg) {
					res := agent.RunResume(ctx, agent.ResumeRunnerConfig{
						ServerURL: *server,
						APIKey:    *apiKey,
						AgentID:   *agentID,
						WorkDir:   *worktree,
						SessionID: m.SessionID,
						IssueID:   m.IssueID,
						LogFn:     func(format string, a ...any) { log.Printf(format, a...) },
					})
					if res.Err != nil {
						log.Printf("resume failed: %v", res.Err)
					} else {
						log.Printf("resume completed: session=%s success=%v", m.SessionID, res.Success)
					}
				}(msg)
			}
		}
	}()

	if err := d.RunForever(ctx); err != nil {
		log.Fatalf("daemon: %v", err)
	}
	close(ctrlCh)
}

// gitBranch returns the current branch of the given worktree, or "unknown" on error.
func gitBranch(worktree string) string {
	out, err := exec.Command("git", "-C", worktree, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
