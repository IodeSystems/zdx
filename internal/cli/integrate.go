package cli

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func IntegrateCmd() *cobra.Command {
	var serverURL, slug, apiKey string
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

			// Validate API key by hitting an authenticated endpoint.
			if apiKey != "" {
				client := &Client{base: serverURL, token: apiKey, http: &http.Client{}}
				var projects struct {
					Items []struct{} `json:"items"`
				}
				if err := client.get("/api/projects", nil, &projects); err != nil {
					return fmt.Errorf("API key validation failed: %w", err)
				}
				fmt.Println("api key: valid")
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
				client, err := DefaultClient()
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
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key for authentication")
	cmd.Flags().BoolVar(&bootstrap, "bootstrap", false, "create onboarding issue with project setup questions")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}
