package e2e

import (
	"net/http"
	"testing"
)

func TestBlockerQuestionAdd(t *testing.T) {
	// Ensure project exists.
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-bq", "name": "E2E Blocker Questions"},
		nil,
	)

	var q struct {
		ID         int32    `json:"id"`
		TargetType string   `json:"target_type"`
		TargetID   string   `json:"target_id"`
		Context    string   `json:"context"`
		Choices    []string `json:"choices"`
		Status     string   `json:"status"`
	}
	resp := apiDo(t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{
			"slug":        "e2e-bq",
			"target_type": "issue",
			"target_id":   "IS-1",
			"context":     "Should we use postgres or sqlite?",
			"choices":     []string{"postgres", "sqlite"},
		},
		&q,
	)
	mustOK(t, resp)

	if q.ID == 0 {
		t.Fatal("expected non-zero BQ ID")
	}
	if q.TargetType != "issue" {
		t.Errorf("target_type: want %q got %q", "issue", q.TargetType)
	}
	if q.TargetID != "IS-1" {
		t.Errorf("target_id: want %q got %q", "IS-1", q.TargetID)
	}
	if q.Status != "pending" {
		t.Errorf("status: want %q got %q", "pending", q.Status)
	}
	if q.Context != "Should we use postgres or sqlite?" {
		t.Errorf("context: want question text, got %q", q.Context)
	}
	if len(q.Choices) != 2 {
		t.Errorf("choices: want 2, got %d", len(q.Choices))
	}
}
