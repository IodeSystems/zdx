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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/cli/agent"
	"github.com/iodesystems/zdx-go/internal/cli/mcpcmd"
)

func main() {
	server := flag.String("server", "http://localhost:7600", "dx-server base URL")
	agentID := flag.String("agent-id", "", "agent identifier (default: hostname-pid)")
	worktree := flag.String("worktree", ".", "worktree path")
	apiKey := flag.String("api-key", "", "API key (env ZDX_API_KEY)")
	mcpStdio := flag.Bool("mcp-stdio", false, "serve fs+shell MCP tools over stdin/stdout (dev-container mode); skips the WS daemon")
	mcpRoot := flag.String("mcp-root", "/workspace", "filesystem root tools operate against in --mcp-stdio mode")
	flag.Parse()

	if *mcpStdio {
		if err := runMCPStdio(*mcpRoot); err != nil {
			log.Fatalf("dx-agent --mcp-stdio: %v", err)
		}
		return
	}

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

// runMCPStdio is the dev-container mode: register the same fs+shell MCP
// tools the in-process providers use, then serve them over stdin/stdout
// using the MCP StdioTransport. Host-side opencode/local providers
// connect by spawning `docker exec -i <container> dx-agent --mcp-stdio`
// and using mcp.CommandTransport. The host runs the LLM loop; this
// process executes the tool calls against /workspace inside the container.
//
// Skips the WS daemon entirely — this mode is short-lived (lifecycle =
// the spawning client's lifecycle) and runs in a sandboxed container, so
// the daemon's pause/resume surface doesn't apply.
func runMCPStdio(root string) error {
	if root == "" {
		return fmt.Errorf("--mcp-root must not be empty")
	}
	if _, err := os.Stat(root); err != nil {
		return fmt.Errorf("mcp-root %q: %w", root, err)
	}

	srv := mcp.NewServer(&mcp.Implementation{
		Name:    "dx-agent",
		Version: "0.1.0",
	}, nil)
	mcpcmd.RegisterFSTools(srv, root)
	mcpcmd.RegisterShellTools(srv, root)
	mcpcmd.RegisterOutlineTools(srv, root)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// StdioTransport reads/writes the calling process's stdin/stdout. The
	// MCP framing is newline-delimited JSON; no auth, no port, no daemon.
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("mcp serve: %w", err)
	}
	return nil
}

// gitBranch returns the current branch of the given worktree, or "unknown" on error.
func gitBranch(worktree string) string {
	out, err := exec.Command("git", "-C", worktree, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
