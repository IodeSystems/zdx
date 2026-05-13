package agent

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// xhighFallthroughWarn / maxFallthroughWarn fire once per process when a
// session asks for the xhigh / max tier but no admin LLM config has the
// matching slot populated. The fall-through to model_high is intentional
// (preserves existing rows that pre-date IS-1175), but silent degradation
// hides operator mistakes, so the warning is mandatory.
var (
	xhighFallthroughWarn sync.Once
	maxFallthroughWarn   sync.Once
)

// applyComplexityModel overrides llmCfg.Model with the matching slot from the
// server's zdx_llm_configs (set via admin/llm). Walks configs in priority
// order and uses the first non-empty slot for the requested complexity; on
// any error the local config's Model is kept so the agent still has a
// working default.
//
// IS-1176: for the xhigh and max tiers, an empty slot falls through to
// model_high within the same priority row and emits a once-per-session
// warning so operators notice unconfigured rows. Existing rows with only
// low/medium/high populated keep working without churn.
//
// Used by the openai adapter and any future adapter that resolves models
// out of the shared admin/llm-configs catalog. Not adapter-specific despite
// living in this file (kept here so the helper is co-located with the
// historical local-provider code that originally introduced it; will move
// to a dedicated llm-resolver file once the catalog rework lands).
func applyComplexityModel(ctx context.Context, llmCfg config.LLMLocal, complexity string) config.LLMLocal {
	c, err := cli.DefaultClient()
	if err != nil {
		return llmCfg
	}
	resp, err := c.ListLlmConfigsWithResponse(ctx)
	if err != nil || resp == nil || resp.JSON200 == nil || resp.JSON200.Configs == nil {
		return llmCfg
	}
	if model := pickComplexityModel(*resp.JSON200.Configs, complexity, defaultFallthroughWarn); model != "" {
		llmCfg.Model = model
	}
	return llmCfg
}

// pickComplexityModel walks configs in their existing order and returns the
// first non-empty slot for the requested tier. For xhigh / max it falls
// through to the configured model_high when the dedicated slot is empty and
// invokes warnFn once per (tier, fall-through) pair to surface the
// degradation. Pure — owns no I/O — so unit tests can exercise the matrix.
//
// warnFn is invoked at most once per tier across the process lifetime
// (default: defaultFallthroughWarn, which prints to stderr); tests pass a
// capturing fn.
func pickComplexityModel(configs []dxclient.LLMConfigBody, complexity string, warnFn func(tier string)) string {
	for _, cfg := range configs {
		var picked string
		switch complexity {
		case ComplexityLow:
			picked = cfg.ModelLow
		case ComplexityHigh:
			picked = cfg.ModelHigh
		case ComplexityXHigh:
			if v := derefStr(cfg.ModelXhigh); v != "" {
				picked = v
			} else if cfg.ModelHigh != "" {
				warnFn(ComplexityXHigh)
				picked = cfg.ModelHigh
			}
		case ComplexityMax:
			if v := derefStr(cfg.ModelMax); v != "" {
				picked = v
			} else if cfg.ModelHigh != "" {
				warnFn(ComplexityMax)
				picked = cfg.ModelHigh
			}
		default:
			picked = cfg.ModelMedium
		}
		if picked != "" {
			return picked
		}
	}
	return ""
}

// defaultFallthroughWarn is the production warnFn for pickComplexityModel.
// It prints to stderr at most once per (tier, process) pair so a loop that
// resolves dozens of sessions doesn't spam the operator.
func defaultFallthroughWarn(tier string) {
	switch tier {
	case ComplexityXHigh:
		xhighFallthroughWarn.Do(func() {
			fmt.Fprintf(os.Stderr,
				"[agent] WARN: --complexity=xhigh has no model_xhigh configured on any LLM config row; falling through to model_high. Configure model_xhigh in admin/llm to silence.\n")
		})
	case ComplexityMax:
		maxFallthroughWarn.Do(func() {
			fmt.Fprintf(os.Stderr,
				"[agent] WARN: --complexity=max has no model_max configured on any LLM config row; falling through to model_high. Configure model_max in admin/llm to silence.\n")
		})
	}
}

func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
