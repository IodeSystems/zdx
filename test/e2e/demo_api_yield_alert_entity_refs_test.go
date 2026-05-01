package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestDemoAPI_YieldAlertEntityRefs covers TK-1466: yield alert issue bodies must
// include explicit entity IDs so triagers can click through without a separate
// dx spec list / dx issue list step.
//
// Two breach paths are exercised:
//
//  1. spec_verification_gap — owner standup deferred > covered; the auto-filed
//     issue body must include a "Deferred specs: S-N, ..." line listing the spec IDs.
//
//  2. agent_thrashing — tech standup sessions/closed > 5.0; the auto-filed issue
//     body must include a "Top thrashing issues: IS-N (K sessions)" line.
func TestDemoAPI_YieldAlertEntityRefs(t *testing.T) {
	rec := newApiRecorder(t, "yield-alert-entity-refs")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_yield_alert_entity_refs_test.go", Note: "yield alert entity refs demo source"})
	rec.AddCoderef(coderef{FilePath: "queries/journal.sql", Note: "StandupSpecDelta + StandupTopThrashingIssues queries"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/yield_breach.go", Note: "yieldBreach.Body renders EntityRefs"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_dx.go", Note: "spec_verification_gap and agent_thrashing breaches populate EntityRefs"})
	t.Cleanup(rec.Save)

	t.Setenv("STANDUP_AUTO_FILE_ALERTS", "true")

	t.Run("spec_verification_gap_entity_refs", func(t *testing.T) {
		const slug = "yield-alert-spec-gap"
		mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
			"slug": slug,
			"name": "Yield Alert Spec Gap",
		}, nil))
		mustOK(t, rec.Do(http.MethodPost, "/api/feature", map[string]any{
			"slug": slug, "name": "platform", "description": "core platform",
		}, nil))

		// Add 5 specs.
		for _, desc := range []string{"sc1", "sc2", "sd1", "sd2", "sd3"} {
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
		for _, want := range []string{"sc1", "sc2", "sd1", "sd2", "sd3"} {
			if _, ok := specByDesc[want]; !ok {
				t.Fatalf("uncovered specs missing %q (got %d specs)", want, len(uncovered.Specs))
			}
		}

		// Cover 2 specs via closed issues.
		for _, specDesc := range []string{"sc1", "sc2"} {
			var iss struct {
				ID int32 `json:"id"`
			}
			mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
				"slug": slug, "title": "covered work " + specDesc, "context": "covers spec", "auto_ready": true,
			}, &iss))
			issueID := fmt.Sprintf("IS-%d", iss.ID)
			mustOK(t, rec.Do(http.MethodPost, "/api/dx/specs/link-issue", map[string]any{
				"spec_id": specByDesc[specDesc], "issue_id": issueID,
			}, nil))
			mustOK(t, rec.Do(http.MethodPost, "/api/issue-work", map[string]any{
				"issue_id": iss.ID, "by_role": "test", "note": "ready",
			}, nil))
			mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/close", map[string]any{
				"slug": slug, "id": iss.ID, "reason": "done",
			}, nil))
		}

		// Create open issue to receive deferrals.
		var openIss struct {
			ID int32 `json:"id"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
			"slug": slug, "title": "deferral target", "context": "open", "auto_ready": true,
		}, &openIss))
		openIssueID := fmt.Sprintf("IS-%d", openIss.ID)

		// Defer 3 specs — more than covered, triggers the breach.
		for _, desc := range []string{"sd1", "sd2", "sd3"} {
			mustOK(t, rec.Do(http.MethodPost, "/api/dx/specs/deferrals/add", map[string]any{
				"slug": slug, "spec_id": specByDesc[desc], "issue_id": openIssueID, "note": "deferred",
			}, nil))
		}

		mustOK(t, rec.Do(http.MethodPost, "/api/dx/journal/generate", map[string]any{
			"slug": slug, "role": "owner",
		}, nil))

		// Find the auto-filed yield alert issue.
		var issueList struct {
			Issues []struct {
				Title   string `json:"title"`
				Context string `json:"context"`
			} `json:"issues"`
		}
		mustOK(t, rec.Do(http.MethodGet, "/api/dx/todo/issue/list?slug="+slug+"&status=open", nil, &issueList))

		var alertContext string
		for _, iss := range issueList.Issues {
			if strings.HasPrefix(iss.Title, "Yield alert: Spec verification gap") {
				alertContext = iss.Context
				break
			}
		}
		if alertContext == "" {
			t.Fatal("no 'Yield alert: Spec verification gap' issue auto-filed")
		}
		if !strings.Contains(alertContext, "Deferred specs:") {
			t.Errorf("alert body missing 'Deferred specs:' section\ngot:\n%s", alertContext)
		}
		for _, desc := range []string{"sd1", "sd2", "sd3"} {
			id := specByDesc[desc]
			want := fmt.Sprintf("S-%d", id)
			if !strings.Contains(alertContext, want) {
				t.Errorf("alert body missing deferred spec ref %q\ngot:\n%s", want, alertContext)
			}
		}
	})

	t.Run("agent_thrashing_entity_refs", func(t *testing.T) {
		const slug = "yield-alert-thrash"
		mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
			"slug": slug,
			"name": "Yield Alert Thrash",
		}, nil))

		// Tech standup requires git config; use the local repo so EnsureRepo can
		// clone without network access. RepoDir will clone to a temp path under
		// the server's home directory.
		repoRoot, err := findRoot()
		if err != nil {
			t.Skipf("could not find repo root for git config: %v", err)
		}
		mustOK(t, rec.Do(http.MethodPut, "/api/admin/project-git-config", map[string]any{
			"slug": slug, "git_url": "file://" + repoRoot, "git_branch": "dev",
		}, nil))

		// Close 3 issues to meet standupMinClosedForRatios.
		for i := range 3 {
			var iss struct {
				ID int32 `json:"id"`
			}
			mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
				"slug": slug, "title": fmt.Sprintf("close-me-%d", i), "context": "work", "auto_ready": true,
			}, &iss))
			mustOK(t, rec.Do(http.MethodPost, "/api/issue-work", map[string]any{
				"issue_id": iss.ID, "by_role": "test", "note": "done",
			}, nil))
			mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/close", map[string]any{
				"slug": slug, "id": iss.ID, "reason": "done",
			}, nil))
		}

		// Create the thrashing issue (high session count).
		var thrashIss struct {
			ID int32 `json:"id"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
			"slug": slug, "title": "stuck-work", "context": "keeps failing", "auto_ready": true,
		}, &thrashIss))
		thrashIssueID := fmt.Sprintf("IS-%d", thrashIss.ID)

		// Create 16 sessions for the thrashing issue: 16/3 = 5.33 > 5.0 threshold.
		for i := range 16 {
			sessionID := fmt.Sprintf("thrash-session-%s-%d", slug, i)
			mustOK(t, rec.Do(http.MethodPost,
				fmt.Sprintf("/api/dx/agent/sessions/%s/create?slug=%s", sessionID, slug),
				map[string]any{
					"provider": "claude", "alias": "test-agent", "trigger": "manual",
					"title":    fmt.Sprintf("thrash session %d", i),
					"issue_id": thrashIssueID, "agent_id": "", "agent_type": "claude",
				}, nil))
		}

		mustOK(t, rec.Do(http.MethodPost, "/api/dx/journal/generate", map[string]any{
			"slug": slug, "role": "tech",
		}, nil))

		var issueList struct {
			Issues []struct {
				Title   string `json:"title"`
				Context string `json:"context"`
			} `json:"issues"`
		}
		mustOK(t, rec.Do(http.MethodGet, "/api/dx/todo/issue/list?slug="+slug+"&status=open", nil, &issueList))

		var alertContext string
		for _, iss := range issueList.Issues {
			if strings.HasPrefix(iss.Title, "Yield alert: Agent thrashing") {
				alertContext = iss.Context
				break
			}
		}
		if alertContext == "" {
			t.Fatal("no 'Yield alert: Agent thrashing' issue auto-filed")
		}
		if !strings.Contains(alertContext, "Top thrashing issues:") {
			t.Errorf("alert body missing 'Top thrashing issues:' section\ngot:\n%s", alertContext)
		}
		if !strings.Contains(alertContext, thrashIssueID) {
			t.Errorf("alert body missing thrashing issue ref %q\ngot:\n%s", thrashIssueID, alertContext)
		}
	})
}
