// Package ship is the execution layer for the ship harness defined in
// internal/config (Component.Ship). It runs declared stages in order,
// optionally over ssh, and returns per-stage results.
package ship

import (
	"context"
	"os/exec"
	"strings"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// StageResult records one stage's outcome.
type StageResult struct {
	Name     string
	Status   string // "ok" | "failed" | "skipped"
	Duration time.Duration
	Log      string
	Finalize bool // true when the stage was declared with finalize: true
}

// Standard ZDX_* env keys the harness understands. Callers pass values
// in the env map; the runner does not synthesize them. Keys are listed
// here for documentation; nothing in this file references them by name.
//
//	ZDX_DEPLOY_DIR  — remote directory the artifact is unpacked to
//	ZDX_SSH_HOST    — canonical ssh target (informational; Stage.Target overrides per-stage)
//	ZDX_SUDO        — "sudo" or "" — prefix for privileged remote commands
//	ZDX_REMOTE_HOME — remote $HOME (avoids needing a login shell on the far side)
//	ZDX_SERVICE_URL — health-check URL the deploy stage probes after restart
//
// Env precedence for local stages (last wins): os.Environ() < caller env < Ship.Env.
// Ship.Env wins because the project author's intent (config) outranks ambient shell state.
//
// Env over ssh: NOT propagated. sshd resets the environment by default. Callers that need
// vars on the remote must inline them into Stage.Run (e.g. `FOO=bar cmd`) or use ssh -o SendEnv
// out-of-band — the harness keeps the default minimal.
//
// RunOptions carries caller-level flags to strategy implementations.
type RunOptions struct {
	// NoResume forces a full re-run, deleting any saved stage-state files
	// before starting (rolling-pair strategy only).
	NoResume bool
	// ComponentName is the config map key for the component being deployed.
	// Used by resume-capable strategies to scope state files per component.
	ComponentName string
	// StateDir overrides the default .zdx/ship-state directory used for
	// resume state files. Primarily useful in tests for hermetic isolation.
	StateDir string
}

// Run dispatches to the Strategy implementation selected by
// comp.Ship.Strategy; an empty value means "simple" (single pass, no
// extra env). Per-stage execution lives in runStages (strategy.go).
func Run(ctx context.Context, comp config.Component, env map[string]string, opts RunOptions) ([]StageResult, error) {
	return dispatch(comp).Run(ctx, comp, env, opts)
}

// buildCmd constructs the *exec.Cmd for a stage without running it.
// Local stages run via `sh -c <Run>`. Remote stages (Target != "") run
// via `ssh -o StrictHostKeyChecking=no -o BatchMode=yes <Target> <Run>`;
// BatchMode=yes prevents password prompts from hanging the harness.
// The Run string is passed verbatim as the final ssh argv — quoting is
// the caller's responsibility, matching local sh -c semantics.
func buildCmd(ctx context.Context, stage config.Stage) *exec.Cmd {
	if stage.Target == "" {
		return exec.CommandContext(ctx, "sh", "-c", stage.Run)
	}
	return exec.CommandContext(ctx, "ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "BatchMode=yes",
		stage.Target,
		stage.Run,
	)
}

// mergeEnv flattens layered env maps into the KEY=VAL slice exec.Cmd.Env wants.
// Later layers overwrite earlier ones.
func mergeEnv(base []string, layers ...map[string]string) []string {
	merged := make(map[string]string, len(base))
	for _, kv := range base {
		if i := strings.IndexByte(kv, '='); i > 0 {
			merged[kv[:i]] = kv[i+1:]
		}
	}
	for _, layer := range layers {
		for k, v := range layer {
			merged[k] = v
		}
	}
	out := make([]string, 0, len(merged))
	for k, v := range merged {
		out = append(out, k+"="+v)
	}
	return out
}
