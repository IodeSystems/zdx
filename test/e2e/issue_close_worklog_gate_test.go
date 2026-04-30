package e2e

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestIssueCloseWorklogGate covers IS-629 (TK-1394): close-with-reason=done is
// blocked when zdx_issue_work has no substantive (non-bracketed) entry.
// The gate is bypassed for reason in {duplicate, link, wontfix} and for
// issue_type in {tracker, ops}.
func TestIssueCloseWorklogGate(t *testing.T) {
	slug := "e2e-worklog-gate"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Worklog Gate"}, nil)

	addIssue := func(issueType string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      "gate test " + issueType,
				"context":    "context",
				"issue_type": issueType,
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}

	// Impl issues require a resolution before close (separate gate); add one
	// for every impl scenario so we are isolating the worklog gate.
	addResolution := func(id int32) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
			map[string]any{"slug": slug, "id": id, "shas": []string{"deadbeef"}, "source": "manual"}, nil))
	}

	addSubstantiveWork := func(id int32, note string) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work",
			map[string]any{"issue_id": id, "by_role": "test", "note": note}, nil))
	}

	// Bracket-prefixed entries do NOT count as substantive: the gate must
	// still trip when only [...] entries exist.
	addBracketedWork := func(id int32, entryType, note string) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work",
			map[string]any{
				"issue_id":   id,
				"entry_type": entryType,
				"by_role":    "test",
				"note":       note,
			}, nil))
	}

	closeIssue := func(id int32, body map[string]any) *http.Response {
		t.Helper()
		body["slug"] = slug
		body["id"] = id
		return apiDo(t, http.MethodPost, "/api/dx/todo/issue/close", body, nil)
	}

	readBody := func(resp *http.Response) string {
		t.Helper()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	// 1. Impl issue, no substantive worklog → 422 with actionable message.
	t.Run("impl_no_worklog_blocks", func(t *testing.T) {
		id := addIssue("impl")
		addResolution(id)
		// A bracketed auto-stamp must NOT count toward the gate.
		addBracketedWork(id, "auto", "auto-stamp masquerade")

		resp := closeIssue(id, map[string]any{"reason": "done"})
		if resp.StatusCode != 422 {
			t.Fatalf("want 422, got %d: %s", resp.StatusCode, readBody(resp))
		}
		body := readBody(resp)
		if !strings.Contains(body, "work-log has no substantive entries") {
			t.Errorf("error message missing actionable text: %s", body)
		}
		if !strings.Contains(body, "dx journal add") {
			t.Errorf("error message should reference 'dx journal add': %s", body)
		}
	})

	// 2. Impl issue with a substantive worklog entry closes successfully.
	t.Run("impl_with_worklog_succeeds", func(t *testing.T) {
		id := addIssue("impl")
		addResolution(id)
		addSubstantiveWork(id, "did the work")
		resp := closeIssue(id, map[string]any{"reason": "done"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})

	// 3. Impl issue, --reason=wontfix → succeeds (no worklog required).
	t.Run("reason_wontfix_bypasses", func(t *testing.T) {
		id := addIssue("impl")
		resp := closeIssue(id, map[string]any{"reason": "wontfix"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})

	// 4. Impl issue, --reason=duplicate → succeeds.
	t.Run("reason_duplicate_bypasses", func(t *testing.T) {
		target := addIssue("impl")
		addResolution(target)
		addSubstantiveWork(target, "primary closed")
		mustOK(t, closeIssue(target, map[string]any{"reason": "done"}))

		dup := addIssue("impl")
		resp := closeIssue(dup, map[string]any{
			"reason":       "duplicate",
			"duplicate_of": fmt.Sprintf("IS-%d", target),
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})

	// 5. Impl issue, --reason=link → succeeds.
	t.Run("reason_link_bypasses", func(t *testing.T) {
		target := addIssue("impl")
		addResolution(target)
		addSubstantiveWork(target, "link target")
		mustOK(t, closeIssue(target, map[string]any{"reason": "done"}))

		linked := addIssue("impl")
		resp := closeIssue(linked, map[string]any{
			"reason":  "link",
			"link_of": fmt.Sprintf("IS-%d", target),
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})

	// 6. Tracker issue closes without worklog (issue-type bypass).
	t.Run("tracker_bypasses", func(t *testing.T) {
		id := addIssue("tracker")
		resp := closeIssue(id, map[string]any{"reason": "done"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})

	// 7. Ops issue closes without worklog (issue-type bypass).
	t.Run("ops_bypasses", func(t *testing.T) {
		id := addIssue("ops")
		resp := closeIssue(id, map[string]any{"reason": "done"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})
}
