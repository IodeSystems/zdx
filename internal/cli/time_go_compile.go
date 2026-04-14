package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

type actiongraphEntry struct {
	Mode      string    `json:"Mode"`
	Package   string    `json:"Package"`
	TimeStart time.Time `json:"TimeStart"`
	TimeDone  time.Time `json:"TimeDone"`
}

var modeMap = map[string]string{
	"build":        "go-compile",
	"link":         "go-link",
	"link-install": "go-link-install",
	"cgo run":      "go-cgo-run",
	"collect cgo":  "go-collect-cgo",
}

func normalizeMode(mode string) string {
	if mapped, ok := modeMap[mode]; ok {
		return mapped
	}
	if strings.HasPrefix(mode, "cgo compile") {
		return "go-cgo-compile"
	}
	return ""
}

func TimeGoCompileCmd() *cobra.Command {
	var component, env string

	cmd := &cobra.Command{
		Use:   "go-compile [flags] -- go build [args...]",
		Short: "Wrap go build with -debug-actiongraph and ingest per-action timings",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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

			tmpFile, err := os.CreateTemp("", "actiongraph-*.json")
			if err != nil {
				return fmt.Errorf("create temp file: %w", err)
			}
			tmpPath := tmpFile.Name()
			tmpFile.Close()
			defer os.Remove(tmpPath)

			if len(args) < 2 {
				return fmt.Errorf("expected at least 'go build', got: %v", args)
			}
			childArgs := make([]string, 0, len(args)+1)
			childArgs = append(childArgs, args[1], "-debug-actiongraph="+tmpPath)
			childArgs = append(childArgs, args[2:]...)
			child := exec.Command(args[0], childArgs...)
			child.Stdout = os.Stdout
			child.Stderr = os.Stderr
			child.Stdin = os.Stdin

			runErr := child.Run()

			data, err := os.ReadFile(tmpPath)
			if err != nil {
				if runErr != nil {
					if exitErr, ok := runErr.(*exec.ExitError); ok {
						os.Exit(exitErr.ExitCode())
					}
					return runErr
				}
				return fmt.Errorf("read actiongraph: %w", err)
			}

			var actions []actiongraphEntry
			if err := json.Unmarshal(data, &actions); err != nil {
				return fmt.Errorf("parse actiongraph: %w", err)
			}

			c, err := zdxclient.New(zdxclient.Config{
				Endpoint:      endpoint,
				Token:         token,
				Component:     component,
				Environment:   env,
				FlushInterval: 2 * time.Second,
				MaxBatch:      500,
				BufferSize:    len(actions) + 64,
				OnError: func(err error, n int) {
					fmt.Fprintf(os.Stderr, "dx time:go-compile: ingest error: %v\n", err)
				},
			})
			if err != nil {
				return err
			}

			var recorded int
			for _, a := range actions {
				mode := normalizeMode(a.Mode)
				if mode == "" {
					continue
				}
				dur := a.TimeDone.Sub(a.TimeStart)
				if dur <= 0 {
					continue
				}
				name := mode + ":" + a.Package
				c.Record(name, dur, nil)
				recorded++
			}

			_ = c.Close(context.Background())
			fmt.Fprintf(os.Stderr, "dx time:go-compile: ingested %d actions\n", recorded)

			if runErr != nil {
				if exitErr, ok := runErr.(*exec.ExitError); ok {
					os.Exit(exitErr.ExitCode())
				}
				return runErr
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&component, "component", "", "component label")
	cmd.Flags().StringVar(&env, "env", "", "environment label")

	return cmd
}
