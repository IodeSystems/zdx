package servercmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/cli/project"
	"github.com/iodesystems/zdx-go/internal/config"
)

func InitCmd() *cobra.Command {
	var bootstrap bool
	var useLocal bool
	var remoteURL, slug, email, name string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create .zdx/ scaffold",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := scaffoldZdx(); err != nil {
				return err
			}

			connected := detectExistingConnection()

			if !connected && useLocal {
				var err error
				connected, err = initStartLocal(slug, email, name)
				if err != nil {
					return err
				}
			} else if !connected && remoteURL != "" {
				var err error
				connected, err = initConnectRemote(remoteURL, slug)
				if err != nil {
					return err
				}
			}

			if !connected {
				fmt.Println("\nno server configured. options:")
				fmt.Println("  dx init --local --slug=<name> --email=<email> --name=<name>")
				fmt.Println("    start a local zdx server with docker")
				fmt.Println("  dx integrate --url=<server-url>")
				fmt.Println("    connect to an existing zdx server")
			}

			if bootstrap && connected {
				c, err := cli.DefaultClient()
				if err != nil {
					fmt.Fprintf(os.Stderr, "bootstrap: cannot create client: %v\n", err)
				} else if err := bootstrapOnboardingIssue(c); err != nil {
					fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
				}
			}

			// Run doctor to diagnose and guide the project to a working state.
			fmt.Println("\nrunning project health check...")
			doctorCmd := project.DoctorCmd()
			doctorCmd.SetArgs([]string{"--fix"})
			if err := doctorCmd.Execute(); err != nil {
				fmt.Fprintf(os.Stderr, "doctor: %v\n", err)
			}

			return nil
		},
	}
	cmd.Flags().BoolVar(&bootstrap, "bootstrap", false, "create onboarding issue with project setup questions")
	cmd.Flags().BoolVar(&useLocal, "local", false, "start a local zdx server via docker")
	cmd.Flags().StringVar(&remoteURL, "url", "", "connect to an existing zdx server URL")
	cmd.Flags().StringVar(&slug, "slug", "", "project slug (defaults to current directory name)")
	cmd.Flags().StringVar(&email, "email", "", "admin email (required with --local)")
	cmd.Flags().StringVar(&name, "name", "", "admin display name (required with --local)")
	return cmd
}

func scaffoldZdx() error {
	if err := os.MkdirAll(".zdx", 0755); err != nil {
		return err
	}
	if _, err := os.Stat(".git"); err == nil {
		if _, cfgErr := os.Stat(".zdx/config.yaml"); os.IsNotExist(cfgErr) {
			fmt.Println("tip: this directory has a git repo — use Admin UI > Projects > Add Existing to bind it to zdx.")
		}
	}
	if _, err := os.Stat(".zdx/config.yaml"); os.IsNotExist(err) {
		const skeleton = `# zdx project configuration

# remote:
#   url: https://your-zdx-server.example.com
#   slug: your-project

hooks:
  pre-commit:
    - dx lint
    - dx build

components:
  app:
    build:
      steps:
        - name: compile
          run: go build ./...
    test:
      unit:
        runner: basic
        run: go test ./...
    lint:
      external:
        - name: vet
          run: go vet ./...
`
		if err := os.WriteFile(".zdx/config.yaml", []byte(skeleton), 0644); err != nil {
			return err
		}
		fmt.Println("created .zdx/config.yaml")
	} else {
		fmt.Println(".zdx/config.yaml already exists")
	}

	ignoreEntries := []string{
		".zdx/context",
		".zdx/credentials",
		".zdx/test-results.json",
		".zdx/test-index.json",
		".zdx/smoke.json",
		".zdx/metrics-latest.yaml",
	}
	gitignore, _ := os.ReadFile(".gitignore")
	var added []string
	for _, e := range ignoreEntries {
		if !strings.Contains(string(gitignore), e) {
			added = append(added, e)
		}
	}
	if len(added) > 0 {
		f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			for _, e := range added {
				fmt.Fprintln(f, e)
			}
			f.Close()
			fmt.Printf("added %d entries to .gitignore\n", len(added))
		}
	}
	fmt.Println("✓ .zdx/ initialized")
	return nil
}

// detectExistingConnection checks if a server connection is already configured.
func detectExistingConnection() bool {
	cfg := config.Load()
	if cfg != nil && cfg.RemoteURL() != "" {
		fmt.Printf("server:  %s (configured)\n", cfg.RemoteURL())
		return true
	}
	conn := config.ReadDaemonConn()
	if conn != nil {
		resp, err := http.Get(conn.URL + "/api/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				fmt.Printf("server:  %s (local daemon)\n", conn.URL)
				return true
			}
		}
	}
	return false
}

// initStartLocal starts a local daemon and runs setup.
func initStartLocal(slug, email, name string) (bool, error) {
	if email == "" || name == "" {
		return false, fmt.Errorf("--email and --name are required with --local")
	}

	if slug == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return false, err
		}
		slug = filepath.Base(cwd)
	}

	fmt.Println("starting local zdx server...")
	startCmd := daemonStartCmd()
	if err := startCmd.RunE(startCmd, nil); err != nil {
		return false, fmt.Errorf("daemon start: %w", err)
	}

	conn := config.ReadDaemonConn()
	if conn == nil {
		return false, fmt.Errorf("daemon started but connection info not found")
	}

	fmt.Println("bootstrapping project...")
	setupCmd := SetupCmd()
	setupCmd.SetArgs([]string{
		"--url", conn.URL,
		"--slug", slug,
		"--email", email,
		"--name", name,
	})
	if err := setupCmd.Execute(); err != nil {
		return false, fmt.Errorf("setup: %w", err)
	}

	return true, nil
}

// initConnectRemote validates and connects to a remote server.
func initConnectRemote(serverURL, slug string) (bool, error) {
	serverURL = strings.TrimRight(serverURL, "/")

	if slug == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return false, err
		}
		slug = filepath.Base(cwd)
	}

	fmt.Printf("connecting to %s ...\n", serverURL)
	resp, err := http.Get(serverURL + "/api/health")
	if err != nil {
		return false, fmt.Errorf("cannot reach server: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("server returned HTTP %d on /api/health", resp.StatusCode)
	}

	writeRemoteConfig(serverURL, slug)
	fmt.Printf("remote:  %s (slug: %s)\n", serverURL, slug)
	fmt.Println("note: set credentials with DX_REMOTE_API_KEY or write to .zdx/credentials")
	return true, nil
}

// RunInit kept for compatibility.
func RunInit(_ []string) {}
