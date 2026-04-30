package devtools

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ShipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Ship-time checks and helpers",
	}
	cmd.AddCommand(shipGateCmd())
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
