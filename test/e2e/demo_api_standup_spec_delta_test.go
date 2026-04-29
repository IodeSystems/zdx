package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestDemoAPI_StandupSpecDelta covers TK-1335 / IS-585 gap 4: the owner standup
// must surface spec coverage delta (added/covered/deferred in 30d) so verification
// gaps are visible. When deferrals exceed coverings, a concern fires.
//
// Strategy:
//   - Seed a feature with 5 specs.
//   - Link 2 specs to issues that get closed → covered=2, added=2 (link timestamp
//     proxies "added" since zdx_specs has no created_at).
//   - Defer 3 different specs against an open issue → deferred=3.
//   - POST /api/dx/journal/generate (owner) and assert the assessment includes
//     "Spec coverage (30d): +2 added · 2 covered · 3 deferred" and the concerns
//     section flags "verification gap widening."
func TestDemoAPI_StandupSpecDelta(t *testing.T) {
	rec := newApiRecorder(t, "standup-spec-delta")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_standup_spec_delta_test.go", Note: "standup-spec-delta demo source"})
	rec.AddCoderef(coderef{FilePath: "queries/journal.sql", Note: "StandupSpecDelta query"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_dx.go", Note: "owner standup wires StandupSpecDelta into assessment + concerns"})
	t.Cleanup(rec.Save)

	const slug = "demo-spec-delta"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Standup Spec Delta",
	}, nil))

	mustOK(t, rec.Do(http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "platform", "description": "core platform",
	}, nil))

	for _, desc := range []string{"spec-cov-1", "spec-cov-2", "spec-def-1", "spec-def-2", "spec-def-3"} {
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/specs/update", map[string]any{
			"slug": slug, "feature": "platform", "field": "must", "value": desc,
		}, nil))
	}

	var uncovered struct {
		Specs []struct {
			ID          int32  `json:"id"`
			Description string `json:"description"`
		} `json:"specs"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/specs/uncovered?slug="+slug, nil, &uncovered))
	specByDesc := map[string]int32{}
	for _, s := range uncovered.Specs {
		specByDesc[s.Description] = s.ID
	}
	for _, want := range []string{"spec-cov-1", "spec-cov-2", "spec-def-1", "spec-def-2", "spec-def-3"} {
		if _, ok := specByDesc[want]; !ok {
			t.Fatalf("uncovered specs missing %q (got %d specs)", want, len(uncovered.Specs))
		}
	}

	for _, specDesc := range []string{"spec-cov-1", "spec-cov-2"} {
		var iss struct {
			ID int32 `json:"id"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
			"slug": slug, "title": "covered work " + specDesc, "context": "covers spec",
			"auto_ready": true,
		}, &iss))
		issueID := fmt.Sprintf("IS-%d", iss.ID)
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/specs/link-issue", map[string]any{
			"spec_id": specByDesc[specDesc], "issue_id": issueID,
		}, nil))
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/close", map[string]any{
			"slug": slug, "id": iss.ID, "reason": "done",
		}, nil))
	}

	var openIss struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug": slug, "title": "blocking deferrals", "context": "open issue receives spec deferrals",
		"auto_ready": true,
	}, &openIss))
	openIssueID := fmt.Sprintf("IS-%d", openIss.ID)

	for _, desc := range []string{"spec-def-1", "spec-def-2", "spec-def-3"} {
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/specs/deferrals/add", map[string]any{
			"slug": slug, "spec_id": specByDesc[desc], "issue_id": openIssueID,
			"note": "deferred for test",
		}, nil))
	}

	var gen struct {
		Entry struct {
			Assessment string `json:"assessment"`
			Concerns   string `json:"concerns"`
		} `json:"entry"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/journal/generate", map[string]any{
		"slug": slug, "role": "owner",
	}, &gen))

	wantAssess := "**Spec coverage (30d):** +2 added · 2 covered · 3 deferred"
	if !strings.Contains(gen.Entry.Assessment, wantAssess) {
		t.Errorf("assessment missing spec delta line\nwant substring: %q\ngot:\n%s", wantAssess, gen.Entry.Assessment)
	}
	wantConcern := "verification gap widening"
	if !strings.Contains(gen.Entry.Concerns, wantConcern) {
		t.Errorf("concerns missing verification gap message\nwant substring: %q\ngot:\n%s", wantConcern, gen.Entry.Concerns)
	}
}
