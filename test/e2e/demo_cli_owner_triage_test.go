package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoCLI_OwnerTriageRevisions is the demo for spec 72: given an untriaged
// issue, when owner triage is run with --priority and optional
// --title/--context/--type, then the issue is classified and a revision log
// entry is recorded for each changed field.
//
// Drives the full flag surface via CLI exec; uses the API driver to verify
// classification and per-field revision entries.
func TestDemoCLI_OwnerTriageRevisions(t *testing.T) {
	const name = "owner-triage-revisions"
	rec := newRecorder(t, name, "bin/dx")
	t.Cleanup(rec.Save)
	slug := "demo-" + name

	// 1. Create an untriaged issue with a title and context.
	rec.Run("issue", "add",
		"--title=Auth middleware is slow on cold start",
		"--context=Noticed 800ms p99 spike on first request after deploy",
		"--auto-ready",
	)
	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Fatal("could not extract issue ID from output")
	}

	// 2. Triage: set priority, refine title, context, and type in a single call.
	rec.Run("todo", "owner", "triage", issueID,
		"--priority=2",
		"--title=Reduce cold-start latency in auth middleware",
		"--context=p99 spikes to 800ms on first request; likely connection pool warm-up",
		"--type=impl",
	)
	triageStep := rec.steps[len(rec.steps)-1]
	if triageStep.ExitCode != 0 {
		t.Fatalf("triage exited %d:\n%s", triageStep.ExitCode, triageStep.Stderr)
	}

	// 3. Verify classification: issue now has a priority set.
	type issueListResp struct {
		Issues []struct {
			ID       int32  `json:"id"`
			Priority string `json:"priority"`
		} `json:"issues"`
	}
	var listResp issueListResp
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/list?slug=%s", slug), nil, &listResp))
	var found bool
	for _, iss := range listResp.Issues {
		if fmt.Sprintf("IS-%d", iss.ID) == issueID {
			found = true
			if iss.Priority != "2" {
				t.Errorf("expected priority=2 after triage, got %q", iss.Priority)
			}
		}
	}
	if !found {
		t.Errorf("issue %s not found in list", issueID)
	}

	// 4. Verify revision log: one entry per changed field.
	type revEntry struct {
		Field  string `json:"field"`
		OldVal string `json:"old_val"`
		NewVal string `json:"new_val"`
	}
	var revResp struct {
		Revisions []revEntry `json:"revisions"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/revisions?slug=%s&target_type=issue&target_id=%s&limit=200",
			slug, issueID), nil, &revResp))

	byField := map[string]revEntry{}
	for _, r := range revResp.Revisions {
		byField[r.Field] = r
	}
	if len(byField) < 4 {
		t.Errorf("expected at least 4 revision entries (priority/title/context/issue_type), got %d: %+v",
			len(byField), revResp.Revisions)
	}
	for _, field := range []string{"priority", "title", "context", "issue_type"} {
		if _, ok := byField[field]; !ok {
			t.Errorf("missing revision entry for field %q", field)
		}
	}
}
