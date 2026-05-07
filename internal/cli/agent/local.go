package agent

import (
	"context"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
)

// applyComplexityModel overrides llmCfg.Model with the matching slot from the
// server's zdx_llm_configs (set via admin/llm). Walks configs in priority
// order and uses the first non-empty slot for the requested complexity; on
// any error the local config's Model is kept so the agent still has a
// working default.
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
	for _, cfg := range *resp.JSON200.Configs {
		var picked string
		switch complexity {
		case "low":
			picked = cfg.ModelLow
		case "high":
			picked = cfg.ModelHigh
		default:
			picked = cfg.ModelMedium
		}
		if picked != "" {
			llmCfg.Model = picked
			return llmCfg
		}
	}
	return llmCfg
}
