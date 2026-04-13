package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func InitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create .zdx/ scaffold",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(".zdx", 0755); err != nil {
				return err
			}
			if _, err := os.Stat(".zdx/config.yaml"); os.IsNotExist(err) {
				const skeleton = `# zdx project configuration

# remote:
#   url: https://your-zdx-server.example.com
#   slug: your-project

hooks:
  pre-commit:
    - dx lint
    - dx build

components:
  app:
    build:
      steps:
        - name: compile
          run: go build ./...
    test:
      unit:
        runner: basic
        run: go test ./...
    lint:
      external:
        - name: vet
          run: go vet ./...
`
				if err := os.WriteFile(".zdx/config.yaml", []byte(skeleton), 0644); err != nil {
					return err
				}
				fmt.Println("created .zdx/config.yaml")
			} else {
				fmt.Println(".zdx/config.yaml already exists")
			}

			ignoreEntries := []string{
				".zdx/context",
				".zdx/credentials",
				".zdx/test-results.json",
				".zdx/test-index.json",
				".zdx/smoke.json",
				".zdx/metrics-latest.yaml",
			}
			gitignore, _ := os.ReadFile(".gitignore")
			var added []string
			for _, e := range ignoreEntries {
				if !strings.Contains(string(gitignore), e) {
					added = append(added, e)
				}
			}
			if len(added) > 0 {
				f, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err == nil {
					for _, e := range added {
						fmt.Fprintln(f, e)
					}
					f.Close()
					fmt.Printf("added %d entries to .gitignore\n", len(added))
				}
			}
			fmt.Println("✓ .zdx/ initialized")
			return nil
		},
	}
}

// RunInit kept for compatibility.
func RunInit(_ []string) {}
