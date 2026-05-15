package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestApi_IssueForceCloseMarker covers IS-1088 acceptance: --force closes
// with a non-categorical reason (heuristic-false-positive, emergency,
// rollback, other) emit a [closed:force:<reason>] worklog marker and
// persist force_close_reason on the row, distinguishing them from clean
// closes and categorical (duplicate/wontfix/superseded/link) closes which
// keep their existing [closed:<reason>] markers.
func TestApi_IssueForceCloseMarker(t *testing.T) {
	slug := "e2e-force-close-marker"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Force Close Marker"}, nil))

	addIssue := func(title, kind string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      title,
				"context":    "force-marker test",
				"issue_type": kind,
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}

	type issueShape struct {
		Issue struct {
			ID               int32  `json:"id"`
			Status           string `json:"status"`
			CloseReason      string `json:"close_reason"`
			ForceCloseReason string `json:"force_close_reason"`
		} `json:"issue"`
		Work []struct {
			Note string `json:"note"`
		} `json:"work"`
	}

	getIssue := func(id int32) issueShape {
		t.Helper()
		var out issueShape
		mustOK(t, apiDo(t, http.MethodGet,
			"/api/dx/todo/issue/show?slug="+slug+"&id="+fmt.Sprintf("IS-%d", id), nil, &out))
		return out
	}

	// firstClosedNote returns the first work-log entry whose note begins with
	// "[closed" — i.e. the close marker emitted by the close handler. Skipping
	// other bracketed entries (e.g. cascade-skip warnings) lets the assertion
	// stay narrow even if more bracketed notes are appended later.
	firstClosedNote := func(s issueShape) string {
		t.Helper()
		for _, w := range s.Work {
			if strings.HasPrefix(w.Note, "[closed") {
				return w.Note
			}
		}
		t.Fatalf("no [closed...] worklog entry found in %+v", s.Work)
		return ""
	}

	// Case 1: tracker close --force --reason=heuristic-false-positive emits
	// [closed:force:heuristic-false-positive] and populates force_close_reason.
	// Tracker is used to bypass the worklog/resolution gates that are not
	// part of this contract.
	t.Run("force_heuristic_false_positive", func(t *testing.T) {
		id := addIssue("force heuristic", "tracker")
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{
				"slug":   slug,
				"id":     id,
				"reason": "heuristic-false-positive",
				"force":  true,
				"notes":  "decomp gate fired on descriptive prose",
			}, nil))
		got := getIssue(id)
		if got.Issue.Status != "closed" {
			t.Fatalf("status=%q, want closed", got.Issue.Status)
		}
		if got.Issue.ForceCloseReason != "heuristic-false-positive" {
			t.Errorf("force_close_reason=%q, want %q", got.Issue.ForceCloseReason, "heuristic-false-positive")
		}
		note := firstClosedNote(got)
		want := "[closed:force:heuristic-false-positive] decomp gate fired on descriptive prose"
		if note != want {
			t.Errorf("worklog note mismatch:\n  got:  %q\n  want: %q", note, want)
		}
	})

	// Case 2: --force --reason=emergency works the same way.
	t.Run("force_emergency", func(t *testing.T) {
		id := addIssue("force emergency", "tracker")
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{
				"slug":   slug,
				"id":     id,
				"reason": "emergency",
				"force":  true,
				"notes":  "prod outage rollback",
			}, nil))
		got := getIssue(id)
		if got.Issue.ForceCloseReason != "emergency" {
			t.Errorf("force_close_reason=%q, want emergency", got.Issue.ForceCloseReason)
		}
		note := firstClosedNote(got)
		want := "[closed:force:emergency] prod outage rollback"
		if note != want {
			t.Errorf("worklog note mismatch:\n  got:  %q\n  want: %q", note, want)
		}
	})

	// Case 3: --force --reason=duplicate (categorical) keeps the existing
	// [closed:duplicate] marker and DOES NOT populate force_close_reason —
	// categorical reasons are not force-bypasses.
	t.Run("categorical_duplicate_unchanged", func(t *testing.T) {
		target := addIssue("primary", "tracker")
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{"slug": slug, "id": target, "reason": "done"}, nil))

		dup := addIssue("dup", "tracker")
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{
				"slug":         slug,
				"id":           dup,
				"reason":       "duplicate",
				"duplicate_of": fmt.Sprintf("IS-%d", target),
				"force":        true,
			}, nil))
		got := getIssue(dup)
		if got.Issue.ForceCloseReason != "" {
			t.Errorf("categorical close should not set force_close_reason; got %q", got.Issue.ForceCloseReason)
		}
		note := firstClosedNote(got)
		// Categorical close marker stays as [closed:<reason>], not
		// [closed:force:<reason>]. The duplicate-of suffix is appended by the
		// close handler.
		wantPrefix := "[closed:duplicate]"
		if !strings.HasPrefix(note, wantPrefix) {
			t.Errorf("worklog note %q does not start with %q", note, wantPrefix)
		}
		if strings.Contains(note, ":force") {
			t.Errorf("worklog note %q must not contain ':force' for categorical close", note)
		}
	})

	// Case 4: --force --reason=wontfix (categorical) keeps [closed:wontfix].
	t.Run("categorical_wontfix_unchanged", func(t *testing.T) {
		id := addIssue("wontfix it", "tracker")
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{
				"slug":   slug,
				"id":     id,
				"reason": "wontfix",
				"force":  true,
			}, nil))
		got := getIssue(id)
		if got.Issue.ForceCloseReason != "" {
			t.Errorf("wontfix should not set force_close_reason; got %q", got.Issue.ForceCloseReason)
		}
		note := firstClosedNote(got)
		if !strings.HasPrefix(note, "[closed:wontfix]") {
			t.Errorf("worklog note %q does not start with [closed:wontfix]", note)
		}
	})

	// Case 5: clean close --reason=done keeps [closed:done] and does not set
	// force_close_reason.
	t.Run("clean_done_unchanged", func(t *testing.T) {
		id := addIssue("clean done", "tracker")
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{"slug": slug, "id": id, "reason": "done"}, nil))
		got := getIssue(id)
		if got.Issue.ForceCloseReason != "" {
			t.Errorf("clean close should not set force_close_reason; got %q", got.Issue.ForceCloseReason)
		}
		note := firstClosedNote(got)
		if !strings.HasPrefix(note, "[closed:done]") {
			t.Errorf("worklog note %q does not start with [closed:done]", note)
		}
	})
}
