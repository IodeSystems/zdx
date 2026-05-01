package ship

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/config"
)

// blueGreenStrategy deploys to the standby slot, then re-runs verify-
// tagged stages with the slot vars swapped so verification observes
// what is now the active slot. Two passes total:
//
//	pass 1 (deploy):  ZDX_ACTIVE_SLOT=<a>, ZDX_STANDBY_SLOT=<b>  — all stages
//	pass 2 (verify):  ZDX_ACTIVE_SLOT=<b>, ZDX_STANDBY_SLOT=<a>  — only stages tagged "verify"
//
// Slot values come from the caller's env (ZDX_ACTIVE_SLOT,
// ZDX_STANDBY_SLOT). Pass 1 halts the whole strategy on a non-optional
// failure (no swap, no verify); pass 2 halts on a non-optional failure
// in any verify stage.
type blueGreenStrategy struct{}

func (blueGreenStrategy) Run(ctx context.Context, comp config.Component, env map[string]string, _ RunOptions) ([]StageResult, error) {
	active := env["ZDX_ACTIVE_SLOT"]
	standby := env["ZDX_STANDBY_SLOT"]

	deployEnv := map[string]string{
		"ZDX_ACTIVE_SLOT":  active,
		"ZDX_STANDBY_SLOT": standby,
	}
	results, err := runStages(ctx, comp, env, deployEnv, comp.Ship.Stages, nil, "")
	if err != nil {
		return results, err
	}

	verifyStages := filterTagged(comp.Ship.Stages, "verify")
	if len(verifyStages) == 0 {
		return results, nil
	}

	verifyEnv := map[string]string{
		"ZDX_ACTIVE_SLOT":  standby,
		"ZDX_STANDBY_SLOT": active,
	}
	verifyResults, err := runStages(ctx, comp, env, verifyEnv, verifyStages, nil, "")
	results = append(results, verifyResults...)
	if err != nil {
		return results, err
	}
	return results, nil
}

// filterTagged returns the subset of stages whose Tags contain tag.
func filterTagged(stages []config.Stage, tag string) []config.Stage {
	var out []config.Stage
	for _, s := range stages {
		for _, t := range s.Tags {
			if t == tag {
				out = append(out, s)
				break
			}
		}
	}
	return out
}
