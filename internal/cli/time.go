package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

func TimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "time",
		Short: "Timing instrumentation commands",
	}

	cmd.AddCommand(timeRunCmd())
	cmd.AddCommand(TimeGoCompileCmd())

	return cmd
}

func timeRunCmd() *cobra.Command {
	var name, component, env string

	cmd := &cobra.Command{
		Use:   "run [flags] -- cmd [args...]",
		Short: "Run a command and ingest its wall-clock duration",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = args[0]
			}
			if component == "" {
				component = config.ActiveComponent("")
			}

			token, err := resolveIngestToken()
			if err != nil {
				return fmt.Errorf("ingest token: %w", err)
			}

			cfg := config.Load()
			endpoint := cfg.RemoteURL()
			if endpoint == "" {
				return fmt.Errorf("no remote URL configured")
			}

			c, err := zdxclient.New(zdxclient.Config{
				Endpoint:      endpoint,
				Token:         token,
				Component:     component,
				Environment:   env,
				FlushInterval: 2 * time.Second,
				MaxBatch:      1,
				BufferSize:    4,
				OnError: func(err error, n int) {
					fmt.Fprintf(os.Stderr, "dx time: ingest error: %v\n", err)
				},
			})
			if err != nil {
				return err
			}

			child := exec.Command(args[0], args[1:]...)
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			child.Stdin = os.Stdin

			start := time.Now()
			runErr := child.Run()
			elapsed := time.Since(start)

			c.Record(name, elapsed, nil)
			_ = c.Close(context.Background())

			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				return runErr
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "event name (default: first arg)")
	cmd.Flags().StringVar(&component, "component", "", "component label")
	cmd.Flags().StringVar(&env, "env", "", "environment label")

	return cmd
}

func resolveIngestToken() (string, error) {
	if tok := os.Getenv("ZDX_INGEST_TOKEN"); tok != "" {
		return tok, nil
	}

	cached, err := os.ReadFile(".zdx/ingest-token")
	if err == nil {
		tok := strings.TrimSpace(string(cached))
		if tok != "" {
			return tok, nil
		}
	}

	cl, err := DefaultClient()
	if err != nil {
		return "", fmt.Errorf("cannot auto-provision token: %w", err)
	}

	var resp struct {
		Token string `json:"token"`
	}
	err = cl.post("/api/admin/integration-tokens", map[string]string{
		"slug": cl.SlugOrDie(),
		"name": "dx-time-cli",
	}, &resp)
	if err != nil {
		return "", fmt.Errorf("create integration token: %w", err)
	}

	_ = os.WriteFile(".zdx/ingest-token", []byte(resp.Token+"\n"), 0600)
	return resp.Token, nil
}
