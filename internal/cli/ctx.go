package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func CtxCmd() *cobra.Command {
	var clearScope string
	var remote, slug, apiKey string
	cmd := &cobra.Command{
		Use:   "ctx [component]",
		Short: "Set or show active component context",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clearScope != "" {
				switch clearScope {
				case "local", "all", "true":
					_ = config.WriteContext("", "")
					_ = os.Remove(".zdx/credentials")
					fmt.Println("local context cleared")
					if clearScope == "local" {
						return nil
					}
					fallthrough
				case "global":
					home, _ := os.UserHomeDir()
					_ = os.Remove(filepath.Join(home, ".zdx", "credentials"))
					_ = os.Remove(filepath.Join(home, ".zdx", "config.yaml"))
					fmt.Println("global context cleared")
				default:
					return fmt.Errorf("--clear: invalid scope %q (use local, global, or all)", clearScope)
				}
				return nil
			}
			if apiKey != "" {
				if err := os.WriteFile(".zdx/credentials", []byte(apiKey+"\n"), 0600); err != nil {
					return err
				}
				fmt.Println("credentials written to .zdx/credentials")
			}
			if slug != "" {
				_ = config.WriteContext(config.ActiveComponent(""), slug)
			}
			if remote != "" {
				fmt.Printf("set DX_REMOTE_URL=%s or add to .zdx/config.yaml:\n  remote:\n    url: %s\n", remote, remote)
			}
			if apiKey != "" || slug != "" || remote != "" {
				return nil
			}

			if len(args) > 0 {
				component := args[0]
				cfg := config.Load()
				if cfg != nil {
					if _, ok := cfg.Components[component]; !ok {
						fmt.Fprintf(os.Stderr, "unknown component: %s\nAvailable:", component)
						for name := range cfg.Components {
							fmt.Fprintf(os.Stderr, " %s", name)
						}
						fmt.Fprintln(os.Stderr)
						return fmt.Errorf("unknown component: %s", component)
					}
				}
				_ = config.WriteContext(component, "")
				fmt.Printf("context: %s\n", component)
				return nil
			}

			// Show status: global layer first, then local.
			home, _ := os.UserHomeDir()
			globalCfg := config.LoadGlobal()
			fmt.Println("[global]")
			if globalCfg != nil && globalCfg.Remote.URL != "" {
				fmt.Printf("  remote: %s\n", globalCfg.Remote.URL)
			} else {
				fmt.Println("  remote: not configured")
			}
			if home != "" {
				globalCreds := filepath.Join(home, ".zdx", "credentials")
				if _, err := os.Stat(globalCreds); err == nil {
					fmt.Printf("  credentials: %s\n", globalCreds)
				} else {
					fmt.Println("  credentials: none")
				}
			}

			fmt.Println("[local]")
			cfg := config.Load()
			if comp := config.ActiveComponent(""); comp != "" {
				fmt.Printf("  component: %s\n", comp)
			} else {
				fmt.Println("  component: (all)")
			}
			if u := cfg.RemoteURL(); u != "" {
				fmt.Printf("  remote: %s\n", u)
				if s := cfg.RemoteSlug(); s != "" {
					fmt.Printf("  slug: %s\n", s)
				}
			} else if conn := config.ReadDaemonConn(); conn != nil {
				fmt.Printf("  remote: %s (daemon)\n", conn.URL)
			} else {
				fmt.Println("  remote: not configured")
			}
			if _, err := os.Stat(".zdx/credentials"); err == nil {
				fmt.Println("  credentials: .zdx/credentials")
			} else {
				fmt.Println("  credentials: none")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&clearScope, "clear", "", "clear context: local, global, or all")
	cmd.Flags().StringVar(&remote, "remote", "", "set remote URL")
	cmd.Flags().StringVar(&slug, "slug", "", "set project slug")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "set API key (written to .zdx/credentials)")
	return cmd
}

// RunCtx kept for compatibility.
func RunCtx(_ []string) {}
