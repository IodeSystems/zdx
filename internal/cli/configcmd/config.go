package configcmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func ConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage dx configuration",
	}
	cmd.AddCommand(configInitCmd(), configShowCmd())
	return cmd
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
			agentDefaults := existing.ResolvedGlobalAgent()

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

			url := prompt("Remote URL", existing.Remote.URL)
			apiKey := prompt("API key (written to ~/.zdx/credentials)", config.GlobalRemoteAPIKey())
			workDir := prompt("Agent work dir", agentDefaults.WorkDir)
			model := prompt("Claude model", agentDefaults.ClaudeModel)

			maxWStr := prompt("Max worktrees", strconv.Itoa(agentDefaults.MaxWorktrees))
			maxWorktrees, _ := strconv.Atoi(maxWStr)
			if maxWorktrees <= 0 {
				maxWorktrees = 4
			}

			leaseStr := prompt("Lease minutes", strconv.Itoa(agentDefaults.LeaseMinutes))
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

func configShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display resolved configuration (global + project overrides)",
		RunE: func(cmd *cobra.Command, args []string) error {
			global := config.LoadGlobal()
			project := config.Load()

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
		},
	}
}

func mask(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
