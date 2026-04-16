package servercmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func DaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage local dx-server daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return daemonStatusCmd().RunE(cmd, args)
		},
	}
	cmd.AddCommand(daemonStartCmd(), daemonStopCmd(), daemonStatusCmd())
	return cmd
}

// ── start ─────────────────────────────────────────────────────────────────────

func daemonStartCmd() *cobra.Command {
	var serverPort, pgPort int
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start local dx-server (uses docker-compose for postgres)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			zdxDir := filepath.Join(home, ".zdx")
			if err := os.MkdirAll(zdxDir, 0700); err != nil {
				return err
			}

			// Already running?
			if pid := readPidFile(filepath.Join(zdxDir, "daemon.pid")); pid > 0 {
				if processAlive(pid) {
					fmt.Printf("daemon already running (pid %d)\n", pid)
					return nil
				}
				fmt.Printf("stale pid %d — restarting\n", pid)
			}

			// Ensure token.
			token, err := ensureToken(filepath.Join(zdxDir, "daemon.token"))
			if err != nil {
				return fmt.Errorf("token: %w", err)
			}

			dbURL := os.Getenv("DATABASE_URL")

			// If no external DATABASE_URL, start postgres via docker-compose.
			if dbURL == "" {
				composeFile := devComposeFile()
				if err := startDevPostgres(composeFile, pgPort); err != nil {
					return fmt.Errorf("postgres: %w", err)
				}
				dbURL = fmt.Sprintf("postgres://zdx:zdx@localhost:%d/zdx?sslmode=disable", pgPort)
				fmt.Printf("postgres started (port %d)\n", pgPort)
				// Store compose file path for stop command.
				_ = os.WriteFile(filepath.Join(zdxDir, "daemon.compose"), []byte(composeFile), 0600)
			}

			// Locate dx-server binary.
			serverBin, err := resolveSiblingBin("dx-server")
			if err != nil {
				return err
			}

			logPath := filepath.Join(zdxDir, "daemon.log")
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				return err
			}

			c := exec.Command(serverBin)
			c.Env = append(os.Environ(),
				"DATABASE_URL="+dbURL,
				fmt.Sprintf("PORT=%d", serverPort),
				"DX_TOKEN="+token,
			)
			c.Stdout = logFile
			c.Stderr = logFile
			c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
			if err := c.Start(); err != nil {
				logFile.Close()
				return fmt.Errorf("dx-server: %w", err)
			}
			logFile.Close()

			_ = os.WriteFile(filepath.Join(zdxDir, "daemon.pid"), []byte(strconv.Itoa(c.Process.Pid)), 0600)
			_ = os.WriteFile(filepath.Join(zdxDir, "daemon.port"), []byte(strconv.Itoa(serverPort)), 0600)

			// Wait for health.
			addr := fmt.Sprintf("http://localhost:%d/health", serverPort)
			deadline := time.Now().Add(15 * time.Second)
			for time.Now().Before(deadline) {
				resp, err := http.Get(addr)
				if err == nil && resp.StatusCode == 200 {
					resp.Body.Close()
					fmt.Printf("daemon started (pid %d, port %d)\n", c.Process.Pid, serverPort)
					fmt.Printf("token: %s...\n", token[:8])
					return nil
				}
				time.Sleep(150 * time.Millisecond)
			}
			fmt.Printf("daemon started (pid %d) — health check timed out, check %s\n", c.Process.Pid, logPath)
			return nil
		},
	}
	cmd.Flags().IntVar(&serverPort, "port", 7600, "dx-server port")
	cmd.Flags().IntVar(&pgPort, "pg-port", 7601, "postgres port")
	return cmd
}

// ── stop ──────────────────────────────────────────────────────────────────────

func daemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop daemon (and docker-compose postgres if managed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			zdxDir := filepath.Join(home, ".zdx")

			stopped := false
			pid := readPidFile(filepath.Join(zdxDir, "daemon.pid"))
			if pid > 0 && processAlive(pid) {
				proc, _ := os.FindProcess(pid)
				_ = proc.Signal(syscall.SIGTERM)
				stopped = true
				fmt.Printf("stopped daemon (pid %d)\n", pid)
			}
			_ = os.Remove(filepath.Join(zdxDir, "daemon.pid"))

			// Stop docker-compose postgres if we started it.
			if b, err := os.ReadFile(filepath.Join(zdxDir, "daemon.compose")); err == nil {
				composeFile := strings.TrimSpace(string(b))
				if composeFile != "" {
					out, err := exec.Command("docker", "compose", "-f", composeFile, "down", "--timeout", "5").CombinedOutput()
					if err != nil {
						fmt.Fprintf(os.Stderr, "docker compose down: %s\n", out)
					} else {
						fmt.Println("postgres stopped")
					}
					_ = os.Remove(filepath.Join(zdxDir, "daemon.compose"))
				}
			}

			if !stopped {
				fmt.Println("daemon not running")
			}
			return nil
		},
	}
}

// ── status ────────────────────────────────────────────────────────────────────

func daemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			zdxDir := filepath.Join(home, ".zdx")

			pid := readPidFile(filepath.Join(zdxDir, "daemon.pid"))
			if pid > 0 && processAlive(pid) {
				port := strings.TrimSpace(func() string {
					b, _ := os.ReadFile(filepath.Join(zdxDir, "daemon.port"))
					return string(b)
				}())
				fmt.Printf("daemon:  running  pid=%d  port=%s\n", pid, port)
			} else {
				fmt.Println("daemon: not running")
			}
			return nil
		},
	}
}

// ── docker-compose postgres ───────────────────────────────────────────────────

// devComposeFile returns the path to the embedded dev compose file.
// It writes it to a temp location so it's available to docker-compose.
func devComposeFile() string {
	content := `services:
  postgres:
    image: postgres:17
    environment:
      POSTGRES_DB: zdx
      POSTGRES_USER: zdx
      POSTGRES_PASSWORD: zdx
    tmpfs:
      - /var/lib/postgresql/data:exec
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U zdx -d zdx"]
      interval: 1s
      timeout: 3s
      retries: 30
`
	tmp, _ := os.CreateTemp("", "zdx-dev-compose-*.yaml")
	_, _ = tmp.WriteString(content)
	_ = tmp.Close()
	return tmp.Name()
}

func startDevPostgres(composeFile string, port int) error {
	// Write a dynamic compose override for the port.
	override := fmt.Sprintf(`services:
  postgres:
    ports:
      - "127.0.0.1:%d:5432"
`, port)
	overrideFile, _ := os.CreateTemp("", "zdx-dev-compose-override-*.yaml")
	_, _ = overrideFile.WriteString(override)
	_ = overrideFile.Close()
	defer os.Remove(overrideFile.Name())

	out, err := exec.Command("docker", "compose",
		"-f", composeFile,
		"-f", overrideFile.Name(),
		"up", "-d", "--wait",
	).CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func ensureToken(tokenPath string) (string, error) {
	if b, err := os.ReadFile(tokenPath); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	return token, os.WriteFile(tokenPath, []byte(token+"\n"), 0600)
}

func resolveSiblingBin(name string) (string, error) {
	self, _ := os.Executable()
	sibling := filepath.Join(filepath.Dir(self), name)
	if _, err := os.Stat(sibling); err == nil {
		return sibling, nil
	}
	if p, err := exec.LookPath(name); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("%s not found in PATH or alongside dx", name)
}

func readPidFile(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}

func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// RunDaemon kept for compatibility.
func RunDaemon(_ []string) {}
