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
//	pass 1: ZDX_PHASE=current, ZDX_SLOT=<env[ZDX_SLOT_A]>, HEALTH_PORT=7600
//	pass 2: ZDX_PHASE=next,    ZDX_SLOT=<env[ZDX_SLOT_B]>, HEALTH_PORT=7602
//
// HEALTH_PORT mirrors the per-slot listen ports baked into the systemd
// units (infra/provision/files/etc/systemd/zdx-{current,next}.service).
// Stage scripts (e.g. health-check curl) reference ${HEALTH_PORT}.
//
// ZDX_SLOT_A/ZDX_SLOT_B are expected to be set in the loaded environment
// (typically via home/deploy.secret.properties as zdx.slot_a / zdx.slot_b
// or via the component's ship.env block). They hold the systemd unit
// suffixes — usually "current" and "next" — and feed `systemctl restart
// zdx-${ZDX_SLOT}` and per-slot rsync targets.
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
	main = filterStagesByTag(main, opts.IncludeTag, opts.ExcludeTag)

	passes := []struct {
		phase      string
		slot       string
		healthPort string
	}{
		{"current", env["ZDX_SLOT_A"], "7600"},
		{"next", env["ZDX_SLOT_B"], "7602"},
	}

	var results []StageResult
	var runErr error
	for _, pass := range passes {
		sf := stateFilePath(opts.StateDir, sha, opts.ComponentName, pass.phase)
		if opts.NoResume {
			_ = os.Remove(sf)
		}
		skip, _ := loadCompletedStages(sf) // read error → empty skip set
		for _, s := range opts.SkipStages {
			skip[s] = true
		}
		extraEnv := map[string]string{
			"ZDX_PHASE":   pass.phase,
			"ZDX_SLOT":    pass.slot,
			"HEALTH_PORT": pass.healthPort,
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
