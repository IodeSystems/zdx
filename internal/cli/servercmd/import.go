package servercmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ImportCmd() *cobra.Command {
	var serverURL, slug, projectName, apiKey string
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Create and connect an existing directory as a zdx project",
		Long:  "Create a project on a running zdx server and write .zdx/config.yaml. Works with or without git.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if slug == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				slug = filepath.Base(cwd)
			}
			if projectName == "" {
				projectName = slug
			}

			// Resolve server URL: flag → existing config → running daemon.
			if serverURL == "" {
				cfg := config.Load()
				serverURL = cfg.RemoteURL()
			}
			if serverURL == "" {
				conn := config.ReadDaemonConn()
				if conn != nil {
					resp, err := http.Get(conn.URL + "/api/health")
					if err == nil {
						resp.Body.Close()
						if resp.StatusCode == 200 {
							serverURL = conn.URL
							if apiKey == "" {
								apiKey = conn.Token
							}
						}
					}
				}
			}
			if serverURL == "" {
				return fmt.Errorf("no server found — pass --url, or start a local daemon with: dx daemon start")
			}
			serverURL = strings.TrimRight(serverURL, "/")

			// Resolve API key: flag → env → .zdx/credentials → ~/.zdx/credentials.
			if apiKey == "" {
				if v := os.Getenv("DX_REMOTE_API_KEY"); v != "" {
					apiKey = strings.TrimSpace(v)
				}
			}
			if apiKey == "" {
				if b, err := os.ReadFile(".zdx/credentials"); err == nil {
					apiKey = strings.TrimSpace(string(b))
				}
			}
			if apiKey == "" {
				if v := config.GlobalRemoteAPIKey(); v != "" {
					apiKey = v
				}
			}
			if apiKey == "" {
				return fmt.Errorf("no API key — pass --api-key, set DX_REMOTE_API_KEY, or write to .zdx/credentials or ~/.zdx/credentials")
			}

			// Ensure .zdx/ exists before writing config.
			if err := os.MkdirAll(".zdx", 0755); err != nil {
				return err
			}

			// Create the project via typed client.
			fmt.Printf("connecting to %s ...\n", serverURL)
			c := cli.NewClient(serverURL, apiKey)
			resp, err := c.CreateProjectWithResponse(cmd.Context(), dxclient.CreateProjectRequest{
				Slug: slug,
				Name: projectName,
			})
			if err != nil {
				return fmt.Errorf("create project: %w", err)
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			proj := resp.JSON200
			fmt.Printf("project: %s (%s)\n", proj.Name, proj.Slug)

			// Write remote config.
			writeRemoteConfig(serverURL, slug)
			fmt.Printf("remote:  %s (slug: %s)\n", serverURL, slug)

			// Persist credentials if .zdx/credentials doesn't already exist.
			if _, err := os.Stat(".zdx/credentials"); os.IsNotExist(err) {
				if err := os.WriteFile(".zdx/credentials", []byte(apiKey+"\n"), 0600); err != nil {
					return fmt.Errorf("write credentials: %w", err)
				}
				fmt.Println("credentials: saved to .zdx/credentials")
			}

			// Idempotently ensure .zdx/credentials is in .gitignore so naive
			// `git add .` doesn't commit the API key.
			gitignore, _ := os.ReadFile(".gitignore")
			if !strings.Contains(string(gitignore), ".zdx/credentials") {
				f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err == nil {
					fmt.Fprintln(f, ".zdx/credentials")
					f.Close()
					fmt.Println("gitignore: added .zdx/credentials")
				}
			}

			fmt.Println("ready:   dx todo queue")
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "url", "", "zdx server base URL (e.g. https://zdx.example.com)")
	cmd.Flags().StringVar(&slug, "slug", "", "project slug (defaults to current directory name)")
	cmd.Flags().StringVar(&projectName, "project-name", "", "project display name (defaults to slug)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	return cmd
}
