package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestApi_IssueCloseRecordsCompletedSha covers IS-1062 acceptance: the close
// endpoint accepts and persists completed_in_sha + closed_dirty, and `issue
// show` surfaces them.
//
// CLI-level clean-tree gate is unit-tested in
// internal/cli/project/issue_test.go (TestIsWorkingTreeDirty); the gate runs
// before this endpoint is hit, so this test focuses on the server contract.
func TestApi_IssueCloseRecordsCompletedSha(t *testing.T) {
	slug := "e2e-close-audit"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Close Audit"}, nil))

	addIssue := func(title string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      title,
				"context":    "audit-recording test",
				"issue_type": "impl",
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}

	// Satisfy unrelated close gates (resolution + substantive work-log)
	// so the audit field write is what we're isolating.
	prep := func(id int32) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
			map[string]any{"slug": slug, "id": id, "shas": []string{"deadbeef"}, "source": "manual"}, nil))
		mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work",
			map[string]any{"issue_id": id, "by_role": "test", "note": "completed the work"}, nil))
	}

	type issueShape struct {
		Issue struct {
			ID             int32  `json:"id"`
			Status         string `json:"status"`
			CompletedInSha string `json:"completed_in_sha"`
			ClosedDirty    bool   `json:"closed_dirty"`
		} `json:"issue"`
	}

	getIssue := func(id int32) issueShape {
		t.Helper()
		var out issueShape
		mustOK(t, apiDo(t, http.MethodGet,
			"/api/dx/todo/issue/show?slug="+slug+"&id="+fmt.Sprintf("IS-%d", id), nil, &out))
		return out
	}

	// Case 1: clean close — sha recorded, closed_dirty=false (omitempty on
	// JSON => the unmarshal decoder leaves it false).
	id1 := addIssue("clean close")
	prep(id1)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{
			"slug":             slug,
			"id":               id1,
			"reason":           "done",
			"completed_in_sha": "abc123def456",
			"closed_dirty":     false,
		}, nil))
	got := getIssue(id1)
	if got.Issue.Status != "closed" {
		t.Errorf("clean close: status=%q, want closed", got.Issue.Status)
	}
	if got.Issue.CompletedInSha != "abc123def456" {
		t.Errorf("clean close: completed_in_sha=%q, want abc123def456", got.Issue.CompletedInSha)
	}
	if got.Issue.ClosedDirty {
		t.Errorf("clean close: closed_dirty=true, want false")
	}

	// Case 2: dirty force-close — sha recorded, closed_dirty=true persists.
	id2 := addIssue("dirty force close")
	prep(id2)
	tru := true
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{
			"slug":             slug,
			"id":               id2,
			"reason":           "done",
			"force":            true,
			"completed_in_sha": "feedface00",
			"closed_dirty":     tru,
		}, nil))
	got = getIssue(id2)
	if got.Issue.CompletedInSha != "feedface00" {
		t.Errorf("dirty close: completed_in_sha=%q, want feedface00", got.Issue.CompletedInSha)
	}
	if !got.Issue.ClosedDirty {
		t.Errorf("dirty close: closed_dirty=false, want true")
	}

	// Case 3: --closed-dirty filter on list-issues returns only id2.
	type listShape struct {
		Issues []struct {
			ID          int32 `json:"id"`
			ClosedDirty bool  `json:"closed_dirty"`
		} `json:"issues"`
	}
	var ls listShape
	mustOK(t, apiDo(t, http.MethodGet,
		"/api/dx/todo/issue/list?slug="+slug+"&closed_dirty=true", nil, &ls))
	if len(ls.Issues) != 1 {
		t.Fatalf("list closed_dirty: got %d issues, want 1", len(ls.Issues))
	}
	if ls.Issues[0].ID != id2 {
		t.Errorf("list closed_dirty: got id=%d, want id=%d", ls.Issues[0].ID, id2)
	}
	if !ls.Issues[0].ClosedDirty {
		t.Errorf("list closed_dirty: row carries closed_dirty=false")
	}
}
