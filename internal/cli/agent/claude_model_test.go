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

// resolveComplexityModel must prefer non-empty server slots over local defaults.
func TestResolveComplexityModelUsesServerConfig(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/admin/llm-configs" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []map[string]string{
				{
					"model_low":    "srv-haiku",
					"model_medium": "srv-sonnet",
					"model_high":   "srv-opus",
				},
			},
		})
	}))
	defer srv.Close()

	rc := remoteConfig{url: srv.URL, slug: "demo", key: "k"}
	cfg := config.AgentConfig{ClaudeModel: "ignored-because-server-set"}

	if got := resolveComplexityModel(rc, "low", cfg); got != "srv-haiku" {
		t.Fatalf("low: got %q, want srv-haiku", got)
	}
	if got := resolveComplexityModel(rc, "medium", cfg); got != "srv-sonnet" {
		t.Fatalf("medium: got %q, want srv-sonnet", got)
	}
	if got := resolveComplexityModel(rc, "h", cfg); got != "srv-opus" {
		t.Fatalf("h alias: got %q, want srv-opus", got)
	}
}

// Empty server slots must fall through to defaults; the medium slot honors
// agentCfg.ClaudeModel ahead of the hard-coded default.
func TestResolveComplexityModelFallbackChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []map[string]string{
				{"model_low": "", "model_medium": "", "model_high": ""},
			},
		})
	}))
	defer srv.Close()

	rc := remoteConfig{url: srv.URL, slug: "demo", key: "k"}

	if got := resolveComplexityModel(rc, "low", config.AgentConfig{}); got != defaultModelLow {
		t.Fatalf("low fallback: got %q, want %q", got, defaultModelLow)
	}
	if got := resolveComplexityModel(rc, "med", config.AgentConfig{ClaudeModel: "legacy"}); got != "legacy" {
		t.Fatalf("medium fallback to agentCfg: got %q, want legacy", got)
	}
	if got := resolveComplexityModel(rc, "medium", config.AgentConfig{}); got != defaultModelMedium {
		t.Fatalf("medium default: got %q, want %q", got, defaultModelMedium)
	}
	if got := resolveComplexityModel(rc, "high", config.AgentConfig{}); got != defaultModelHigh {
		t.Fatalf("high fallback: got %q, want %q", got, defaultModelHigh)
	}
}

// The resolver must walk multiple configs in priority order, returning the
// first non-empty slot for the requested tier — empty slots "fall through"
// to the next config.
func TestResolveComplexityModelWalksPriorityOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []map[string]string{
				// priority 1: only high is set
				{"model_low": "", "model_medium": "", "model_high": "p1-opus"},
				// priority 2: low + medium set
				{"model_low": "p2-haiku", "model_medium": "p2-sonnet", "model_high": ""},
			},
		})
	}))
	defer srv.Close()

	rc := remoteConfig{url: srv.URL, slug: "demo", key: "k"}

	if got := resolveComplexityModel(rc, "low", config.AgentConfig{}); got != "p2-haiku" {
		t.Fatalf("low: got %q, want p2-haiku (falls through to priority-2 config)", got)
	}
	if got := resolveComplexityModel(rc, "medium", config.AgentConfig{}); got != "p2-sonnet" {
		t.Fatalf("medium: got %q, want p2-sonnet", got)
	}
	if got := resolveComplexityModel(rc, "high", config.AgentConfig{}); got != "p1-opus" {
		t.Fatalf("high: got %q, want p1-opus (first config wins)", got)
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
