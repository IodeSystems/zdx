package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/dx/internal/config"
)

const hookPath = ".git/hooks/pre-commit"

func HooksCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hooks",
		Short: "Manage git hooks",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(hookPath); err == nil {
				fmt.Println("✓ pre-commit hook installed")
				fmt.Println("  uninstall: dx hooks --uninstall")
			} else {
				fmt.Println("no pre-commit hook installed")
				fmt.Println("  install: dx hooks --install")
			}
			return nil
		},
	}
	var install, uninstall bool
	cmd.Flags().BoolVar(&install, "install", false, "install git hooks")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "uninstall git hooks")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		switch {
		case install:
			script := generateHookScript()
			if err := os.MkdirAll(".git/hooks", 0755); err != nil {
				return err
			}
			if err := os.WriteFile(hookPath, []byte(script), 0755); err != nil {
				return err
			}
			fmt.Println("✓ installed .git/hooks/pre-commit")
		case uninstall:
			if err := os.Remove(hookPath); err != nil {
				if os.IsNotExist(err) {
					fmt.Println("no pre-commit hook installed")
					return nil
				}
				return err
			}
			fmt.Println("✓ removed .git/hooks/pre-commit")
		default:
			if _, err := os.Stat(hookPath); err == nil {
				fmt.Println("✓ pre-commit hook installed")
				fmt.Println("  uninstall: dx hooks --uninstall")
			} else {
				fmt.Println("no pre-commit hook installed")
				fmt.Println("  install: dx hooks --install")
			}
		}
		return nil
	}
	return cmd
}

func generateHookScript() string {
	cfg := config.Load()
	if cfg == nil || len(cfg.Hooks.PreCommit) == 0 {
		return "#!/bin/sh\nset -e\ndx build\ndx lint\n"
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n# Installed by: dx hooks --install\nset -e\n\n")
	for _, cmd := range cfg.Hooks.PreCommit {
		fmt.Fprintf(&b, "echo \"pre-commit: %s...\"\n%s\n\n", cmd, cmd)
	}
	return b.String()
}

// RunHooks kept for compatibility.
func RunHooks(_ []string) {}
