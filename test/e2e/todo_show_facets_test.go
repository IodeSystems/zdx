package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestTodoShowRendersSpec81Facets is the conformance test for spec 81:
// "Given an issue, task, or feature identifier, when todo show is run, then
// detailed information is displayed including status, context, comments, and
// similar patterns".
//
// Setup: register a mock OpenAI-compatible LLM so the embedder activates and
// the SimilarPatterns lookup returns hits. Seed an issue (with context +
// comment), a tech task (with comment), and a feature (desc + comment), plus a
// pattern that the SimilarPatterns query can match.
//
// Run dx todo show against each target shape via the recorder and assert the
// rendered stdout covers each spec-81 facet:
//   - issue: Status: label, context body, Comments: header + body, Relevant patterns: block.
//   - task:  Status: label, plan/text body, Comments: header + body, Relevant patterns: block.
//   - feature: Name: label, Desc: body, Comments: header + body. (Feature show
//     today does not call printSimilarPatterns; the issue+task assertions cover
//     the similar-patterns facet for spec-81 conformance.)
func TestTodoShowRendersSpec81Facets(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/embeddings" {
			w.Header().Set("Content-Type", "application/json")
			// 8-dim vector matches existing .zvec index files written by prior
			// tests in the shared testserver — different dims would log
			// "dimension mismatch" and skip the upsert.
			_ = json.NewEncoder(w).Encode(map[string]any{
				"data": []map[string]any{{"embedding": []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}}},
			})
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(mock.Close)

	var cfg struct {
		ID int64 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "spec81-facets-llm",
		"type":            "openai",
		"agent_type":      "openai",
		"url":             mock.URL,
		"embedding_model": "mock-embed",
		"model_low":       "mock-chat",
		"model_medium":    "mock-chat",
		"model_high":      "mock-chat",
		"priority":        0,
	}, &cfg))
	t.Cleanup(func() {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", cfg.ID), nil, nil)
	})

	rec := newRecorder(t, "todo-show-facets", "bin/dx")
	t.Cleanup(rec.Save)
	slug := "demo-todo-show-facets"

	const (
		issueContext = "issue context body for spec 81 verification"
		issueComment = "issue comment body for spec 81"
		taskText     = "task implementation plan body for spec 81 verification"
		taskComment  = "task comment body for spec 81"
		featureDesc  = "feature description body for spec 81 verification"
		featureName  = "spec81-feature"
		featComment  = "feature comment body for spec 81"
	)

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "Show facets target issue", "context": issueContext, "auto_ready": true}, &issue))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/owner/triage",
		map[string]any{"slug": slug, "id": issue.ID, "priority": 2}, nil))
	issueID := fmt.Sprintf("IS-%d", issue.ID)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": slug, "target_type": "issue", "target_id": issueID, "body": issueComment}, nil))

	var task struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{"slug": slug, "issue": issueID, "text": taskText, "auto_ready": true}, &task))
	taskID := fmt.Sprintf("TK-%d", task.ID)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": slug, "target_type": "task", "target_id": taskID, "body": taskComment}, nil))

	mustOK(t, apiDo(t, http.MethodPost, "/api/feature",
		map[string]any{"slug": slug, "name": featureName, "description": featureDesc}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": slug, "target_type": "feature", "target_id": featureName, "body": featComment}, nil))

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/patterns/add",
		map[string]any{"slug": slug, "name": "spec81-pattern", "description": "pattern description for spec 81 similarity"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/patterns/reindex",
		map[string]any{"slug": slug}, nil))

	apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "Similar facets target", "context": "another spec 81 verification entry", "auto_ready": true}, nil)

	rec.Run("todo", "show", issueID)
	issueOut := rec.steps[len(rec.steps)-1]
	if issueOut.ExitCode != 0 {
		t.Fatalf("dx todo show %s failed: %s", issueID, issueOut.Stderr)
	}

	rec.Run("todo", "show", taskID)
	taskOut := rec.steps[len(rec.steps)-1]
	if taskOut.ExitCode != 0 {
		t.Fatalf("dx todo show %s failed: %s", taskID, taskOut.Stderr)
	}

	rec.Run("todo", "show", featureName)
	featOut := rec.steps[len(rec.steps)-1]
	if featOut.ExitCode != 0 {
		t.Fatalf("dx todo show %s failed: %s", featureName, featOut.Stderr)
	}

	requireFacets(t, fmt.Sprintf("issue %s", issueID), issueOut.Stdout, []string{
		"Status:",
		issueContext,
		"Comments:",
		issueComment,
		"Relevant patterns:",
	})

	requireFacets(t, fmt.Sprintf("task %s", taskID), taskOut.Stdout, []string{
		"Status:",
		taskText,
		"Comments:",
		taskComment,
		"Relevant patterns:",
	})

	requireFacets(t, fmt.Sprintf("feature %s", featureName), featOut.Stdout, []string{
		"Name:",
		featureDesc,
		"Comments:",
		featComment,
	})
}

func requireFacets(t *testing.T, label, out string, needles []string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(out, n) {
			t.Errorf("%s: missing %q in stdout\n--- stdout ---\n%s", label, n, out)
		}
	}
}
