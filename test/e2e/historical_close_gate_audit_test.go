package e2e

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

// TestHistoricalCloseGateAudit covers IS-632 (TK-1404): the
// /api/dx/doctor/historical-close-gate endpoint enumerates closed issues
// that fail the IS-560 close-gate retroactively. Force-closed and
// tracker/ops issues are excluded from the audit so they don't appear
// even when they have no substantive work-log.
//
// The live close-gates (IS-628..IS-631, IS-595) prevent test-side
// construction of an offending close, so this test verifies endpoint
// shape and the bypass exclusions only. Per-gate counting/sample logic
// is covered in TestRetroactiveAuditRung (internal/doctor/vines_test.go).
func TestHistoricalCloseGateAudit(t *testing.T) {
	slug := "e2e-hist-close-gate"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Historical Close Gate"}, nil)

	addIssue := func(issueType string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      "audit test " + issueType,
				"context":    "ctx",
				"issue_type": issueType,
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}

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

	closeIssue := func(id int32, body map[string]any) *http.Response {
		t.Helper()
		full := map[string]any{"slug": slug, "id": id}
		for k, v := range body {
			full[k] = v
		}
		return apiDo(t, http.MethodPost, "/api/dx/todo/issue/close", full, nil)
	}

	// 1) impl issue properly closed (resolution + substantive work-log) —
	// should NOT appear in audit.
	implOK := addIssue("impl")
	addResolution(implOK)
	addSubstantiveWork(implOK, "real work done")
	mustOK(t, closeIssue(implOK, map[string]any{"reason": "done"}))

	// 2) tracker issue closed with no substantive work-log — must be
	// excluded from the audit (tracker bypass).
	trackerNoWork := addIssue("tracker")
	mustOK(t, closeIssue(trackerNoWork, map[string]any{"reason": "done"}))

	// 3) impl issue force-closed (close_reason set) — must be excluded
	// from the audit (force-close bypass; force-close substance is
	// covered separately by the force_closes_have_work_log rung).
	implForce := addIssue("impl")
	addResolution(implForce)
	mustOK(t, closeIssue(implForce, map[string]any{"reason": "wontfix", "force": true}))

	// Query the audit endpoint.
	type offender struct {
		IssueID string `json:"issue_id"`
		Gate    string `json:"gate"`
		Detail  string `json:"detail"`
	}
	var resp struct {
		Offenders []offender `json:"offenders"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		"/api/dx/doctor/historical-close-gate?slug="+url.QueryEscape(slug),
		nil, &resp))

	excluded := map[string]bool{
		fmt.Sprintf("IS-%d", implOK):        true,
		fmt.Sprintf("IS-%d", trackerNoWork): true,
		fmt.Sprintf("IS-%d", implForce):     true,
	}
	for _, off := range resp.Offenders {
		if excluded[off.IssueID] {
			t.Errorf("issue %s should be excluded from audit (gate=%s detail=%s)",
				off.IssueID, off.Gate, off.Detail)
		}
	}
}
