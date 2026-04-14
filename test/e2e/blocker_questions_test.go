package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func setupBQProject(t *testing.T) {
	t.Helper()
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-bq", "name": "E2E Blocker Questions"},
		nil,
	)
}

func TestBlockerQuestionAdd(t *testing.T) {
	setupBQProject(t)

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

func TestBlockerQuestionAnswer(t *testing.T) {
	setupBQProject(t)

	var created struct {
		ID     int32  `json:"id"`
		Status string `json:"status"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{
			"slug":        "e2e-bq",
			"target_type": "task",
			"target_id":   "TK-1",
			"context":     "Which auth provider?",
		},
		&created,
	))
	if created.Status != "pending" {
		t.Fatalf("initial status: want pending, got %q", created.Status)
	}

	var answered struct {
		ID         int32  `json:"id"`
		Status     string `json:"status"`
		Answer     string `json:"answer"`
		AnsweredBy string `json:"answered_by"`
		AnsweredAt string `json:"answered_at"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/blocker-questions/answer",
		map[string]any{
			"slug":        "e2e-bq",
			"id":          created.ID,
			"answer":      "Use OAuth2 via Google",
			"answered_by": "eng-lead",
		},
		&answered,
	))

	if answered.Status != "answered" {
		t.Errorf("status: want %q got %q", "answered", answered.Status)
	}
	if answered.Answer != "Use OAuth2 via Google" {
		t.Errorf("answer: want expected text, got %q", answered.Answer)
	}
	if answered.AnsweredAt == "" {
		t.Error("answered_at should be set")
	}
}

func TestBlockerQuestionList(t *testing.T) {
	setupBQProject(t)

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{
			"slug":        "e2e-bq",
			"target_type": "feature",
			"target_id":   "auth",
			"context":     "List test question",
		},
		nil,
	))

	var resp struct {
		Questions []struct {
			ID         int32  `json:"id"`
			TargetType string `json:"target_type"`
			TargetID   string `json:"target_id"`
			Status     string `json:"status"`
			Context    string `json:"context"`
		} `json:"questions"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/dx/blocker-questions/list?slug=e2e-bq", nil, &resp))

	if len(resp.Questions) == 0 {
		t.Fatal("expected at least one question")
	}
	found := false
	for _, q := range resp.Questions {
		if q.Context == "List test question" {
			found = true
			if q.TargetType != "feature" {
				t.Errorf("target_type: want %q got %q", "feature", q.TargetType)
			}
			if q.TargetID != "auth" {
				t.Errorf("target_id: want %q got %q", "auth", q.TargetID)
			}
		}
	}
	if !found {
		t.Error("created question not found in list")
	}
}

func TestBlockerQuestionListPending(t *testing.T) {
	setupBQProject(t)

	var pending struct{ ID int32 `json:"id"` }
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{
			"slug":        "e2e-bq",
			"target_type": "issue",
			"target_id":   "IS-99",
			"context":     "Pending filter test",
		},
		&pending,
	))

	var answeredQ struct{ ID int32 `json:"id"` }
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{
			"slug":        "e2e-bq",
			"target_type": "issue",
			"target_id":   "IS-100",
			"context":     "Will be answered",
		},
		&answeredQ,
	))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/blocker-questions/answer",
		map[string]any{
			"slug":   "e2e-bq",
			"id":     answeredQ.ID,
			"answer": "done",
		},
		nil,
	))

	var resp struct {
		Questions []struct {
			ID     int32  `json:"id"`
			Status string `json:"status"`
		} `json:"questions"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/blocker-questions/list?slug=e2e-bq&status=pending"),
		nil, &resp,
	))

	for _, q := range resp.Questions {
		if q.Status != "pending" {
			t.Errorf("BQ-%d: want pending, got %q", q.ID, q.Status)
		}
	}
	found := false
	for _, q := range resp.Questions {
		if q.ID == pending.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("pending question BQ-%d not in filtered list", pending.ID)
	}
}
