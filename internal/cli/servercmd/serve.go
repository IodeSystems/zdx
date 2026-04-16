package servercmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

// ServeCmd starts the backend daemon (if not running) and the vite dev server.
// Ctrl-C stops both.
func ServeCmd() *cobra.Command {
	var serverPort, pgPort int
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run backend daemon + vite dev server for local development",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			zdxDir := filepath.Join(home, ".zdx")

			// Start daemon if not already running.
			if pid := readPidFile(filepath.Join(zdxDir, "daemon.pid")); pid <= 0 || !processAlive(pid) {
				start := daemonStartCmd()
				start.Flags().Set("port", fmt.Sprintf("%d", serverPort)) //nolint:errcheck
				start.Flags().Set("pg-port", fmt.Sprintf("%d", pgPort))  //nolint:errcheck
				if err := start.RunE(start, nil); err != nil {
					return fmt.Errorf("start daemon: %w", err)
				}
			} else {
				fmt.Printf("daemon already running (pid %d)\n", pid)
			}

			// Find vite. Must be run from the ui/ directory.
			uiDir := "ui"
			if _, err := os.Stat(uiDir); err != nil {
				return fmt.Errorf("ui/ directory not found — run from project root")
			}

			fmt.Println("starting vite dev server (http://localhost:7610) ...")
			vite := exec.Command("npx", "vite")
			vite.Dir = uiDir
			vite.Stdout = os.Stdout
			vite.Stderr = os.Stderr
			vite.Env = os.Environ()
			if err := vite.Start(); err != nil {
				return fmt.Errorf("vite: %w", err)
			}

			// Wait for Ctrl-C, then stop vite (daemon keeps running).
			ch := make(chan os.Signal, 1)
			signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
			<-ch
			fmt.Println("\nstopping vite...")
			_ = vite.Process.Signal(syscall.SIGTERM)
			_ = vite.Wait()

			port := strings.TrimSpace(func() string {
				b, _ := os.ReadFile(filepath.Join(zdxDir, "daemon.port"))
				return string(b)
			}())
			fmt.Printf("daemon still running on port %s — stop it with: dx daemon stop\n", port)
			return nil
		},
	}
	cmd.Flags().IntVar(&serverPort, "port", 7600, "dx-server port")
	cmd.Flags().IntVar(&pgPort, "pg-port", 7601, "postgres port")
	return cmd
}
