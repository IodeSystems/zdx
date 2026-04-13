package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func BuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build [component]",
		Short: "Run configured build steps",
		RunE: func(cmd *cobra.Command, args []string) error {
			component := config.ActiveComponent("")
			if len(args) > 0 {
				component = args[0]
			}
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no .zdx/config.yaml found")
			}
			steps := cfg.AllBuildSteps(component)
			if len(steps) == 0 {
				return fmt.Errorf("no build steps configured for %q", component)
			}
			for _, s := range steps {
				label := s.Name
				if label == "" {
					label = s.Run
				}
				fmt.Printf("● %s (%s)\n", label, s.Component)
				if err := runShell(s.Run, s.CWD); err != nil {
					return fmt.Errorf("%s: %w", s.Run, err)
				}
			}
			fmt.Println("\n✓ build complete")
			return nil
		},
	}
}

func CheckCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [component]",
		Short: "Build + lint + test",
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, sub := range []*cobra.Command{BuildCmd(), LintCmd(), TestCmd()} {
				if err := sub.RunE(sub, args); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func runShell(cmd, cwd string) error {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if cwd != "" {
		c.Dir = cwd
	}
	return c.Run()
}

// RunBuild kept for compatibility.
func RunBuild(_ []string) {}
func RunCheck(_ []string) {}
