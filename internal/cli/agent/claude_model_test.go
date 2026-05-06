package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

// modelSelector.resolve must honor precedence: --model > --complexity > balance.
func TestModelSelectorPrecedence(t *testing.T) {
	rc := remoteConfig{} // invalid → fetchLLMConfig returns empty, complexity falls to defaults

	cases := []struct {
		name string
		sel  modelSelector
		idx  int
		want string
	}{
		{
			name: "explicit model wins over complexity",
			sel:  modelSelector{modelFlag: "claude-custom", complexity: "high"},
			want: "claude-custom",
		},
		{
			name: "complexity=low maps to default low when LLM config unset",
			sel:  modelSelector{complexity: "low"},
			want: defaultModelLow,
		},
		{
			name: "complexity=medium maps to default medium when no agentCfg.ClaudeModel",
			sel:  modelSelector{complexity: "medium"},
			want: defaultModelMedium,
		},
		{
			name: "complexity=medium honors agentCfg.ClaudeModel as fallback",
			sel:  modelSelector{complexity: "medium", agentCfg: config.AgentConfig{ClaudeModel: "legacy-sonnet"}},
			want: "legacy-sonnet",
		},
		{
			name: "complexity=high maps to default high",
			sel:  modelSelector{complexity: "high"},
			want: defaultModelHigh,
		},
		{
			name: "no model/complexity + idx 0 → medium (sonnet)",
			sel:  modelSelector{},
			idx:  0,
			want: defaultModelMedium,
		},
		{
			name: "no model/complexity + idx 1 → high (opus)",
			sel:  modelSelector{},
			idx:  1,
			want: defaultModelHigh,
		},
		{
			name: "no model/complexity + idx 4 → medium (sonnet)",
			sel:  modelSelector{},
			idx:  4,
			want: defaultModelMedium,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.sel.resolve(rc, tc.idx)
			if got != tc.want {
				t.Fatalf("resolve: got %q, want %q", got, tc.want)
			}
		})
	}
}

// IS-1034: claude must NOT consult the project's generic admin/llm-configs
// table — those slots are populated for opencode/local generic LLM endpoints
// (often non-claude models like Qwen). resolveComplexityModel returns the
// hardcoded claude-* defaults regardless of what the server's table contains.
func TestResolveComplexityModelIgnoresServerConfig(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []map[string]string{
				{
					"model_low":    "Qwen3-6-27B",
					"model_medium": "Qwen3-6-27B",
					"model_high":   "Qwen3-6-27B",
				},
			},
		})
	}))
	defer srv.Close()

	rc := remoteConfig{url: srv.URL, slug: "demo", key: "k"}

	if got := resolveComplexityModel(rc, "low", config.AgentConfig{}); got != defaultModelLow {
		t.Errorf("low: got %q, want %q (hardcoded)", got, defaultModelLow)
	}
	if got := resolveComplexityModel(rc, "medium", config.AgentConfig{}); got != defaultModelMedium {
		t.Errorf("medium: got %q, want %q (hardcoded)", got, defaultModelMedium)
	}
	if got := resolveComplexityModel(rc, "high", config.AgentConfig{}); got != defaultModelHigh {
		t.Errorf("high: got %q, want %q (hardcoded)", got, defaultModelHigh)
	}
	if called {
		t.Errorf("resolveComplexityModel should not contact the server's admin/llm-configs endpoint; called=%v", called)
	}
}

// agentCfg.ClaudeModel (the legacy .zdx/config.yaml field) overrides the
// medium-tier hardcoded default. Low/high never honor it — those tiers
// have no legacy override.
func TestResolveComplexityModelHonorsAgentCfgClaudeModelForMediumOnly(t *testing.T) {
	rc := remoteConfig{}

	if got := resolveComplexityModel(rc, "med", config.AgentConfig{ClaudeModel: "legacy"}); got != "legacy" {
		t.Errorf("medium with agentCfg.ClaudeModel: got %q, want legacy", got)
	}
	if got := resolveComplexityModel(rc, "low", config.AgentConfig{ClaudeModel: "legacy"}); got != defaultModelLow {
		t.Errorf("low must ignore agentCfg.ClaudeModel: got %q, want %q", got, defaultModelLow)
	}
	if got := resolveComplexityModel(rc, "high", config.AgentConfig{ClaudeModel: "legacy"}); got != defaultModelHigh {
		t.Errorf("high must ignore agentCfg.ClaudeModel: got %q, want %q", got, defaultModelHigh)
	}
}

// Unknown complexity returns "" so the caller can let claude CLI pick its default
// rather than silently substituting a wrong tier.
func TestResolveComplexityModelUnknownTier(t *testing.T) {
	rc := remoteConfig{}
	if got := resolveComplexityModel(rc, "extreme", config.AgentConfig{}); got != "" {
		t.Fatalf("unknown complexity: got %q, want empty", got)
	}
}
