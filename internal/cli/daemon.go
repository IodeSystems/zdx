package cli

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		Short: "Start local dx-server (auto-installs pgsqlite)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			zdxDir := filepath.Join(home, ".zdx")
			if err := os.MkdirAll(filepath.Join(zdxDir, "bin"), 0700); err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Join(zdxDir, "data"), 0700); err != nil {
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

			// Ensure pgsqlite.
			pgsqliteBin, err := ensurePgsqlite(filepath.Join(zdxDir, "bin"))
			if err != nil {
				fmt.Fprintf(os.Stderr, "warn: pgsqlite unavailable (%v) — DATABASE_URL must be set externally\n", err)
				pgsqliteBin = ""
			}

			dbURL := os.Getenv("DATABASE_URL")

			// Start pgsqlite if we have the binary and no external DB.
			if pgsqliteBin != "" && dbURL == "" {
				dbName := "zdx"
				dataDir := filepath.Join(zdxDir, "data")
				pgPid, err := startPgsqlite(pgsqliteBin, dataDir, dbName, pgPort, filepath.Join(zdxDir, "pgsqlite.log"))
				if err != nil {
					return fmt.Errorf("pgsqlite: %w", err)
				}
				_ = os.WriteFile(filepath.Join(zdxDir, "pgsqlite.pid"), []byte(strconv.Itoa(pgPid)), 0600)
				dbURL = fmt.Sprintf("postgres://localhost:%d/%s?sslmode=disable", pgPort, dbName)
				fmt.Printf("pgsqlite started (pid %d, port %d)\n", pgPid, pgPort)
			}

			if dbURL == "" {
				return fmt.Errorf("no DATABASE_URL and pgsqlite unavailable — set DATABASE_URL or run 'dx daemon start' after installing pgsqlite")
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
			deadline := time.Now().Add(8 * time.Second)
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
	cmd.Flags().IntVar(&pgPort, "pg-port", 7651, "pgsqlite postgres port")
	return cmd
}

// ── stop ──────────────────────────────────────────────────────────────────────

func daemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop daemon (and pgsqlite if managed)",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			zdxDir := filepath.Join(home, ".zdx")

			stopped := false
			for _, name := range []string{"daemon.pid", "pgsqlite.pid"} {
				pid := readPidFile(filepath.Join(zdxDir, name))
				if pid <= 0 || !processAlive(pid) {
					continue
				}
				proc, _ := os.FindProcess(pid)
				_ = proc.Signal(syscall.SIGTERM)
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				for {
					select {
					case <-ctx.Done():
						_ = proc.Signal(syscall.SIGKILL)
					default:
						if !processAlive(pid) {
							goto done
						}
						time.Sleep(100 * time.Millisecond)
					}
				}
			done:
				cancel()
				_ = os.Remove(filepath.Join(zdxDir, name))
				fmt.Printf("stopped pid %d (%s)\n", pid, strings.TrimSuffix(name, ".pid"))
				stopped = true
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
			portBytes, _ := os.ReadFile(filepath.Join(zdxDir, "daemon.port"))
			port := strings.TrimSpace(string(portBytes))
			pgPid := readPidFile(filepath.Join(zdxDir, "pgsqlite.pid"))

			if pid <= 0 || !processAlive(pid) {
				fmt.Println("daemon: not running")
				return nil
			}
			fmt.Printf("daemon:    running  pid=%-6d port=%s  url=http://localhost:%s\n", pid, port, port)
			if pgPid > 0 && processAlive(pgPid) {
				fmt.Printf("pgsqlite:  running  pid=%d\n", pgPid)
			}
			return nil
		},
	}
}

// ── pgsqlite ──────────────────────────────────────────────────────────────────

const pgsqliteURL = "https://github.com/erans/pgsqlite/releases/latest/download/pgsqlite-%s-%s.tar.gz"

func ensurePgsqlite(binDir string) (string, error) {
	binPath := filepath.Join(binDir, "pgsqlite")
	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	goos := runtime.GOOS     // linux | darwin
	arch := runtime.GOARCH   // amd64 | arm64
	// Map Go arch to pgsqlite release naming
	osName := goos
	archName := arch
	if arch == "amd64" {
		archName = "x64"
	}

	url := fmt.Sprintf(pgsqliteURL, osName, archName)
	tarball := filepath.Join(binDir, "pgsqlite.tar.gz")

	fmt.Fprintf(os.Stderr, "downloading pgsqlite from %s\n", url)
	dlCmd := exec.Command("curl", "-fsSL", "-o", tarball, url)
	dlCmd.Stdout = os.Stderr
	dlCmd.Stderr = os.Stderr
	if err := dlCmd.Run(); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(tarball)

	tarCmd := exec.Command("tar", "-xzf", tarball, "-C", binDir, "pgsqlite")
	tarCmd.Stderr = os.Stderr
	if err := tarCmd.Run(); err != nil {
		return "", fmt.Errorf("extract failed: %w", err)
	}
	if err := os.Chmod(binPath, 0755); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "pgsqlite installed at %s\n", binPath)
	return binPath, nil
}

func startPgsqlite(bin, dataDir, dbName string, port int, logPath string) (int, error) {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return 0, err
	}

	c := exec.Command(bin,
		"--port", strconv.Itoa(port),
		"--data-dir", dataDir,
		"--db-name", dbName,
	)
	c.Stdout = logFile
	c.Stderr = logFile
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		logFile.Close()
		return 0, err
	}
	logFile.Close()

	// Wait for pg to accept connections.
	deadline := time.Now().Add(5 * time.Second)
	dsn := fmt.Sprintf("postgres://localhost:%d/%s?sslmode=disable", port, dbName)
	for time.Now().Before(deadline) {
		out, _ := exec.Command("pg_isready", "-h", "localhost", "-p", strconv.Itoa(port)).Output()
		if strings.Contains(string(out), "accepting") {
			return c.Process.Pid, nil
		}
		// Also try a simple TCP connect
		conn, err := exec.Command("sh", "-c",
			fmt.Sprintf("echo > /dev/tcp/localhost/%d", port)).Output()
		_ = conn
		if err == nil {
			return c.Process.Pid, nil
		}
		_ = dsn
		time.Sleep(200 * time.Millisecond)
	}
	return c.Process.Pid, nil // return pid even on timeout; server may still come up
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
