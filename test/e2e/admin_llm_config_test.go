package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoAPI_AdminLLMConfigMultiple demonstrates creating multiple LLM configs
// each with a distinct agent_type (openai/local/claude), low/med/high model
// slots, and an auto-assigned priority order — covering spec 95.
func TestDemoAPI_AdminLLMConfigMultiple(t *testing.T) {
	rec := newApiRecorder(t, "admin-llm-config-multiple")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_llm_config_test.go", Note: "API demo source"})
	t.Cleanup(rec.Save)

	configs := []struct {
		name      string
		agentType string
		ltype     string
		low       string
		medium    string
		high      string
		embedding string
	}{
		{
			name: "demo-openai", agentType: "openai", ltype: "openai",
			low: "gpt-4o-mini", medium: "gpt-4o-mini", high: "gpt-4o",
			embedding: "text-embedding-3-small",
		},
		{
			name: "demo-local", agentType: "local", ltype: "openai",
			low: "llama3.2", medium: "llama3.3", high: "deepseek-r1",
			embedding: "nomic-embed-text",
		},
		{
			name: "demo-claude", agentType: "claude", ltype: "claude",
			low: "claude-haiku-4-5", medium: "claude-sonnet-4-6", high: "claude-opus-4-7",
		},
	}

	var ids []int64
	var priorities []int32
	for _, cfg := range configs {
		var out struct {
			ID        int64  `json:"id"`
			Priority  int32  `json:"priority"`
			AgentType string `json:"agent_type"`
			ModelLow  string `json:"model_low"`
			ModelHigh string `json:"model_high"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/admin/llm-configs", map[string]any{
			"name":            cfg.name,
			"type":            cfg.ltype,
			"agent_type":      cfg.agentType,
			"url":             "",
			"embedding_model": cfg.embedding,
			"model_low":       cfg.low,
			"model_medium":    cfg.medium,
			"model_high":      cfg.high,
			"priority":        0,
		}, &out))
		if out.AgentType != cfg.agentType {
			t.Errorf("%s: agent_type want %q got %q", cfg.name, cfg.agentType, out.AgentType)
		}
		if out.ModelLow != cfg.low {
			t.Errorf("%s: model_low want %q got %q", cfg.name, cfg.low, out.ModelLow)
		}
		if out.ModelHigh != cfg.high {
			t.Errorf("%s: model_high want %q got %q", cfg.name, cfg.high, out.ModelHigh)
		}
		ids = append(ids, out.ID)
		priorities = append(priorities, out.Priority)
	}
	t.Cleanup(func() {
		for _, id := range ids {
			apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", id), nil, nil)
		}
	})

	// Each successive config gets a higher priority number (lower importance = later in resolution).
	for i := 1; i < len(priorities); i++ {
		if priorities[i-1] >= priorities[i] {
			t.Errorf("priority order: config[%d]=%d >= config[%d]=%d", i-1, priorities[i-1], i, priorities[i])
		}
	}

	// List returns configs sorted by ascending priority.
	var list struct {
		Configs []struct {
			ID       int64 `json:"id"`
			Priority int32 `json:"priority"`
		} `json:"configs"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/admin/llm-configs", nil, &list))
	for i := 1; i < len(list.Configs); i++ {
		if list.Configs[i-1].Priority >= list.Configs[i].Priority {
			t.Errorf("list not in priority order at index %d", i)
		}
	}
}

func TestAdminLLMConfigCRUD(t *testing.T) {
	var created struct {
		ID       int64  `json:"id"`
		Name     string `json:"name"`
		Priority int32  `json:"priority"`
		Type     string `json:"type"`
		ModelLow string `json:"model_low"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "e2e-openai",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "text-embedding-ada-002",
		"model_low":       "gpt-4o-mini",
		"model_medium":    "gpt-4o-mini",
		"model_high":      "gpt-4o",
		"priority":        0,
	}, &created))
	if created.Name != "e2e-openai" {
		t.Errorf("name: want %q got %q", "e2e-openai", created.Name)
	}
	if created.Priority == 0 {
		t.Error("expected non-zero priority")
	}

	var list struct {
		Configs []struct {
			ID       int64  `json:"id"`
			Name     string `json:"name"`
			Priority int32  `json:"priority"`
		} `json:"configs"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/admin/llm-configs", nil, &list))
	found := false
	for _, c := range list.Configs {
		if c.ID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("created config id=%d not in list", created.ID)
	}

	var updated struct {
		ModelLow  string `json:"model_low"`
		ModelHigh string `json:"model_high"`
	}
	mustOK(t, apiDo(t, http.MethodPut, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), map[string]any{
		"name":            "e2e-openai",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "text-embedding-ada-002",
		"model_low":       "gpt-3.5-turbo",
		"model_medium":    "gpt-3.5-turbo",
		"model_high":      "gpt-4o",
		"priority":        created.Priority,
	}, &updated))
	if updated.ModelLow != "gpt-3.5-turbo" {
		t.Errorf("model_low after update: want %q got %q", "gpt-3.5-turbo", updated.ModelLow)
	}

	resp := apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), nil, nil)
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		t.Errorf("delete: want 204/200, got %d", resp.StatusCode)
	}
}

// TestAdminLLMConfigXHighMaxRoundTrip verifies that the two extended
// complexity tier slots (xhigh, max) introduced in IS-1175 round-trip
// through Create / Get / Update / List and that the fields are optional
// on the wire (omitted in the create body still succeeds).
func TestAdminLLMConfigXHighMaxRoundTrip(t *testing.T) {
	var created struct {
		ID         int64  `json:"id"`
		ModelXHigh string `json:"model_xhigh"`
		ModelMax   string `json:"model_max"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "e2e-xhigh-max",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "text-embedding-3-small",
		"model_low":       "gpt-4o-mini",
		"model_medium":    "gpt-4o-mini",
		"model_high":      "gpt-4o",
		"model_xhigh":     "o1",
		"model_max":       "o1-pro",
		"priority":        0,
	}, &created))
	t.Cleanup(func() {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), nil, nil)
	})
	if created.ModelXHigh != "o1" {
		t.Errorf("create: model_xhigh want %q got %q", "o1", created.ModelXHigh)
	}
	if created.ModelMax != "o1-pro" {
		t.Errorf("create: model_max want %q got %q", "o1-pro", created.ModelMax)
	}

	var fetched struct {
		ModelXHigh string `json:"model_xhigh"`
		ModelMax   string `json:"model_max"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), nil, &fetched))
	if fetched.ModelXHigh != "o1" || fetched.ModelMax != "o1-pro" {
		t.Errorf("get: xhigh/max want o1/o1-pro got %q/%q", fetched.ModelXHigh, fetched.ModelMax)
	}

	// Absent new fields on update should clear the columns (round-trip
	// empty → empty), proving they are optional rather than required.
	var updated struct {
		ModelXHigh string `json:"model_xhigh"`
		ModelMax   string `json:"model_max"`
	}
	mustOK(t, apiDo(t, http.MethodPut, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), map[string]any{
		"name":            "e2e-xhigh-max",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "text-embedding-3-small",
		"model_low":       "gpt-4o-mini",
		"model_medium":    "gpt-4o-mini",
		"model_high":      "gpt-4o",
		"priority":        0,
	}, &updated))
	if updated.ModelXHigh != "" || updated.ModelMax != "" {
		t.Errorf("update (omitted): xhigh/max want empty got %q/%q", updated.ModelXHigh, updated.ModelMax)
	}
}

// TestDemoAPI_AdminLLMConfigClaudeEmbeddingRejected demonstrates that a config
// with agent_type=claude rejects any non-null embedding_model on update — covering spec 96.
func TestDemoAPI_AdminLLMConfigClaudeEmbeddingRejected(t *testing.T) {
	rec := newApiRecorder(t, "admin-llm-config-claude-embedding-rejected")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_llm_config_test.go", Note: "API demo source"})
	t.Cleanup(rec.Save)

	var created struct {
		ID int64 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "e2e-claude",
		"type":            "claude",
		"agent_type":      "claude",
		"url":             "",
		"embedding_model": "",
		"model_low":       "claude-haiku-4-5",
		"model_medium":    "claude-sonnet-4-6",
		"model_high":      "claude-opus-4-7",
		"priority":        0,
	}, &created))
	if created.ID == 0 {
		t.Fatal("expected non-zero ID for created claude config")
	}
	t.Cleanup(func() {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), nil, nil)
	})

	resp := rec.Do(http.MethodPut, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), map[string]any{
		"name":            "e2e-claude",
		"type":            "claude",
		"agent_type":      "claude",
		"url":             "",
		"embedding_model": "some-model",
		"model_low":       "claude-haiku-4-5",
		"model_medium":    "claude-sonnet-4-6",
		"model_high":      "claude-opus-4-7",
		"priority":        0,
	}, nil)
	if resp.StatusCode < 400 {
		t.Errorf("expected error when setting embedding_model on claude config, got %d", resp.StatusCode)
	}
}

// TestDemoAPI_AdminLLMConfigLevelResolution demonstrates spec 97: with multiple
// configs, resolving a complexity level (low/med/high) walks configs in priority
// order and returns the first non-empty slot at that level. The recorded trace
// shows the setup (two configs with a deliberate gap pattern) and the list
// response that the client-side resolver walks.
func TestDemoAPI_AdminLLMConfigLevelResolution(t *testing.T) {
	rec := newApiRecorder(t, "admin-llm-config-level-resolution")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_llm_config_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/claude_model.go", Note: "resolveLevelModel implementation"})
	t.Cleanup(rec.Save)

	// Priority 1: only high is set. Priority 2: low + med set, high empty.
	// Resolver should pick p1 for high, p2 for low and med.
	var p1 struct {
		ID       int64 `json:"id"`
		Priority int32 `json:"priority"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "demo-resolve-p1",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "",
		"model_low":       "",
		"model_medium":    "",
		"model_high":      "p1-opus",
		"priority":        0,
	}, &p1))

	var p2 struct {
		ID       int64 `json:"id"`
		Priority int32 `json:"priority"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "demo-resolve-p2",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "",
		"model_low":       "p2-haiku",
		"model_medium":    "p2-sonnet",
		"model_high":      "",
		"priority":        0,
	}, &p2))

	t.Cleanup(func() {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", p1.ID), nil, nil)
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", p2.ID), nil, nil)
	})

	if p1.Priority >= p2.Priority {
		t.Fatalf("priority order: p1=%d must be < p2=%d", p1.Priority, p2.Priority)
	}

	var list struct {
		Configs []struct {
			ID          int64  `json:"id"`
			Priority    int32  `json:"priority"`
			ModelLow    string `json:"model_low"`
			ModelMedium string `json:"model_medium"`
			ModelHigh   string `json:"model_high"`
		} `json:"configs"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/admin/llm-configs", nil, &list))

	// Walk the list like resolveLevelModel does: return the first non-empty
	// slot at the requested level.
	resolve := func(level string) string {
		for _, c := range list.Configs {
			if c.ID != p1.ID && c.ID != p2.ID {
				continue
			}
			var slot string
			switch level {
			case "low":
				slot = c.ModelLow
			case "med":
				slot = c.ModelMedium
			case "high":
				slot = c.ModelHigh
			}
			if slot != "" {
				return slot
			}
		}
		return ""
	}

	if got := resolve("low"); got != "p2-haiku" {
		t.Errorf("low: got %q, want p2-haiku (falls through p1 to p2)", got)
	}
	if got := resolve("med"); got != "p2-sonnet" {
		t.Errorf("med: got %q, want p2-sonnet (falls through p1 to p2)", got)
	}
	if got := resolve("high"); got != "p1-opus" {
		t.Errorf("high: got %q, want p1-opus (first non-empty wins)", got)
	}
}

func TestAdminLLMConfigPriorityResolution(t *testing.T) {
	var cfg1 struct {
		ID       int64 `json:"id"`
		Priority int32 `json:"priority"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "e2e-priority-1",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "",
		"model_low":       "gpt-4o-mini",
		"model_medium":    "",
		"model_high":      "",
		"priority":        0,
	}, &cfg1))

	var cfg2 struct {
		ID       int64 `json:"id"`
		Priority int32 `json:"priority"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "e2e-priority-2",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             "",
		"embedding_model": "",
		"model_low":       "gpt-3.5-turbo",
		"model_medium":    "",
		"model_high":      "",
		"priority":        0,
	}, &cfg2))

	t.Cleanup(func() {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", cfg1.ID), nil, nil)
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", cfg2.ID), nil, nil)
	})

	if cfg1.Priority >= cfg2.Priority {
		t.Errorf("expected cfg2 priority > cfg1: cfg1=%d cfg2=%d", cfg1.Priority, cfg2.Priority)
	}

	var list struct {
		Configs []struct {
			ID       int64 `json:"id"`
			Priority int32 `json:"priority"`
		} `json:"configs"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/admin/llm-configs", nil, &list))
	for i := 1; i < len(list.Configs); i++ {
		if list.Configs[i-1].Priority >= list.Configs[i].Priority {
			t.Errorf("configs not in priority order: [%d].priority=%d >= [%d].priority=%d",
				i-1, list.Configs[i-1].Priority, i, list.Configs[i].Priority)
		}
	}
}
