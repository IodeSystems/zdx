package servercmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func SetupCmd() *cobra.Command {
	var serverURL, slug, projectName, email, name, keyName string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Bootstrap a fresh server: create project, first user, and API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			serverURL = strings.TrimRight(serverURL, "/")
			if projectName == "" {
				projectName = slug
			}

			// Unauthenticated client pointed at the given URL (no token).
			bare := cli.NewClient(serverURL, "")

			// 1. Create project (also unauthenticated during bootstrap —
			//    middleware skips /api/setup/bootstrap, but /api/project still
			//    requires auth. So create project AFTER we have a token.)

			// 1. Bootstrap first: create admin user + API key
			var boot struct {
				Token string `json:"token"`
				Email string `json:"email"`
			}
			bootBody := dxclient.SetupBootstrapRequest{
				Email: email,
				Name:  name,
			}
			if keyName != "" {
				bootBody.KeyName = &keyName
			}
			if err := bare.Post("/api/setup/bootstrap", bootBody, &boot); err != nil {
				return fmt.Errorf("bootstrap: %w", err)
			}
			fmt.Printf("user:  %s\n", boot.Email)

			// 2. Write .zdx/credentials so authed calls work
			if err := os.MkdirAll(".zdx", 0755); err != nil {
				return err
			}
			if err := os.WriteFile(".zdx/credentials", []byte(boot.Token+"\n"), 0600); err != nil {
				return fmt.Errorf("write credentials: %w", err)
			}

			// 3. Write .zdx/config.yaml remote section
			writeRemoteConfig(serverURL, slug)

			// 4. Create project with auth
			authed := cli.NewClient(serverURL, boot.Token)
			var proj struct {
				ID   int32  `json:"id"`
				Slug string `json:"slug"`
				Name string `json:"name"`
			}
			if err := authed.Post("/api/project", dxclient.CreateProjectRequest{
				Slug: slug,
				Name: projectName,
			}, &proj); err != nil {
				return fmt.Errorf("create project: %w", err)
			}
			fmt.Printf("project: %s (%s)\n", proj.Name, proj.Slug)

			fmt.Printf("token:   saved to .zdx/credentials\n")
			fmt.Printf("ready:   dx todo solo\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&serverURL, "url", "", "server base URL (e.g. https://zdx.example.com)")
	cmd.Flags().StringVar(&slug, "slug", "", "project slug")
	cmd.Flags().StringVar(&projectName, "project-name", "", "project display name (defaults to slug)")
	cmd.Flags().StringVar(&email, "email", "", "admin user email")
	cmd.Flags().StringVar(&name, "name", "", "admin user display name")
	cmd.Flags().StringVar(&keyName, "key-name", "default", "API key label")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("slug")
	_ = cmd.MarkFlagRequired("email")
	_ = cmd.MarkFlagRequired("name")
	return cmd
}

// writeRemoteConfig writes/updates the remote section of .zdx/config.yaml.
func writeRemoteConfig(serverURL, slug string) {
	existing, _ := os.ReadFile(".zdx/config.yaml")
	content := string(existing)

	newRemote := "remote:\n  url: " + serverURL + "\n  slug: " + slug

	if strings.Contains(content, "remote:") {
		lines := strings.Split(content, "\n")
		var out []string
		inRemote := false
		injected := false
		for _, line := range lines {
			if strings.HasPrefix(line, "remote:") {
				if !injected {
					out = append(out, newRemote)
					injected = true
				}
				inRemote = true
				continue
			}
			if inRemote {
				if strings.HasPrefix(line, "  ") || line == "" {
					continue
				}
				inRemote = false
			}
			out = append(out, line)
		}
		_ = os.WriteFile(".zdx/config.yaml", []byte(strings.Join(out, "\n")), 0644)
	} else if content != "" {
		_ = os.WriteFile(".zdx/config.yaml", []byte(newRemote+"\n\n"+content), 0644)
	} else {
		_ = os.WriteFile(".zdx/config.yaml", []byte(newRemote+"\n"), 0644)
	}
}
