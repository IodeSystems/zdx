package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

// modelSelector.resolve must honor precedence: --model > --level > balance.
func TestModelSelectorPrecedence(t *testing.T) {
	rc := remoteConfig{} // invalid → fetchLLMConfig returns empty, level falls to defaults

	cases := []struct {
		name string
		sel  modelSelector
		idx  int
		want string
	}{
		{
			name: "explicit model wins over level",
			sel:  modelSelector{modelFlag: "claude-custom", levelFlag: "high"},
			want: "claude-custom",
		},
		{
			name: "level=low maps to default low when LLM config unset",
			sel:  modelSelector{levelFlag: "low"},
			want: defaultModelLow,
		},
		{
			name: "level=med maps to default med when no agentCfg.ClaudeModel",
			sel:  modelSelector{levelFlag: "med"},
			want: defaultModelMed,
		},
		{
			name: "level=med honors agentCfg.ClaudeModel as fallback",
			sel:  modelSelector{levelFlag: "med", agentCfg: config.AgentConfig{ClaudeModel: "legacy-sonnet"}},
			want: "legacy-sonnet",
		},
		{
			name: "level=high maps to default high",
			sel:  modelSelector{levelFlag: "high"},
			want: defaultModelHigh,
		},
		{
			name: "no model/level + idx 0 → med (sonnet)",
			sel:  modelSelector{},
			idx:  0,
			want: defaultModelMed,
		},
		{
			name: "no model/level + idx 1 → high (opus)",
			sel:  modelSelector{},
			idx:  1,
			want: defaultModelHigh,
		},
		{
			name: "no model/level + idx 4 → med (sonnet)",
			sel:  modelSelector{},
			idx:  4,
			want: defaultModelMed,
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

// resolveLevelModel must prefer non-empty server slots over local defaults.
func TestResolveLevelModelUsesServerConfig(t *testing.T) {
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

	if got := resolveLevelModel(rc, "low", cfg); got != "srv-haiku" {
		t.Fatalf("low: got %q, want srv-haiku", got)
	}
	if got := resolveLevelModel(rc, "medium", cfg); got != "srv-sonnet" {
		t.Fatalf("medium: got %q, want srv-sonnet", got)
	}
	if got := resolveLevelModel(rc, "h", cfg); got != "srv-opus" {
		t.Fatalf("h alias: got %q, want srv-opus", got)
	}
}

// Empty server slots must fall through to defaults; the med slot honors
// agentCfg.ClaudeModel ahead of the hard-coded default.
func TestResolveLevelModelFallbackChain(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []map[string]string{
				{"model_low": "", "model_medium": "", "model_high": ""},
			},
		})
	}))
	defer srv.Close()

	rc := remoteConfig{url: srv.URL, slug: "demo", key: "k"}

	if got := resolveLevelModel(rc, "low", config.AgentConfig{}); got != defaultModelLow {
		t.Fatalf("low fallback: got %q, want %q", got, defaultModelLow)
	}
	if got := resolveLevelModel(rc, "med", config.AgentConfig{ClaudeModel: "legacy"}); got != "legacy" {
		t.Fatalf("med fallback to agentCfg: got %q, want legacy", got)
	}
	if got := resolveLevelModel(rc, "med", config.AgentConfig{}); got != defaultModelMed {
		t.Fatalf("med default: got %q, want %q", got, defaultModelMed)
	}
	if got := resolveLevelModel(rc, "high", config.AgentConfig{}); got != defaultModelHigh {
		t.Fatalf("high fallback: got %q, want %q", got, defaultModelHigh)
	}
}

// The resolver must walk multiple configs in priority order, returning the
// first non-empty slot for the requested level — empty slots "fall through"
// to the next config.
func TestResolveLevelModelWalksPriorityOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"configs": []map[string]string{
				// priority 1: only high is set
				{"model_low": "", "model_medium": "", "model_high": "p1-opus"},
				// priority 2: low + med set
				{"model_low": "p2-haiku", "model_medium": "p2-sonnet", "model_high": ""},
			},
		})
	}))
	defer srv.Close()

	rc := remoteConfig{url: srv.URL, slug: "demo", key: "k"}

	if got := resolveLevelModel(rc, "low", config.AgentConfig{}); got != "p2-haiku" {
		t.Fatalf("low: got %q, want p2-haiku (falls through to priority-2 config)", got)
	}
	if got := resolveLevelModel(rc, "med", config.AgentConfig{}); got != "p2-sonnet" {
		t.Fatalf("med: got %q, want p2-sonnet", got)
	}
	if got := resolveLevelModel(rc, "high", config.AgentConfig{}); got != "p1-opus" {
		t.Fatalf("high: got %q, want p1-opus (first config wins)", got)
	}
}

func TestNormalizeLevel(t *testing.T) {
	cases := map[string]string{
		"low":    "low",
		"LOW":    "low",
		" l ":    "low",
		"medium": "med",
		"med":    "med",
		"M":      "med",
		"high":   "high",
		"H":      "high",
		"":       "",
		"xyz":    "",
	}
	for in, want := range cases {
		if got := normalizeLevel(in); got != want {
			t.Errorf("normalizeLevel(%q) = %q, want %q", in, got, want)
		}
	}
}

// Unknown level returns "" so the caller can let claude CLI pick its default
// rather than silently substituting a wrong tier.
func TestResolveLevelModelUnknownLevel(t *testing.T) {
	rc := remoteConfig{}
	if got := resolveLevelModel(rc, "extreme", config.AgentConfig{}); got != "" {
		t.Fatalf("unknown level: got %q, want empty", got)
	}
}
