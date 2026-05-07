package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestApi_TaskAdopt covers IS-558 / TK-1323: POST /api/dx/tasks/adopt links an
// orphan task to a parent issue so the orphan-<id> nudge resolves.
func TestApi_TaskAdopt(t *testing.T) {
	requiresAPI(t)

	const slug = "e2e-task-adopt"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Task Adopt"}, nil))

	// Seed an open issue.
	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "host issue", "context": "adopt target", "auto_ready": true},
		&issue))
	issueRef := fmt.Sprintf("IS-%d", issue.ID)

	// Seed an orphan ready task (no issue).
	var task struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{"slug": slug, "text": "an orphan task body", "auto_ready": true},
		&task))
	taskRef := fmt.Sprintf("TK-%d", task.ID)

	// Confirm the task starts orphan: agent queue shows orphan-<id>.
	queue := apiEvaluateQueue(t, slug)
	orphanKey := fmt.Sprintf("orphan-%s", taskRef)
	if !containsSoloKey(queue, orphanKey) {
		t.Fatalf("expected %q in agent queue before adopt; got keys %v", orphanKey, queueKeys(queue))
	}

	// Adopt the orphan task into the open issue.
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/tasks/adopt",
		map[string]any{"slug": slug, "task_id": taskRef, "issue_id": issueRef}, nil))

	// Re-fetch task — issue field is now set.
	var got struct {
		IssueID *int32 `json:"issue_id"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/task?slug=%s&id=%s", slug, taskRef), nil, &got))
	if got.IssueID == nil || *got.IssueID != issue.ID {
		t.Fatalf("after adopt: want issue_id=%d, got %+v", issue.ID, got.IssueID)
	}

	// Agent queue no longer surfaces the orphan nudge.
	queue = apiEvaluateQueue(t, slug)
	if containsSoloKey(queue, orphanKey) {
		t.Fatalf("expected %q to be gone from agent queue after adopt; got keys %v", orphanKey, queueKeys(queue))
	}

	// Re-adopting the now-linked task is a 409.
	resp := apiDo(t, http.MethodPost, "/api/dx/tasks/adopt",
		map[string]any{"slug": slug, "task_id": taskRef, "issue_id": issueRef}, nil)
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("re-adopt: want 409, got %d", resp.StatusCode)
	}
}

func apiEvaluateQueue(t *testing.T, slug string) []AgentQueueItem {
	t.Helper()
	var resp struct {
		Added     []AgentQueueItem `json:"added"`
		Unchanged []AgentQueueItem `json:"unchanged"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/agent/evaluate",
		map[string]any{"slug": slug, "issue": ""}, &resp))
	return append(append([]AgentQueueItem{}, resp.Added...), resp.Unchanged...)
}

func containsSoloKey(items []AgentQueueItem, key string) bool {
	for _, it := range items {
		if it.Key == key {
			return true
		}
	}
	return false
}

func queueKeys(items []AgentQueueItem) []string {
	keys := make([]string, len(items))
	for i, it := range items {
		keys[i] = it.Key
	}
	return keys
}
