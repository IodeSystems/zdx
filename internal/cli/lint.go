package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/dx/internal/config"
)

func LintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint [component]",
		Short: "Run configured linters",
		RunE: func(cmd *cobra.Command, args []string) error {
			component := config.ActiveComponent("")
			if len(args) > 0 {
				component = args[0]
			}
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no .zdx/config.yaml found")
			}
			failed := false
			for name, comp := range cfg.Components {
				if component != "" && name != component {
					continue
				}
				for _, step := range comp.Lint.External {
					label := step.Name
					if label == "" {
						label = step.Run
					}
					fmt.Printf("● lint %s (%s)\n", label, name)
					if err := runShell(step.Run, step.CWD); err != nil {
						fmt.Printf("  FAILED: %v\n", err)
						failed = true
					}
				}
			}
			if failed {
				return fmt.Errorf("lint failed")
			}
			return nil
		},
	}
}

// RunLint kept for compatibility.
func RunLint(_ []string) {}
