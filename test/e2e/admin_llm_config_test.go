package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

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

func TestAdminLLMConfigClaudeEmbeddingRejected(t *testing.T) {
	var created struct {
		ID int64 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
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

	resp := apiDo(t, http.MethodPut, fmt.Sprintf("/api/admin/llm-configs/%d", created.ID), map[string]any{
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
