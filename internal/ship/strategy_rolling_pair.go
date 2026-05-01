package ship

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/iodesystems/zdx-go/internal/config"
)

// rollingPairStrategy runs the full stage list twice — once against the
// "current" slot (ZDX_SLOT_A) and once against the "next" slot
// (ZDX_SLOT_B). Each pass injects ZDX_PHASE so stage scripts can branch
// on which half of the pair they're servicing:
//
//	pass 1: ZDX_PHASE=current, ZDX_SLOT=<env[ZDX_SLOT_A]>
//	pass 2: ZDX_PHASE=next,    ZDX_SLOT=<env[ZDX_SLOT_B]>
//
// A non-optional failure in pass 1 halts before pass 2 runs; results
// from both passes are concatenated in execution order.
//
// Resume: completed stages are persisted to .zdx/ship-state/<sha>-<comp>-<phase>.json.
// On restart, already-completed stages are skipped. Pass opts.NoResume=true
// to delete the state files and force a full re-run.
type rollingPairStrategy struct{}

func (rollingPairStrategy) Run(ctx context.Context, comp config.Component, env map[string]string, opts RunOptions) ([]StageResult, error) {
	sha := gitSHA()
	main, fin := splitStages(comp.Ship.Stages)

	passes := []struct {
		phase string
		slot  string
	}{
		{"current", env["ZDX_SLOT_A"]},
		{"next", env["ZDX_SLOT_B"]},
	}

	var results []StageResult
	var runErr error
	for _, pass := range passes {
		sf := stateFilePath(opts.StateDir, sha, opts.ComponentName, pass.phase)
		if opts.NoResume {
			_ = os.Remove(sf)
		}
		skip, _ := loadCompletedStages(sf) // read error → empty skip set
		extraEnv := map[string]string{
			"ZDX_PHASE": pass.phase,
			"ZDX_SLOT":  pass.slot,
		}
		r, err := runStages(ctx, comp, env, extraEnv, main, skip, sf)
		results = append(results, r...)
		if err != nil {
			runErr = err
			break
		}
	}
	results = append(results, runFinalize(ctx, comp, env, nil, fin, runErr)...)
	return results, runErr
}

func gitSHA() string {
	out, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}
