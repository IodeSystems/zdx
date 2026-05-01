package ship

import (
	"context"

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
type rollingPairStrategy struct{}

func (rollingPairStrategy) Run(ctx context.Context, comp config.Component, env map[string]string) ([]StageResult, error) {
	currentEnv := map[string]string{
		"ZDX_PHASE": "current",
		"ZDX_SLOT":  env["ZDX_SLOT_A"],
	}
	results, err := runStages(ctx, comp, env, currentEnv, comp.Ship.Stages)
	if err != nil {
		return results, err
	}

	nextEnv := map[string]string{
		"ZDX_PHASE": "next",
		"ZDX_SLOT":  env["ZDX_SLOT_B"],
	}
	nextResults, err := runStages(ctx, comp, env, nextEnv, comp.Ship.Stages)
	results = append(results, nextResults...)
	if err != nil {
		return results, err
	}
	return results, nil
}
