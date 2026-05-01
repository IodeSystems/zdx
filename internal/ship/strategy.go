package ship

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// Strategy is the per-shape execution policy for a ship pipeline.
// Implementations decide how many passes to run, what env to inject,
// and which stages participate in each pass. All strategies share the
// runStages helper for the per-stage exec/log/halt-on-failure loop so
// behavior stays consistent across shapes.
type Strategy interface {
	Run(ctx context.Context, comp config.Component, env map[string]string, opts RunOptions) ([]StageResult, error)
}

// dispatch returns the Strategy for comp.Ship.Strategy. An empty string
// defaults to simple — config validation already rejects unknown values,
// so an unrecognized non-empty value here means the const list and the
// dispatch table drifted; we panic to surface that wiring bug loudly
// rather than silently degrading to simple.
func dispatch(comp config.Component) Strategy {
	switch comp.Ship.Strategy {
	case "", config.ShipStrategySimple:
		return simpleStrategy{}
	case config.ShipStrategyBlueGreen:
		return blueGreenStrategy{}
	case config.ShipStrategyMaintenance:
		return maintenanceStrategy{}
	case config.ShipStrategyRollingPair:
		return rollingPairStrategy{}
	}
	panic(fmt.Sprintf("ship: no Strategy registered for %q (config validated but dispatch did not)", comp.Ship.Strategy))
}

// runStages executes the given stages once and returns per-stage results.
// Stages whose names appear in skipSet are recorded as "skipped" without
// execution. On success, each stage name is appended to stateFile (best-
// effort; failure is logged but does not abort the run). Pass nil/""
// to disable both behaviors. The extra map is layered between caller env
// and Ship.Env so strategies can inject ZDX_* vars without overriding
// project intent.
func runStages(ctx context.Context, comp config.Component, env, extra map[string]string, stages []config.Stage, skipSet map[string]bool, stateFile string) ([]StageResult, error) {
	results := make([]StageResult, 0, len(stages))
	for _, stage := range stages {
		if skipSet[stage.Name] {
			results = append(results, StageResult{
				Name:   stage.Name,
				Status: "skipped",
				Log:    "skipped (already completed)",
			})
			continue
		}
		start := time.Now()
		cmd := buildCmd(ctx, stage)
		if stage.Target == "" {
			cmd.Env = mergeEnv(os.Environ(), env, extra, comp.Ship.Env)
		}
		out, runErr := cmd.CombinedOutput()
		res := StageResult{
			Name:     stage.Name,
			Duration: time.Since(start),
			Log:      string(out),
		}
		if runErr != nil {
			res.Status = "failed"
			results = append(results, res)
			if stage.Optional {
				fmt.Fprintf(os.Stderr, "ship: optional stage %q failed: %v\n", stage.Name, runErr)
				continue
			}
			return results, fmt.Errorf("stage %q failed: %w", stage.Name, runErr)
		}
		res.Status = "ok"
		if stateFile != "" {
			if err := appendCompletedStage(stateFile, stage.Name); err != nil {
				fmt.Fprintf(os.Stderr, "ship: state write failed for %q: %v\n", stage.Name, err)
			}
		}
		results = append(results, res)
	}
	return results, nil
}
