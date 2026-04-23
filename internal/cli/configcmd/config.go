package configcmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ConfigCmd() *cobra.Command {
	var clearScope string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show or manage dx configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clearScope != "" {
				return runClear(clearScope)
			}
			return runShow()
		},
	}
	cmd.Flags().StringVar(&clearScope, "clear", "", "clear context: local, global, or all")
	cmd.AddCommand(configInitCmd(), configShowCmd(), configRegisterCmd())
	return cmd
}

func runClear(scope string) error {
	switch scope {
	case "local", "all", "true":
		_ = config.WriteContext("", "")
		_ = os.Remove(".zdx/credentials")
		fmt.Println("local context cleared")
		if scope == "local" {
			return nil
		}
		fallthrough
	case "global":
		home, _ := os.UserHomeDir()
		_ = os.Remove(filepath.Join(home, ".zdx", "credentials"))
		_ = os.Remove(filepath.Join(home, ".zdx", "config.yaml"))
		fmt.Println("global context cleared")
	default:
		return fmt.Errorf("--clear: invalid scope %q (use local, global, or all)", scope)
	}
	return nil
}

func runShow() error {
	global := config.LoadGlobal()
	project := config.Load()

	// connection section — resolved effective client
	fmt.Println("connection:")
	effectiveURL, effectiveSrc := resolveEffectiveURL(global, project)
	if effectiveURL != "" {
		fmt.Printf("  %-24s %s  [%s]\n", "url", effectiveURL, effectiveSrc)
	} else {
		fmt.Printf("  %-24s (not configured)\n", "url")
	}
	effectiveCreds, effectiveCredsSrc := resolveEffectiveCreds(effectiveSrc)
	if effectiveCreds != "" {
		fmt.Printf("  %-24s %s  [%s]\n", "credentials", effectiveCreds, effectiveCredsSrc)
	} else {
		fmt.Printf("  %-24s none\n", "credentials")
	}
	comp := config.ActiveComponent("")
	if comp == "" {
		comp = "(all)"
	}
	fmt.Printf("  %-24s %s\n", "component", comp)

	source := func(label, globalVal, projectVal, envKey string) {
		val := globalVal
		src := "global"
		if envKey != "" {
			if v := os.Getenv(envKey); v != "" {
				val = v
				src = "env"
			}
		}
		if projectVal != "" {
			val = projectVal
			src = "project"
		}
		if val == "" {
			fmt.Printf("  %-24s (not set)\n", label)
		} else {
			fmt.Printf("  %-24s %s  [%s]\n", label, val, src)
		}
	}

	fmt.Println("remote:")
	globalURL := ""
	globalAPIKey := config.GlobalRemoteAPIKey()
	if global != nil {
		globalURL = global.Remote.URL
	}
	projectURL := ""
	projectSlug := ""
	projectAPIKey := config.RemoteAPIKey()
	if project != nil {
		projectURL = project.Remote.URL
		projectSlug = project.Remote.Slug
	}
	source("url", globalURL, projectURL, "DX_REMOTE_URL")
	source("api_key", mask(globalAPIKey), mask(projectAPIKey), "DX_REMOTE_API_KEY")
	if projectSlug != "" || os.Getenv("DX_REMOTE_SLUG") != "" {
		source("slug", "", projectSlug, "DX_REMOTE_SLUG")
	} else {
		fmt.Printf("  %-24s (none — srcless mode)\n", "slug")
	}

	fmt.Println("agent:")
	var ga config.GlobalAgentConfig
	if global != nil {
		ga = global.ResolvedGlobalAgent()
	}
	var pa config.AgentConfig
	if project != nil {
		pa = project.ResolvedAgent()
	}

	if global != nil {
		workDir := ga.WorkDir
		fmt.Printf("  %-24s %s  [global]\n", "work_dir", workDir)
	} else {
		fmt.Printf("  %-24s (no global config — run dx config init)\n", "work_dir")
	}

	modelGlobal := ga.ClaudeModel
	modelProject := pa.ClaudeModel
	source("claude_model", modelGlobal, modelProject, "")

	maxWGlobal := strconv.Itoa(ga.MaxWorktrees)
	maxWProject := ""
	if project != nil && pa.MaxWorktrees > 0 {
		maxWProject = strconv.Itoa(pa.MaxWorktrees)
	}
	source("max_worktrees", maxWGlobal, maxWProject, "")

	leaseGlobal := strconv.Itoa(ga.LeaseMinutes)
	leaseProject := ""
	if project != nil && pa.LeaseMinutes > 0 {
		leaseProject = strconv.Itoa(pa.LeaseMinutes)
	}
	source("lease_minutes", leaseGlobal, leaseProject, "")

	return nil
}

// resolveEffectiveURL returns the URL that DefaultClient() will use and its source label.
func resolveEffectiveURL(global *config.GlobalConfig, project *config.Config) (string, string) {
	if v := os.Getenv("DX_REMOTE_URL"); v != "" {
		return v, "env"
	}
	if project != nil && project.Remote.URL != "" {
		return project.Remote.URL, "project"
	}
	if global != nil && global.Remote.URL != "" {
		return global.Remote.URL, "global"
	}
	if conn := config.ReadDaemonConn(); conn != nil {
		return conn.URL, "daemon"
	}
	return "", ""
}

// resolveEffectiveCreds returns the credentials path/description and its source label.
func resolveEffectiveCreds(urlSrc string) (string, string) {
	if os.Getenv("DX_REMOTE_API_KEY") != "" {
		return "$DX_REMOTE_API_KEY", "env"
	}
	if urlSrc == "project" || urlSrc == "daemon" {
		if config.ReadCredentials() != "" {
			return ".zdx/credentials", "project"
		}
	}
	home, _ := os.UserHomeDir()
	if home != "" {
		globalCreds := filepath.Join(home, ".zdx", "credentials")
		if _, err := os.Stat(globalCreds); err == nil {
			return "~/.zdx/credentials", "global"
		}
	}
	return "", ""
}

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display resolved configuration (global + project overrides)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow()
		},
	}
}

func configInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Scaffold ~/.zdx/config.yaml interactively",
		RunE: func(cmd *cobra.Command, args []string) error {
			existing := config.LoadGlobal()
			if existing == nil {
				existing = &config.GlobalConfig{}
			}
			local := config.Load()
			agentDefaults := existing.ResolvedGlobalAgent()

			coalesce := func(global, local string) string {
				if global != "" {
					return global
				}
				return local
			}

			r := bufio.NewReader(os.Stdin)
			prompt := func(label, def string) string {
				if def != "" {
					fmt.Printf("%s [%s]: ", label, def)
				} else {
					fmt.Printf("%s: ", label)
				}
				line, _ := r.ReadString('\n')
				line = strings.TrimSpace(line)
				if line == "" {
					return def
				}
				return line
			}

			fmt.Println("Configure ~/.zdx/config.yaml for srcless agent operation.")
			fmt.Println()

			localURL := ""
			localAPIKey := ""
			localModel := ""
			localMaxWorktrees := 0
			localLeaseMinutes := 0
			if local != nil {
				localURL = local.Remote.URL
				localAPIKey = config.RemoteAPIKey()
				localModel = local.Agent.ClaudeModel
				localMaxWorktrees = local.Agent.MaxWorktrees
				localLeaseMinutes = local.Agent.LeaseMinutes
			}

			defaultModel := coalesce(existing.Agent.ClaudeModel, localModel)
			if defaultModel == "" {
				defaultModel = "claude-sonnet-4-6"
			}
			defaultMaxW := agentDefaults.MaxWorktrees
			if existing.Agent.MaxWorktrees == 0 && localMaxWorktrees > 0 {
				defaultMaxW = localMaxWorktrees
			}
			defaultLease := agentDefaults.LeaseMinutes
			if existing.Agent.LeaseMinutes == 0 && localLeaseMinutes > 0 {
				defaultLease = localLeaseMinutes
			}

			url := prompt("Remote URL", coalesce(existing.Remote.URL, localURL))
			apiKey := prompt("API key (written to ~/.zdx/credentials)", coalesce(config.GlobalRemoteAPIKey(), localAPIKey))
			workDir := prompt("Agent work dir", agentDefaults.WorkDir)
			model := prompt("Claude model", defaultModel)

			maxWStr := prompt("Max worktrees", strconv.Itoa(defaultMaxW))
			maxWorktrees, _ := strconv.Atoi(maxWStr)
			if maxWorktrees <= 0 {
				maxWorktrees = 4
			}

			leaseStr := prompt("Lease minutes", strconv.Itoa(defaultLease))
			leaseMinutes, _ := strconv.Atoi(leaseStr)
			if leaseMinutes <= 0 {
				leaseMinutes = 30
			}

			cfg := &config.GlobalConfig{
				Remote: config.GlobalRemote{
					URL: url,
				},
				Agent: config.GlobalAgentConfig{
					WorkDir:      workDir,
					ClaudeModel:  model,
					MaxWorktrees: maxWorktrees,
					LeaseMinutes: leaseMinutes,
				},
			}

			if err := config.WriteGlobal(cfg); err != nil {
				return fmt.Errorf("writing ~/.zdx/config.yaml: %w", err)
			}
			fmt.Println("wrote ~/.zdx/config.yaml")

			if apiKey != "" {
				if err := config.WriteGlobalCredentials(apiKey); err != nil {
					return fmt.Errorf("writing ~/.zdx/credentials: %w", err)
				}
				fmt.Println("wrote ~/.zdx/credentials")
			}

			return nil
		},
	}
}

func configRegisterCmd() *cobra.Command {
	var slug, projectName string
	cmd := &cobra.Command{
		Use:   "register",
		Short: "Register this project on the remote server",
		Long:  "Creates the project on the remote and writes the slug to .zdx/config.yaml.",
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

			c, err := cli.DefaultClient()
			if err != nil {
				return fmt.Errorf("cannot connect to remote: %w", err)
			}

			// Check if project already exists.
			var listResp struct {
				Projects []struct {
					Slug string `json:"slug"`
				} `json:"projects"`
			}
			if err := c.Get("/api/projects", nil, &listResp); err != nil {
				return fmt.Errorf("listing projects: %w", err)
			}

			exists := false
			for _, p := range listResp.Projects {
				if p.Slug == slug {
					exists = true
					break
				}
			}

			if exists {
				fmt.Printf("project: %s (already registered)\n", slug)
			} else {
				var proj struct {
					Slug string `json:"slug"`
					Name string `json:"name"`
				}
				if err := c.Post("/api/project", dxclient.CreateProjectRequest{Slug: slug, Name: projectName}, &proj); err != nil {
					return fmt.Errorf("registering project: %w", err)
				}
				fmt.Printf("project: %s registered\n", proj.Slug)
				slug = proj.Slug
			}

			// Ensure .zdx/ exists and write remote config with slug.
			if err := os.MkdirAll(".zdx", 0755); err != nil {
				return err
			}
			writeRemoteSlug(slug)
			fmt.Printf("slug written to .zdx/config.yaml\n")
			fmt.Printf("ready: dx todo solo\n")
			return nil
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "project slug (defaults to current directory name)")
	cmd.Flags().StringVar(&projectName, "project-name", "", "project display name (defaults to slug)")
	return cmd
}

// writeRemoteSlug updates only the slug in .zdx/config.yaml, preserving other fields.
func writeRemoteSlug(slug string) {
	existing, _ := os.ReadFile(".zdx/config.yaml")
	content := string(existing)

	if strings.Contains(content, "remote:") {
		// Update or insert slug within the remote: block.
		lines := strings.Split(content, "\n")
		var out []string
		inRemote := false
		slugWritten := false
		for _, line := range lines {
			if strings.HasPrefix(line, "remote:") {
				inRemote = true
				out = append(out, line)
				continue
			}
			if inRemote {
				if strings.HasPrefix(line, "  slug:") {
					out = append(out, "  slug: "+slug)
					slugWritten = true
					continue
				}
				if line != "" && !strings.HasPrefix(line, "  ") {
					if !slugWritten {
						out = append(out, "  slug: "+slug)
						slugWritten = true
					}
					inRemote = false
				}
			}
			out = append(out, line)
		}
		if !slugWritten {
			// Append slug at end of remote block (end of file).
			out = append(out, "  slug: "+slug)
		}
		_ = os.WriteFile(".zdx/config.yaml", []byte(strings.Join(out, "\n")), 0644)
	} else if content != "" {
		_ = os.WriteFile(".zdx/config.yaml", []byte("remote:\n  slug: "+slug+"\n\n"+content), 0644)
	} else {
		_ = os.WriteFile(".zdx/config.yaml", []byte("remote:\n  slug: "+slug+"\n"), 0644)
	}
}

func mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
