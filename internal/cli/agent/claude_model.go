package agent

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

// Default claude model names used when neither --model nor --complexity is
// specified and when the project's admin LLM config has no slot set for the
// chosen tier. Kept as constants so unit tests can pin the balancing behavior.
const (
	defaultModelLow    = "claude-haiku-4-5"
	defaultModelMedium = "claude-sonnet-4-6"
	defaultModelHigh   = "claude-opus-4-7"
)

// modelSelector chooses which model to pass to the claude CLI per session.
//
// Precedence (highest first):
//  1. modelFlag — verbatim, passed straight through.
//  2. complexity — resolved against admin /llm-config (model_low/medium/high).
//     Empty slots fall through to defaultModelLow/Medium/High; agentCfg.ClaudeModel
//     overrides the medium fallback so legacy .zdx/config.yaml stays honored.
//  3. neither — alternate sonnet/opus per session so token usage is balanced
//     across tiers when complexity is unspecified.
type modelSelector struct {
	modelFlag  string
	complexity string
	agentCfg   config.AgentConfig
}

func (s modelSelector) resolve(rc remoteConfig, sessionIdx int) string {
	if s.modelFlag != "" {
		return s.modelFlag
	}
	if s.complexity != "" {
		return resolveComplexityModel(rc, s.complexity, s.agentCfg)
	}
	return balancedModel(sessionIdx)
}

// balancedModel alternates sonnet/opus per zero-based session index.
// Even indices get sonnet, odd get opus — yields a 50/50 split over a loop run.
func balancedModel(idx int) string {
	if idx%2 == 0 {
		return defaultModelMedium
	}
	return defaultModelHigh
}

// resolveComplexityModel maps low|medium|high to a concrete claude model name.
// It walks the configured admin LLM configs in priority order and returns the
// first non-empty slot matching the requested tier. If no config provides one,
// it falls back to the hardcoded defaults (agentCfg.ClaudeModel overrides the
// medium default so existing config.yaml ClaudeModel keeps working).
func resolveComplexityModel(rc remoteConfig, complexity string, agentCfg config.AgentConfig) string {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return ""
	}
	configs, _ := fetchLLMConfigs(rc)
	for _, cfg := range configs {
		var slot string
		switch tier {
		case ComplexityLow:
			slot = cfg.ModelLow
		case ComplexityMedium:
			slot = cfg.ModelMedium
		case ComplexityHigh:
			slot = cfg.ModelHigh
		}
		if slot != "" {
			return slot
		}
	}
	switch tier {
	case ComplexityLow:
		return defaultModelLow
	case ComplexityMedium:
		if agentCfg.ClaudeModel != "" {
			return agentCfg.ClaudeModel
		}
		return defaultModelMedium
	case ComplexityHigh:
		return defaultModelHigh
	}
	return ""
}

// llmConfigEntry mirrors the subset of LLMConfigBody the level resolver needs.
type llmConfigEntry struct {
	ModelLow    string `json:"model_low"`
	ModelMedium string `json:"model_medium"`
	ModelHigh   string `json:"model_high"`
}

// fullLLMConfigEntry includes URL and API key for adapters that need the
// complete endpoint config (OpenCode, MCP harness) not just model names.
type fullLLMConfigEntry struct {
	ModelLow    string `json:"model_low"`
	ModelMedium string `json:"model_medium"`
	ModelHigh   string `json:"model_high"`
	Url         string `json:"url"`
	ApiKey      string `json:"api_key"`
	Type        string `json:"type"`
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

// fetchFullLLMConfigs pulls the full LLM config including URL and API key.
func fetchFullLLMConfigs(rc remoteConfig) ([]fullLLMConfigEntry, error) {
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
		Configs []fullLLMConfigEntry `json:"configs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	return body.Configs, nil
}

// resolveLLMConfigFromServer looks up the server's LLM config for the given
// complexity tier and returns a fully-resolved LLMLocal (base_url, model,
// api_key). Walks configs in priority order; returns zero LLMLocal on miss.
func resolveLLMConfigFromServer(rc remoteConfig, complexity string) config.LLMLocal {
	tier, err := NormalizeComplexity(complexity)
	if err != nil {
		return config.LLMLocal{}
	}
	configs, _ := fetchFullLLMConfigs(rc)
	for _, cfg := range configs {
		var picked string
		switch tier {
		case ComplexityLow:
			picked = cfg.ModelLow
		case ComplexityMedium:
			picked = cfg.ModelMedium
		case ComplexityHigh:
			picked = cfg.ModelHigh
		}
		if picked != "" && cfg.Url != "" {
			return config.LLMLocal{
				BaseURL:        cfg.Url,
				Model:          picked,
				APIKey:         cfg.ApiKey,
				TimeoutSeconds: 120,
			}
		}
	}
	return config.LLMLocal{}
}
