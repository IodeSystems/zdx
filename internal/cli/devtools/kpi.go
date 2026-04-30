package devtools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// KpiCmd returns the "dx kpi" subcommand group.
func KpiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kpi",
		Short: "Submit and view project KPI samples",
	}
	cmd.AddCommand(kpiSampleCmd())
	return cmd
}

// KpiSampleParams is the input shape for RunKpiSample. Exported so other
// CLI packages (e.g. dx standup checkin) can reuse the same posting logic.
type KpiSampleParams struct {
	Slug                  string
	Scope                 string
	CheckName             string
	Value                 float64
	Unit                  string
	RegressionThresholdMs float64
}

// kpiSampleParams is the unexported alias kept for the local cobra command —
// other packages must use the exported KpiSampleParams.
type kpiSampleParams = KpiSampleParams

// kpiSampleBody returns the JSON request body for POST /api/dx/kpi/sample.
// is_regression_candidate is set when unit=ms and value exceeds the threshold —
// the server currently ignores the field (forward-compatible).
func kpiSampleBody(p kpiSampleParams) ([]byte, error) {
	body := map[string]any{
		"slug":       p.Slug,
		"scope":      p.Scope,
		"check_name": p.CheckName,
		"value":      p.Value,
		"unit":       p.Unit,
	}
	if p.Unit == "ms" && p.Value > p.RegressionThresholdMs {
		body["is_regression_candidate"] = true
	}
	return json.Marshal(body)
}

// RunKpiSample posts a single KPI sample to /api/dx/kpi/sample. Scope must be
// "tech" or "owner". Exported wrapper around the package's internal posting
// path so dx standup checkin can persist tracker metrics the same way.
func RunKpiSample(ctx context.Context, c *cli.Client, p KpiSampleParams) error {
	return runKpiSample(ctx, c, p)
}

func runKpiSample(ctx context.Context, c *cli.Client, p kpiSampleParams) error {
	if p.Scope != "tech" && p.Scope != "owner" {
		return fmt.Errorf("--scope must be 'tech' or 'owner', got %q", p.Scope)
	}
	buf, err := kpiSampleBody(p)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}
	resp, err := c.PostKpiSampleWithBodyWithResponse(ctx, "application/json", bytes.NewReader(buf))
	if err != nil {
		return err
	}
	return c.CheckStatus(resp.StatusCode(), resp.Body)
}

func kpiSampleCmd() *cobra.Command {
	p := kpiSampleParams{}
	cmd := &cobra.Command{
		Use:   "sample",
		Short: "Post a single KPI sample to the server",
		Long: `Submit one KPI measurement (e.g. lint duration, build duration) to the
server. Designed for shell pipelines — silent on success, prints error and
exits 1 on failure.

Example:
  dx kpi sample --check-name=bin_lint --value=4231 --unit=ms --scope=tech`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			p.Slug = c.SlugOrDie()
			return runKpiSample(cmd.Context(), c, p)
		},
	}
	cmd.Flags().StringVar(&p.CheckName, "check-name", "", "metric name (required, e.g. bin_lint, build)")
	cmd.Flags().Float64Var(&p.Value, "value", 0, "metric value (required)")
	cmd.Flags().StringVar(&p.Unit, "unit", "ms", "metric unit (e.g. ms, count, percent)")
	cmd.Flags().StringVar(&p.Scope, "scope", "tech", "scope: tech | owner")
	cmd.Flags().Float64Var(&p.RegressionThresholdMs, "regression-threshold-ms", 15000, "if --unit=ms and value exceeds this, flag the sample as a regression candidate")
	_ = cmd.MarkFlagRequired("check-name")
	return cmd
}
