package credentialhelper

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func CredentialHelperCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "credential-helper <operation>",
		Short: "Git credential helper that authenticates to the zdx git proxy",
		Long: `Standard git credential helper (git-credential protocol).

Configure via:
  git config credential.helper 'dx credential-helper'

Git will invoke this as 'dx credential-helper get', 'store', or 'erase'.
On 'get': outputs username=dx and password=<api_key> from .zdx/credentials or DX_REMOTE_API_KEY.
On 'store' and 'erase': exits 0 (no-op).`,
		Args:               cobra.MaximumNArgs(1),
		DisableFlagParsing: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			op := "get"
			if len(args) > 0 {
				op = args[0]
			}
			switch op {
			case "get":
				return runGet()
			case "store", "erase":
				// Drain stdin per protocol; nothing to do.
				scanner := bufio.NewScanner(os.Stdin)
				for scanner.Scan() {
					if scanner.Text() == "" {
						break
					}
				}
				return nil
			default:
				return nil
			}
		},
	}
	cmd.AddCommand(installCmd())
	return cmd
}

func runGet() error {
	// Read and discard the git credential protocol input.
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if scanner.Text() == "" {
			break
		}
	}

	apiKey := config.RemoteAPIKey()
	if apiKey == "" {
		apiKey = config.GlobalRemoteAPIKey()
	}
	if apiKey == "" {
		return fmt.Errorf("no API key found: set DX_REMOTE_API_KEY or run 'dx ctx --api-key <key>'")
	}

	fmt.Printf("username=dx\npassword=%s\n\n", apiKey)
	return nil
}

func installCmd() *cobra.Command {
	var workDir string
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Configure git credential.helper in agent work_dir repos",
		RunE: func(cmd *cobra.Command, args []string) error {
			if workDir == "" {
				gcfg := config.LoadGlobal()
				if gcfg == nil {
					gcfg = &config.GlobalConfig{}
				}
				workDir = gcfg.ResolvedGlobalAgent().WorkDir
			}

			entries, err := os.ReadDir(workDir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Printf("work_dir %s does not exist — nothing to configure\n", workDir)
					return nil
				}
				return fmt.Errorf("reading work_dir %s: %w", workDir, err)
			}

			configured := 0
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				repoPath := filepath.Join(workDir, e.Name())
				gitDir := filepath.Join(repoPath, ".git")
				if _, err := os.Stat(gitDir); os.IsNotExist(err) {
					continue
				}
				out, err := exec.Command("git", "-C", repoPath, "config", "credential.helper", "dx credential-helper").CombinedOutput()
				if err != nil {
					fmt.Fprintf(os.Stderr, "warn: %s: %s\n", repoPath, strings.TrimSpace(string(out)))
					continue
				}
				fmt.Printf("configured: %s\n", repoPath)
				configured++
			}

			if configured == 0 {
				fmt.Println("no git repos found in work_dir")
			} else {
				fmt.Printf("%d repo(s) configured\n", configured)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&workDir, "work-dir", "", "agent work directory (default: from config)")
	return cmd
}
