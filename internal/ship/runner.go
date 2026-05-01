// Package ship is the execution layer for the ship harness defined in
// internal/config (Component.Ship). It runs declared stages in order,
// optionally over ssh, and returns per-stage results.
package ship

import (
	"context"
	"fmt"
	"os"
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
func Run(ctx context.Context, comp config.Component, env map[string]string) ([]StageResult, error) {
	results := make([]StageResult, 0, len(comp.Ship.Stages))
	for _, stage := range comp.Ship.Stages {
		start := time.Now()
		cmd := buildCmd(ctx, stage)
		if stage.Target == "" {
			cmd.Env = mergeEnv(os.Environ(), env, comp.Ship.Env)
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
		results = append(results, res)
	}
	return results, nil
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
