package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestApi_IssueReopenFlipsStatus covers IS-1063: the reopen-issue handler
// previously called ReopenIssue with ProjectID=0, which the WHERE clause
// never matched, so the UPDATE was a silent no-op while the handler returned
// OK. This test fails on the old behavior — `Status` stays "closed" and
// `ReopenCount` stays 0 after the reopen call.
func TestApi_IssueReopenFlipsStatus(t *testing.T) {
	slug := "e2e-reopen"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Reopen"}, nil))

	var added struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{
			"slug":       slug,
			"title":      "reopen round-trip",
			"context":    "IS-1063 regression",
			"issue_type": "impl",
			"auto_ready": true,
		}, &added))

	// Satisfy close gates so we can close without --force.
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
		map[string]any{"slug": slug, "id": added.ID, "shas": []string{"deadbeef"}, "source": "manual"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work",
		map[string]any{"issue_id": added.ID, "by_role": "test", "note": "completed the work"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{"slug": slug, "id": added.ID, "reason": "done"}, nil))

	type issueShape struct {
		Issue struct {
			ID          int32  `json:"id"`
			Status      string `json:"status"`
			ReopenCount int32  `json:"reopen_count,omitempty"`
		} `json:"issue"`
	}
	getIssue := func() issueShape {
		t.Helper()
		var out issueShape
		mustOK(t, apiDo(t, http.MethodGet,
			fmt.Sprintf("/api/dx/todo/issue/show?slug=%s&id=IS-%d", slug, added.ID), nil, &out))
		return out
	}

	if got := getIssue(); got.Issue.Status != "closed" {
		t.Fatalf("after close: status=%q, want closed", got.Issue.Status)
	}

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/reopen",
		map[string]any{"id": added.ID}, nil))

	got := getIssue()
	if got.Issue.Status != "open" {
		t.Errorf("after reopen: status=%q, want open", got.Issue.Status)
	}
	if got.Issue.ReopenCount != 1 {
		t.Errorf("after reopen: reopen_count=%d, want 1", got.Issue.ReopenCount)
	}
}
