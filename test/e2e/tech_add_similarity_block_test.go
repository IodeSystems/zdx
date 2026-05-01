package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// IS-492: dx todo tech add must refuse to file a new task when the top
// embedding-similarity match is >= 90% AND the matched task is open
// (wip/ready/active). Closed similar tasks are informational and do not
// block; --force bypasses.

type techAddResp struct {
	ID               int32 `json:"id"`
	DuplicateBlocked bool  `json:"duplicate_blocked"`
	Similar          []struct {
		ID     string  `json:"id"`
		Title  string  `json:"title"`
		Status string  `json:"status"`
		Score  float64 `json:"score"`
	} `json:"similar"`
}

// startConstantEmbedMock starts a mock OpenAI-compatible LLM server that
// returns an identical 8-dim embedding for every input, so any two tasks
// score 1.0 (above the 0.90 block threshold).
func startConstantEmbedMock(t *testing.T) string {
	t.Helper()
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/embeddings" {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"embedding": []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(mock.Close)
	return mock.URL
}

func registerEmbedMock(t *testing.T, name, url string) {
	t.Helper()
	var cfg struct {
		ID int64 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            name,
		"type":            "openai",
		"agent_type":      "openai",
		"url":             url,
		"embedding_model": "mock-embed",
		"model_low":       "mock-chat",
		"model_medium":    "mock-chat",
		"model_high":      "mock-chat",
		"priority":        0,
	}, &cfg))
	t.Cleanup(func() {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", cfg.ID), nil, nil)
	})
}

// waitForTaskIndexed polls /api/dx/tasks/similar until at least one match
// is returned for queryText. UpsertTask runs in a goroutine on task create,
// so back-to-back creates can race the index.
func waitForTaskIndexed(t *testing.T, slug, queryText string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var out struct {
			Tasks []struct {
				ID    string  `json:"id"`
				Score float64 `json:"score"`
			} `json:"tasks"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/tasks/similar",
			map[string]any{"slug": slug, "text": queryText, "n": 5}, &out))
		if len(out.Tasks) > 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("task index not populated within timeout for slug %s", slug)
}

func TestTechAddBlocksOpenHighSimilarity(t *testing.T) {
	url := startConstantEmbedMock(t)
	registerEmbedMock(t, "is492-block-llm", url)

	slug := "e2e-tech-add-sim-block"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Similarity Block"}, nil)

	var first techAddResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "implement embedding-based duplicate detection alpha",
			"auto_ready": true,
		}, &first))
	if first.ID == 0 {
		t.Fatalf("first add: expected task id, got 0")
	}

	waitForTaskIndexed(t, slug, "implement embedding-based duplicate detection alpha")

	var second techAddResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug": slug,
			"text": "completely different prose so stem and exact-text checks do not fire",
		}, &second))
	if !second.DuplicateBlocked {
		t.Fatalf("second add: expected duplicate_blocked=true, got false (id=%d)", second.ID)
	}
	if second.ID != 0 {
		t.Errorf("second add: expected no task created, got id=%d", second.ID)
	}
	if len(second.Similar) == 0 {
		t.Fatalf("second add: expected at least one similar task in response")
	}
	if second.Similar[0].Score < 0.90 {
		t.Errorf("similar[0] score should be >= 0.90; got %.2f", second.Similar[0].Score)
	}
	wantID := fmt.Sprintf("TK-%d", first.ID)
	if second.Similar[0].ID != wantID {
		t.Errorf("similar[0].ID = %q, want %q", second.Similar[0].ID, wantID)
	}
}

func TestTechAddSimilarityForceBypasses(t *testing.T) {
	url := startConstantEmbedMock(t)
	registerEmbedMock(t, "is492-force-llm", url)

	slug := "e2e-tech-add-sim-force"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Similarity Force"}, nil)

	var first techAddResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "first task body",
			"auto_ready": true,
		}, &first))
	waitForTaskIndexed(t, slug, "first task body")

	var second techAddResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "second task body different prose",
			"force":      true,
			"auto_ready": true,
		}, &second))
	if second.DuplicateBlocked {
		t.Fatalf("--force must bypass similarity block; got duplicate_blocked=true")
	}
	if second.ID == 0 {
		t.Fatalf("--force add: expected task id, got 0")
	}
}

func TestTechAddSimilarityIgnoresClosed(t *testing.T) {
	url := startConstantEmbedMock(t)
	registerEmbedMock(t, "is492-closed-llm", url)

	slug := "e2e-tech-add-sim-closed"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Similarity Closed"}, nil)

	var first techAddResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "original task body",
			"auto_ready": true,
		}, &first))
	waitForTaskIndexed(t, slug, "original task body")

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/done",
		map[string]any{"id": first.ID, "test_plan": "covered"}, nil))

	var second techAddResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug": slug,
			"text": "fresh task after close, different prose to skip exact-text check",
		}, &second))
	if second.DuplicateBlocked {
		t.Fatalf("closed similar task must not block; got duplicate_blocked=true (similar=%+v)", second.Similar)
	}
	if second.ID == 0 {
		t.Fatalf("after-close add: expected task id, got 0")
	}

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/done",
		map[string]any{"id": second.ID, "test_plan": "covered"}, nil))
}
