package devtools

import (
	"fmt"
	"github.com/iodesystems/zdx-go/internal/cli"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func WatchCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch for changes and re-run a command",
		RunE: func(cmd *cobra.Command, args []string) error {
			component := config.ActiveComponent("")
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no .zdx/config.yaml found")
			}
			watches := cfg.AllWatches(component)
			if len(watches) == 0 {
				return fmt.Errorf("no watch entries configured")
			}

			var entry *config.NamedWatch
			if name != "" {
				for i := range watches {
					if watches[i].Name == name {
						entry = &watches[i]
						break
					}
				}
				if entry == nil {
					fmt.Fprintf(os.Stderr, "unknown watch: %s\nAvailable:\n", name)
					for _, w := range watches {
						fmt.Fprintf(os.Stderr, "  --name=%-20s %s\n", w.Name, w.Run)
					}
					return fmt.Errorf("watch %q not found", name)
				}
			} else {
				entry = &watches[0]
			}

			fmt.Printf("● watch:%s — %s\n  dirs: %s\n", entry.Name, entry.Run, strings.Join(entry.Dirs, ", "))
			if entry.Include != "" {
				fmt.Printf("  include: %s\n", entry.Include)
			}
			fmt.Println()

			_ = cli.RunShell(entry.Run, "")
			for {
				if !inotifyWait(entry.Dirs, entry.Include) {
					return fmt.Errorf("inotifywait unavailable — install inotify-tools")
				}
				fmt.Print("\n— change detected —\n\n")
				_ = cli.RunShell(entry.Run, "")
			}
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "watch entry name")
	return cmd
}

func inotifyWait(dirs []string, include string) bool {
	inoArgs := []string{"-q", "-r", "-e", "modify,create,delete"}
	if include != "" {
		inoArgs = append(inoArgs, "--include", include)
	}
	inoArgs = append(inoArgs, dirs...)
	cmd := exec.Command("inotifywait", inoArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run() == nil
}

// RunWatch kept for compatibility.
func RunWatch(_ []string) {}
