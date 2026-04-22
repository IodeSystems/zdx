package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// TestDemoCLI_MockBoundary is the demo for spec 23: demo recordings mock only
// third-party services and preserve real app behaviour.
//
// The test registers a local mock OpenAI-compatible server as the LLM provider,
// runs a discussion workflow via the dx CLI, then asserts the three-way
// mocking-boundary contract:
//
//  1. Third-party (LLM) traffic went to the mock, not the network.
//  2. dx API calls hit the real in-process server (discussion row in DB).
//  3. The demo JSON log contains server-assigned IDs consistent with the DB.
func TestDemoCLI_MockBoundary(t *testing.T) {
	// ── 1. Mock LLM server ────────────────────────────────────────────────────
	var mockHits atomic.Int32
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			mockHits.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			chunk, _ := json.Marshal(map[string]any{
				"choices": []map[string]any{
					{"delta": map[string]string{"content": "mock boundary response"}},
				},
			})
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", chunk)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case "/v1/embeddings":
			// Satisfy background reindex calls silently.
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"data": []map[string]any{
					{"embedding": []float32{0.1, 0.2, 0.3}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer mock.Close()

	// ── 2. Register mock as the LLM config (triggers ReloadEmbedder) ─────────
	var cfg struct {
		ID int64 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/admin/llm-configs", map[string]any{
		"name":            "mock-boundary-llm",
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

	// ── 3. Demo recording ─────────────────────────────────────────────────────
	rec := newRecorder(t, "mock-boundary", "bin/dx")
	t.Cleanup(rec.Save)
	rec.AddCoderef(coderef{
		FilePath: "test/e2e/demo_mock_boundary_test.go",
		Note:     "mock boundary demo source",
	})

	rec.Run("discussion", "start", "--title=Mock boundary test")
	if rec.steps[len(rec.steps)-1].ExitCode != 0 {
		t.Fatalf("discussion start failed:\n%s", rec.steps[len(rec.steps)-1].Stderr)
	}

	// Parse the DS-N id from the start output: "DS-1  [active]  Mock boundary test\n"
	startOut := rec.steps[len(rec.steps)-1].Stdout
	dsID := parseBoundaryDSID(t, startOut)

	rec.Run("discussion", "send", dsID, "--message=What is the mock boundary?")
	rec.Run("discussion", "list")
	rec.Run("discussion", "show", dsID)

	// ── 4. Assert: mock was called (third-party call intercepted) ────────────
	if mockHits.Load() == 0 {
		t.Fatal("mock LLM server was never called — third-party traffic escaped the mock")
	}

	// ── 5. Assert: discussion exists in real server DB ────────────────────────
	slug := "demo-mock-boundary"
	var listResp struct {
		Discussions []struct {
			ID    int32  `json:"id"`
			Title string `json:"title"`
		} `json:"discussions"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/dx/discussions?slug="+slug, nil, &listResp))
	if len(listResp.Discussions) == 0 {
		t.Fatal("no discussions in DB — dx API calls were short-circuited")
	}

	// ── 6. Assert: demo log ID is consistent with the DB ─────────────────────
	dbID := fmt.Sprintf("DS-%d", listResp.Discussions[0].ID)
	if dsID != dbID {
		t.Errorf("demo log ID %q doesn't match DB discussion ID %q", dsID, dbID)
	}

	// ── 7. Assert: mock response text appears in the send step ───────────────
	for _, step := range rec.steps {
		if len(step.Args) > 0 && step.Args[0] == "discussion" && len(step.Args) > 1 && step.Args[1] == "send" {
			if !strings.Contains(step.Stdout, "mock boundary response") {
				t.Errorf("send step stdout doesn't contain mock response text; got: %q", step.Stdout)
			}
			break
		}
	}
}

// parseBoundaryDSID extracts the first token from a discussion start output line.
// Expected format: "DS-N  [status]  title\n"
func parseBoundaryDSID(t *testing.T, out string) string {
	t.Helper()
	fields := strings.Fields(out)
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "DS-") {
		t.Fatalf("cannot parse DS-N from discussion start output: %q", out)
	}
	return fields[0]
}
