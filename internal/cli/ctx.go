package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iodesystems/dx/internal/config"
)

func CtxCmd() *cobra.Command {
	var clear bool
	var remote, slug, apiKey string
	cmd := &cobra.Command{
		Use:   "ctx [component]",
		Short: "Set or show active component context",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clear {
				_ = config.WriteContext("", "")
				fmt.Println("context cleared")
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

			// Show status
			cfg := config.Load()
			if comp := config.ActiveComponent(""); comp != "" {
				fmt.Printf("component: %s\n", comp)
			} else {
				fmt.Println("component: (all)")
			}
			if u := cfg.RemoteURL(); u != "" {
				fmt.Printf("remote: %s\n", u)
				if s := cfg.RemoteSlug(); s != "" {
					fmt.Printf("slug: %s\n", s)
				}
			} else if conn := config.ReadDaemonConn(); conn != nil {
				fmt.Printf("remote: %s (daemon)\n", conn.URL)
			} else {
				fmt.Println("remote: not configured")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&clear, "clear", false, "clear component context")
	cmd.Flags().StringVar(&remote, "remote", "", "set remote URL")
	cmd.Flags().StringVar(&slug, "slug", "", "set project slug")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "set API key (written to .zdx/credentials)")
	return cmd
}

// RunCtx kept for compatibility.
func RunCtx(_ []string) {}
