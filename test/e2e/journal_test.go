package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func TestJournalAddAndList(t *testing.T) {
	slug := "e2e-issues"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Issues"}, nil)

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]string{"slug": slug, "title": "journal test issue", "context": "test"},
		&issue))

	mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work", map[string]any{
		"issue_id":   issue.ID,
		"entry_type": "note",
		"by_role":    "tech",
		"note":       "First entry",
	}, nil))

	mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work", map[string]any{
		"issue_id":   issue.ID,
		"entry_type": "note",
		"by_role":    "owner",
		"note":       "Second entry",
	}, nil))

	var resp struct {
		Entries []struct {
			Agent string `json:"agent"`
			Note  string `json:"note"`
		} `json:"entries"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/dx/worklog?slug=%s", slug), nil, &resp))
	if len(resp.Entries) < 2 {
		t.Fatalf("want >= 2 worklog entries, got %d", len(resp.Entries))
	}
}
