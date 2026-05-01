package devtools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/ship"
)

func ShipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Ship-time checks and helpers",
	}
	cmd.AddCommand(shipGateCmd())
	cmd.AddCommand(shipRunCmd())
	return cmd
}

func shipGitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func shipRunCmd() *cobra.Command {
	var envFlag, componentFlag string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute ship stages for a component and record the deploy event",
		RunE: func(cmd *cobra.Command, _ []string) error {
			compName := config.ActiveComponent(componentFlag)
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no .zdx/config.yaml found")
			}
			comp, ok := cfg.Components[compName]
			if !ok {
				return fmt.Errorf("component %q not found", compName)
			}
			if comp.Ship.IsZero() {
				return fmt.Errorf("component %q has no ship config", compName)
			}

			sha := shipGitOutput("rev-parse", "HEAD")
			branch := shipGitOutput("rev-parse", "--abbrev-ref", "HEAD")

			results, runErr := ship.Run(cmd.Context(), comp, nil)

			c, err := cli.DefaultClient()
			if err != nil {
				return err
			}
			slug := c.SlugOrDie()
			_ = ship.PostDeployEvent(cmd.Context(), c, slug, envFlag, sha, branch, results)

			return runErr
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "environment name to record the deploy against (required)")
	_ = cmd.MarkFlagRequired("env")
	cmd.Flags().StringVar(&componentFlag, "component", "", "component name (default: active component from config or DX_COMPONENT)")
	return cmd
}

func shipGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gate",
		Short: "Check must-tier spec demos before deploy",
		Long:  "Queries must-tier spec demo gaps via the API. Exits non-zero if any must-specs lack passing demos. Informational should/nice-to-have counts are printed but never affect exit code.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cli.DefaultClient()
			if err != nil {
				return fmt.Errorf("ship gate: %w", err)
			}
			slug := c.SlugOrDie()
			ctx := cmd.Context()

			// ── Must-spec gate (blocking) ──────────────────────────────────────
			mustResp, err := c.ListMustSpecShipGateOffendersWithResponse(ctx, &dxclient.ListMustSpecShipGateOffendersParams{
				Slug: slug,
			})
			if err != nil {
				return fmt.Errorf("ship gate: could not fetch must-spec demo gaps: %v", err)
			}
			if mustResp.JSON200 == nil {
				if mustResp.StatusCode() == 404 {
					fmt.Println("[ship gate] endpoint not yet deployed — skipping must-spec check")
					return nil
				}
				return fmt.Errorf("ship gate: could not fetch must-spec demo gaps: HTTP %d", mustResp.StatusCode())
			}
			offenders := *mustResp.JSON200.Offenders

			// ── Informational should/nice-to-have counts ───────────────────────
			gapResp, gapErr := c.ListSpecsWithoutDemosWithResponse(ctx, &dxclient.ListSpecsWithoutDemosParams{
				Slug: slug,
			})
			var shouldCount, niceCount int
			if gapErr == nil && gapResp.JSON200 != nil {
				for _, s := range *gapResp.JSON200.Specs {
					switch s.Importance {
					case "should":
						shouldCount++
					case "nice-to-have":
						niceCount++
					}
				}
			}

			if len(offenders) == 0 {
				fmt.Println("[ship gate] No must-spec demo gaps. Ready to ship.")
				if shouldCount > 0 || niceCount > 0 {
					fmt.Printf("[ship gate] (informational) shoulds: %d, nice-to-have: %d\n", shouldCount, niceCount)
				}
				return nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "[ship gate] Must-spec demo gaps — deploy blocked:\n")
			for _, o := range offenders {
				fmt.Fprintf(&b, "  SP-%d  %s  %s  (%s)\n", o.SpecId, o.Feature, o.Description, o.Reason)
			}
			if shouldCount > 0 || niceCount > 0 {
				fmt.Fprintf(&b, "[ship gate] (informational) shoulds: %d, nice-to-have: %d\n", shouldCount, niceCount)
			}
			fmt.Fprint(os.Stderr, b.String())
			return fmt.Errorf("must-spec gate: %d offender(s)", len(offenders))
		},
	}
}
