package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// startVariableEmbedMock returns embeddings that vary by input text so the
// cosine similarity between a seeded task and a follow-up query can be
// controlled. Texts containing "alpha" embed to a unit basis vector;
// texts containing "beta" embed to a vector with cosine 0.5 against
// alpha — populates the similar response without crossing the 0.90
// duplicate-block threshold.
func startVariableEmbedMock(t *testing.T) string {
	t.Helper()
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/embeddings" {
			http.NotFound(w, r)
			return
		}
		var req struct {
			Input any `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		var text string
		switch v := req.Input.(type) {
		case string:
			text = v
		case []any:
			for _, s := range v {
				if str, ok := s.(string); ok {
					text += str + " "
				}
			}
		}
		text = strings.ToLower(text)

		var vec []float32
		switch {
		case strings.Contains(text, "alpha"):
			vec = []float32{1, 0, 0, 0, 0, 0, 0, 0}
		case strings.Contains(text, "beta"):
			// |v| = sqrt(0.25 + 0.75) = 1; cos vs alpha = 0.5.
			vec = []float32{0.5, 0.8660254, 0, 0, 0, 0, 0, 0}
		default:
			vec = []float32{0, 0, 1, 0, 0, 0, 0, 0}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": vec}},
		})
	}))
	t.Cleanup(mock.Close)
	return mock.URL
}

// TestTaskTechAddWithoutAutoReady covers spec 74: when tech add is run
// without --auto-ready, the new task is created as "wip", similar open
// tasks are surfaced in the response, and no auto-promotion happens
// until /api/dx/todo/task/ready is called.
func TestTaskTechAddWithoutAutoReady(t *testing.T) {
	url := startVariableEmbedMock(t)
	registerEmbedMock(t, "spec74-llm", url)

	slug := "e2e-tech-add-wip-default"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add WIP Default"}, nil)

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{
			"slug":       slug,
			"title":      "Auth endpoint reference",
			"context":    "context body for issue",
			"auto_ready": true,
		}, &issue))
	issueRef := fmt.Sprintf("IS-%d", issue.ID)

	var seed techAddResp
	seedText := "alpha endpoint reference for /api/auth"
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"issue":      issueRef,
			"text":       seedText,
			"auto_ready": true,
		}, &seed))
	if seed.ID == 0 {
		t.Fatalf("seed: expected task id, got 0")
	}
	seedRef := fmt.Sprintf("TK-%d", seed.ID)
	waitForTaskIndexed(t, slug, seedText)

	var resp struct {
		ID               int32             `json:"id"`
		Status           string            `json:"status"`
		DuplicateBlocked bool              `json:"duplicate_blocked"`
		Similar          []SimilarTaskItem `json:"similar"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"issue":      issueRef,
			"text":       "beta description of /api/auth handler shape",
			"auto_ready": false,
		}, &resp))

	if resp.DuplicateBlocked {
		t.Fatalf("auto_ready=false must not duplicate-block at cos=0.5; got similar=%+v", resp.Similar)
	}
	if resp.ID == 0 {
		t.Fatalf("expected task id, got 0 (resp=%+v)", resp)
	}
	if resp.Status != "wip" {
		t.Fatalf("spec 74: response status should be \"wip\" without --auto-ready; got %q", resp.Status)
	}
	if len(resp.Similar) == 0 {
		t.Fatalf("spec 74: similar tasks should be surfaced when present; got empty list")
	}
	foundSeed := false
	for _, s := range resp.Similar {
		if s.ID == seedRef {
			foundSeed = true
			if s.Score <= 0 {
				t.Errorf("similar[%s].score must be > 0; got %.3f", seedRef, s.Score)
			}
			if s.Score >= 0.90 {
				t.Errorf("similar[%s].score should stay below the 0.90 block threshold; got %.3f", seedRef, s.Score)
			}
		}
	}
	if !foundSeed {
		t.Errorf("similar list does not include seeded task %s; got %+v", seedRef, resp.Similar)
	}

	var detail TaskInfo
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/task?slug=%s&id=TK-%d", slug, resp.ID), nil, &detail))
	if detail.Status != "wip" {
		t.Fatalf("spec 74: persisted status should still be \"wip\" before explicit promotion; got %q", detail.Status)
	}

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/task/ready",
		map[string]any{"id": resp.ID}, nil))

	var detailAfter TaskInfo
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/task?slug=%s&id=TK-%d", slug, resp.ID), nil, &detailAfter))
	if detailAfter.Status != "ready" {
		t.Fatalf("spec 74: status must flip to \"ready\" after /api/dx/todo/task/ready; got %q", detailAfter.Status)
	}
}

// SimilarTaskItem mirrors the server-side type for response decoding.
type SimilarTaskItem struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Text   string  `json:"text"`
	Status string  `json:"status"`
	Reason string  `json:"reason"`
	Issue  string  `json:"issue"`
	Score  float64 `json:"score"`
}
