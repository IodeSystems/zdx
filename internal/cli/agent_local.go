package cli

import (
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func agentLocalCmd() *cobra.Command {
	var loop bool
	var alias string
	var issue string
	cmd := &cobra.Command{
		Use:   "local",
		Short: "Run local-LLM agent sessions with zdx integration (scaffold)",
		Long: `Scaffold for dx agent local. Loads the llm_local config, registers
the dx project tools plus filesystem/shell MCP tools in-process, and prints a
status summary describing what a full agent loop would see.

The OpenAI-compatible chat-completions loop with tool-calling is not yet
implemented — that is tracked as a follow-up issue.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			llm := cfg.ResolvedLLMLocal()

			c, err := DefaultClient()
			if err != nil {
				return err
			}

			root, err := gitRepoRoot()
			if err != nil {
				return fmt.Errorf("dx agent local must run inside a git repo: %w", err)
			}

			srv := mcp.NewServer(&mcp.Implementation{
				Name:    "dx-agent-local",
				Version: "0.1.0",
			}, nil)
			registerMCPTools(srv, c)
			RegisterFSTools(srv, root)
			RegisterShellTools(srv, root)

			source := "interactive (no --issue or --loop)"
			switch {
			case loop:
				source = "solo loop (dx todo solo)"
			case issue != "":
				source = "issue " + issue
			}

			fmt.Println("dx agent local — scaffold")
			fmt.Println(strings.Repeat("-", 40))
			fmt.Printf("endpoint:  %s\n", llm.BaseURL)
			fmt.Printf("model:     %s\n", llm.Model)
			fmt.Printf("timeout:   %ds\n", llm.TimeoutSeconds)
			if llm.APIKey != "" {
				fmt.Printf("api_key:   set (%d chars)\n", len(llm.APIKey))
			} else {
				fmt.Printf("api_key:   (none)\n")
			}
			fmt.Printf("alias:     %s\n", fallback(alias, "(unset)"))
			fmt.Printf("repo root: %s\n", root)
			fmt.Printf("work:      %s\n", source)
			fmt.Println()
			fmt.Println("registered MCP tools:")
			fmt.Println("  project:    " + strings.Join(projectToolNames(), ", "))
			fmt.Println("  filesystem: " + strings.Join(fsToolNames(), ", "))
			fmt.Println("  shell:      " + strings.Join(shellToolNames(), ", "))
			fmt.Println()
			fmt.Println("chat-completions loop not implemented — see IS-217 follow-up.")
			return nil
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat (not yet implemented)")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	return cmd
}

func fallback(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func projectToolNames() []string {
	return []string{
		"issue_list", "issue_add", "issue_show", "issue_close", "issue_edit",
		"issue_block", "issue_unblock",
		"todo_solo", "todo_show", "todo_list", "todo_dev_done", "todo_dev_start",
		"todo_owner_triage", "todo_tech_add",
		"feature_list", "feature_show",
		"pattern_search", "pattern_refine",
		"question_search", "question_add",
		"comment_list", "comment_add",
	}
}

func fsToolNames() []string {
	return []string{"read_file", "write_file", "edit_file", "list_dir", "glob", "grep"}
}

func shellToolNames() []string {
	return []string{"run_bash"}
}
