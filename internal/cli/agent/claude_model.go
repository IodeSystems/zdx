package agent

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// Default claude model names used when neither --model nor --level is specified
// and when the project's admin LLM config has no slot set for a chosen level.
// Kept as constants so unit tests can pin the balancing behavior.
const (
	defaultModelLow  = "claude-haiku-4-5"
	defaultModelMed  = "claude-sonnet-4-6"
	defaultModelHigh = "claude-opus-4-7"
)

// modelSelector chooses which model to pass to the claude CLI per session.
//
// Precedence (highest first):
//  1. modelFlag — verbatim, passed straight through.
//  2. levelFlag — resolved against admin /llm-config (model_low/medium/high).
//     Empty slots fall through to defaultModelLow/Med/High; agentCfg.ClaudeModel
//     overrides the med fallback so legacy .zdx/config.yaml stays honored.
//  3. neither — alternate sonnet/opus per session so token usage is balanced
//     across tiers when complexity is unspecified.
type modelSelector struct {
	modelFlag string
	levelFlag string
	agentCfg  config.AgentConfig
}

func (s modelSelector) resolve(rc remoteConfig, sessionIdx int) string {
	if s.modelFlag != "" {
		return s.modelFlag
	}
	if s.levelFlag != "" {
		return resolveLevelModel(rc, s.levelFlag, s.agentCfg)
	}
	return balancedModel(sessionIdx)
}

// balancedModel alternates sonnet/opus per zero-based session index.
// Even indices get sonnet, odd get opus — yields a 50/50 split over a loop run.
func balancedModel(idx int) string {
	if idx%2 == 0 {
		return defaultModelMed
	}
	return defaultModelHigh
}

// resolveLevelModel maps low|med|high to a concrete model name. It walks the
// configured admin LLM configs in priority order and returns the first
// non-empty slot matching the requested level. If no config provides one, it
// falls back to defaults (agentCfg.ClaudeModel overrides the med default so
// existing config.yaml ClaudeModel keeps working).
func resolveLevelModel(rc remoteConfig, level string, agentCfg config.AgentConfig) string {
	lvl := normalizeLevel(level)
	if lvl == "" {
		return ""
	}
	configs, _ := fetchLLMConfigs(rc)
	for _, cfg := range configs {
		var slot string
		switch lvl {
		case "low":
			slot = cfg.ModelLow
		case "med":
			slot = cfg.ModelMedium
		case "high":
			slot = cfg.ModelHigh
		}
		if slot != "" {
			return slot
		}
	}
	switch lvl {
	case "low":
		return defaultModelLow
	case "med":
		if agentCfg.ClaudeModel != "" {
			return agentCfg.ClaudeModel
		}
		return defaultModelMed
	case "high":
		return defaultModelHigh
	}
	return ""
}

// normalizeLevel collapses common spellings to "low" / "med" / "high".
// Returns "" for inputs that don't match any tier so callers can detect typos.
func normalizeLevel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low", "l":
		return "low"
	case "med", "medium", "m":
		return "med"
	case "high", "h":
		return "high"
	}
	return ""
}

// llmConfigEntry mirrors the subset of LLMConfigBody the level resolver needs.
type llmConfigEntry struct {
	ModelLow    string `json:"model_low"`
	ModelMedium string `json:"model_medium"`
	ModelHigh   string `json:"model_high"`
}

// fetchLLMConfigs pulls the ordered list of admin LLM configs over HTTP.
// Returns nil + nil error when the server is unreachable, the call 4xx's,
// or the body is missing — callers fall back to defaults.
func fetchLLMConfigs(rc remoteConfig) ([]llmConfigEntry, error) {
	if !rc.valid() {
		return nil, nil
	}
	req, _ := http.NewRequest("GET", rc.url+"/api/admin/llm-configs", nil)
	req.Header.Set("X-Api-Key", rc.key)
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, nil
	}
	var body struct {
		Configs []llmConfigEntry `json:"configs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Configs, nil
}
