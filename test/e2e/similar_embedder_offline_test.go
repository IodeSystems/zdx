package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// IS-808: when the embedder is offline (no LLM client configured), the
// findSimilar* handlers must still surface text matches so the dup-check
// path is not silently dark. Today only findSimilarProposals has the hybrid
// text+vector pattern (similar.go:57); this test pins the contract for the
// other five (issues, features, specs, tasks, questions).
//
// Pre-refactor (TK-1473): all five subtests fail because the handlers only
// call h.Emb.TopN* which returns nil when no LLM is configured.
// Post-refactor: text fallback surfaces seeded items via SearchXxx queries.
func TestFindSimilar_EmbedderOffline_TextFallback(t *testing.T) {
	requiresAPI(t)

	// Defensively clear any LLM config a prior test failed to clean up.
	// findSimilar*'s degraded-path contract is "no LLM client" — registering
	// any LLM here would activate the vector path and mask the regression.
	clearLLMConfigs(t)

	slug := "e2e-similar-offline"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Similar Embedder Offline"}, nil))

	// Distinctive token that won't collide with seed/demo data.
	const token = "widgetX1474"

	// ── seed each entity with text containing the token ───────────────────
	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "investigate " + token + " regression", "context": "details"}, &issue))

	var feat struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/feature",
		map[string]any{"slug": slug, "name": token + "-renderer", "description": "renders the " + token}, &feat))

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/update",
		map[string]any{"slug": slug, "feature": token + "-renderer", "field": "must", "value": "must paint the " + token + " on resize"}, nil))

	var task struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{"slug": slug, "text": "wire up the " + token + " click handler", "issue": fmt.Sprintf("IS-%d", issue.ID), "auto_ready": true}, &task))

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/qa/add",
		map[string]any{"slug": slug, "category": "design", "question": "how should the " + token + " render on mobile?"}, nil))

	// Proposals: create one whose body is a superstring of the queryText the
	// dup-check below will compose. create-proposal builds queryText as
	// `title + " " + body`; SearchProposals does an ILIKE substring match,
	// so the seed body must contain that exact concatenation.
	var prop struct {
		Proposal *struct {
			ID int32 `json:"id"`
		} `json:"proposal,omitempty"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/proposals",
		map[string]any{"slug": slug, "title": "seed", "value": "small", "body": token + " details", "source_type": "conversation"}, &prop))
	if prop.Proposal == nil {
		t.Fatalf("proposal create: expected proposal in response, got nil (similar surfaced unexpectedly?)")
	}

	// ── assert each /similar endpoint surfaces the seeded item ────────────
	t.Run("issues", func(t *testing.T) {
		var resp struct {
			Issues []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"issues"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/issues/similar",
			map[string]any{"slug": slug, "text": token, "n": 5}, &resp))
		if len(resp.Issues) == 0 {
			t.Fatalf("text fallback must surface seeded issue when embedder is offline; got 0 results")
		}
	})

	t.Run("features", func(t *testing.T) {
		var resp struct {
			Features []struct {
				ID   int32  `json:"id"`
				Name string `json:"name"`
			} `json:"features"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/features/similar",
			map[string]any{"slug": slug, "text": token, "n": 5}, &resp))
		if len(resp.Features) == 0 {
			t.Fatalf("text fallback must surface seeded feature when embedder is offline; got 0 results")
		}
	})

	t.Run("specs", func(t *testing.T) {
		var resp struct {
			Specs []struct {
				ID          int32  `json:"id"`
				Description string `json:"description"`
			} `json:"specs"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/similar",
			map[string]any{"slug": slug, "text": token, "n": 5}, &resp))
		if len(resp.Specs) == 0 {
			t.Fatalf("text fallback must surface seeded spec when embedder is offline; got 0 results")
		}
	})

	t.Run("tasks", func(t *testing.T) {
		var resp struct {
			Tasks []struct {
				ID   string `json:"id"`
				Text string `json:"text"`
			} `json:"tasks"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/tasks/similar",
			map[string]any{"slug": slug, "text": token, "n": 5}, &resp))
		if len(resp.Tasks) == 0 {
			t.Fatalf("text fallback must surface seeded task when embedder is offline; got 0 results")
		}
	})

	t.Run("questions", func(t *testing.T) {
		var resp struct {
			Questions []struct {
				ID       int32  `json:"id"`
				Question string `json:"question"`
			} `json:"questions"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/qa/similar",
			map[string]any{"slug": slug, "text": token, "n": 5}, &resp))
		if len(resp.Questions) == 0 {
			t.Fatalf("text fallback must surface seeded question when embedder is offline; got 0 results")
		}
	})

	t.Run("proposals", func(t *testing.T) {
		// Proposals expose the similar path through create-proposal: when an
		// existing open proposal matches, the response carries `similar` and
		// a duplicates_review_token instead of creating the new proposal.
		var resp struct {
			Proposal *struct {
				ID int32 `json:"id"`
			} `json:"proposal,omitempty"`
			Similar []struct {
				ID    int32  `json:"id"`
				Title string `json:"title"`
			} `json:"similar,omitempty"`
			DuplicatesReviewToken *string `json:"duplicates_review_token,omitempty"`
		}
		// queryText composed by handler = title + " " + body = "<token> details".
		// Seed body above contains that exact substring, so SearchProposals matches.
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/proposals",
			map[string]any{"slug": slug, "title": token, "value": "small", "body": "details", "source_type": "conversation"}, &resp))
		if len(resp.Similar) == 0 {
			t.Fatalf("text fallback must surface prior proposal when embedder is offline; got 0 similar (proposal=%v)", resp.Proposal)
		}
	})
}

// clearLLMConfigs removes every llm-config row, leaving the embedder with no
// active client. Pairs with TestFindSimilar_EmbedderOffline_TextFallback.
func clearLLMConfigs(t *testing.T) {
	t.Helper()
	var list struct {
		Configs []struct {
			ID int64 `json:"id"`
		} `json:"configs"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/admin/llm-configs", nil, &list))
	for _, c := range list.Configs {
		apiDo(t, http.MethodDelete, fmt.Sprintf("/api/admin/llm-configs/%d", c.ID), nil, nil)
	}
}
