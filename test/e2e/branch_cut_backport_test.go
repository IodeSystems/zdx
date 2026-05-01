package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestBranchCutAutoBackport covers IS-825 trigger 2: when `dx branch cut`
// creates a new named version branch, the server enumerates open dev issues
// at priority <= 2 (must/should) and auto-creates one ready backport task per
// issue against the new branch. Lower-priority issues are skipped, and the
// idempotency guard prevents duplicate tasks on re-runs.
func TestBranchCutAutoBackport(t *testing.T) {
	const slug = "e2e-branch-cut-backport"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Branch Cut Backport"},
		nil,
	))

	type addIssueResp struct {
		ID int32 `json:"id"`
	}
	addIssue := func(title string, priority int) string {
		var iss addIssueResp
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{"slug": slug, "title": title, "context": title, "auto_ready": true},
			&iss,
		))
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/owner/triage",
			map[string]any{"slug": slug, "id": iss.ID, "priority": priority},
			nil,
		))
		return fmt.Sprintf("IS-%d", iss.ID)
	}

	mustIssue := addIssue("must-tier issue", 1)
	shouldIssue := addIssue("should-tier issue", 2)
	addIssue("nice-to-have issue", 3) // not eligible for backport

	type cutResp struct {
		Name                 string `json:"name"`
		BackportTasksCreated int    `json:"backport_tasks_created"`
	}
	var first cutResp
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/branches", slug),
		map[string]any{"name": "v9.9.x"},
		&first,
	))
	if first.Name != "v9.9.x" {
		t.Fatalf("first cut name: want %q got %q", "v9.9.x", first.Name)
	}
	if first.BackportTasksCreated != 2 {
		t.Errorf("first cut backport_tasks_created: want 2 (must+should), got %d", first.BackportTasksCreated)
	}

	type taskRow struct {
		ID           int32  `json:"id"`
		Title        string `json:"title"`
		Status       string `json:"status"`
		IssueID      *int32 `json:"issue_id,omitempty"`
		TargetBranch string `json:"target_branch"`
	}
	type listTasksResp struct {
		Tasks []taskRow `json:"tasks"`
	}
	listTasks := func(issueRef string) []taskRow {
		var resp listTasksResp
		mustOK(t, apiDo(t, http.MethodGet,
			fmt.Sprintf("/api/dx/todo/issue/tasks?slug=%s&issue_id=%s", slug, issueRef),
			nil, &resp,
		))
		return resp.Tasks
	}

	for _, issueRef := range []string{mustIssue, shouldIssue} {
		tasks := listTasks(issueRef)
		var backport *taskRow
		for i := range tasks {
			if tasks[i].TargetBranch == "v9.9.x" {
				backport = &tasks[i]
				break
			}
		}
		if backport == nil {
			t.Errorf("%s: missing v9.9.x backport task; got %+v", issueRef, tasks)
			continue
		}
		if backport.Status != "ready" {
			t.Errorf("%s backport status: want %q got %q", issueRef, "ready", backport.Status)
		}
		gotIssueRef := ""
		if backport.IssueID != nil {
			gotIssueRef = fmt.Sprintf("IS-%d", *backport.IssueID)
		}
		if gotIssueRef != issueRef {
			t.Errorf("%s backport issue link: want %q got %q", issueRef, issueRef, gotIssueRef)
		}
	}

	// Re-cut the same branch (idempotency guard): no new tasks even if the
	// underlying CreateVersionBranch insert fails on uniqueness, because the
	// auto-fill path skips when an open backport task already exists.
	var second cutResp
	resp := apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/branches", slug),
		map[string]any{"name": "v9.9.x"},
		&second,
	)
	// Either the insert is idempotent (200 + 0 created) or it fails — in either
	// case the database state must not double-count tasks. Tolerate both.
	if resp.StatusCode == http.StatusOK && second.BackportTasksCreated != 0 {
		t.Errorf("re-cut backport_tasks_created: want 0 (idempotent), got %d", second.BackportTasksCreated)
	}
	for _, issueRef := range []string{mustIssue, shouldIssue} {
		count := 0
		for _, task := range listTasks(issueRef) {
			if task.TargetBranch == "v9.9.x" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%s: want exactly 1 backport task to v9.9.x, got %d", issueRef, count)
		}
	}
}
