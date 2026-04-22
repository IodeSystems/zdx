package servercmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func IntegrateCmd() *cobra.Command {
	var serverURL, slug, projectName, apiKey string
	var bootstrap bool
	cmd := &cobra.Command{
		Use:   "integrate",
		Short: "Connect this project to an existing zdx server",
		Long:  "Validate server connectivity, write remote config and credentials so dx commands work against an existing zdx instance.",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL = strings.TrimRight(serverURL, "/")

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

			// Validate server connectivity.
			fmt.Printf("connecting to %s ...\n", serverURL)
			resp, err := http.Get(serverURL + "/api/health")
			if err != nil {
				return fmt.Errorf("cannot reach server: %w", err)
			}
			resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("server returned HTTP %d on /api/health", resp.StatusCode)
			}
			fmt.Println("server: ok")

			// Validate API key and create the project on the remote server.
			if apiKey != "" {
				client := cli.NewClient(serverURL, apiKey)
				var listResp struct {
					Projects []struct {
						Slug string `json:"slug"`
					} `json:"projects"`
				}
				if err := client.Get("/api/projects", nil, &listResp); err != nil {
					return fmt.Errorf("API key validation failed: %w", err)
				}
				fmt.Println("api key: valid")

				exists := false
				for _, p := range listResp.Projects {
					if p.Slug == slug {
						exists = true
						break
					}
				}
				if exists {
					fmt.Printf("project: %s (already exists)\n", slug)
				} else {
					var proj struct {
						Slug string `json:"slug"`
						Name string `json:"name"`
					}
					if err := client.Post("/api/project", dxclient.CreateProjectRequest{Slug: slug, Name: projectName}, &proj); err != nil {
						fmt.Fprintf(os.Stderr, "project: create failed: %v\n", err)
					} else {
						fmt.Printf("project: %s created\n", proj.Slug)
					}
				}
			}

			// Ensure .zdx/ exists.
			if err := os.MkdirAll(".zdx", 0755); err != nil {
				return err
			}

			// Write remote config.
			writeRemoteConfig(serverURL, slug)
			fmt.Printf("remote:  %s (slug: %s)\n", serverURL, slug)

			// Write credentials if provided.
			if apiKey != "" {
				if err := os.WriteFile(".zdx/credentials", []byte(apiKey+"\n"), 0600); err != nil {
					return fmt.Errorf("write credentials: %w", err)
				}
				fmt.Println("credentials: saved")
			} else {
				fmt.Println("credentials: skipped (use --api-key or set DX_REMOTE_API_KEY)")
			}

			// Optionally bootstrap onboarding issue.
			if bootstrap {
				client, err := cli.DefaultClient()
				if err != nil {
					fmt.Fprintf(os.Stderr, "bootstrap: cannot create client: %v\n", err)
				} else if err := bootstrapOnboardingIssue(client); err != nil {
					fmt.Fprintf(os.Stderr, "bootstrap: %v\n", err)
				}
			}

			fmt.Printf("ready:   dx todo solo\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "url", "", "zdx server base URL (e.g. https://zdx.example.com)")
	cmd.Flags().StringVar(&slug, "slug", "", "project slug (defaults to current directory name)")
	cmd.Flags().StringVar(&projectName, "project-name", "", "project display name (defaults to slug)")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&bootstrap, "bootstrap", false, "create onboarding issue with project setup questions")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}
