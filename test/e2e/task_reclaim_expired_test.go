package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestReclaimExpiredTask covers spec 69: /api/tasks/reclaim-expired resets
// active tasks with an expired lease back to "ready" so they can be re-claimed.
func TestReclaimExpiredTask(t *testing.T) {
	d := NewApiDriver(t, "task-reclaim", "Task Reclaim Expired")
	sc := Given(d).
		TriagedIssue("Reclaim expired test", "spec 69 recovery path", 2).
		Task(0, "Task for reclaim expired").
		Build()
	taskInt := sc.Tasks[0]
	taskID := fmt.Sprintf("TK-%d", taskInt)
	issueID := fmt.Sprintf("IS-%d", sc.Issues[0])

	claim := claimTaskWithLease(t, d.Slug, "agent-A", issueID, 30)
	if claim.ID != taskID {
		t.Fatalf("claimed id=%q, want %q", claim.ID, taskID)
	}

	expireTaskLease(t, taskInt)

	var reclaimResp struct {
		Reclaimed int `json:"reclaimed"`
	}
	resp := apiDo(t, http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp)
	mustOK(t, resp)
	if reclaimResp.Reclaimed < 1 {
		t.Fatalf("reclaim-expired returned reclaimed=%d, want >= 1", reclaimResp.Reclaimed)
	}

	task := d.GetTask(taskInt)
	if task.Status != "ready" {
		t.Fatalf("task status=%q after reclaim, want %q", task.Status, "ready")
	}

	claim2 := claimTaskWithLease(t, d.Slug, "agent-B", issueID, 30)
	if claim2.ID != taskID {
		t.Fatalf("agent-B claimed id=%q, want %q (task should be re-claimable)", claim2.ID, taskID)
	}
}

func expireTaskLease(t *testing.T, taskInt int32) {
	t.Helper()
	resp := apiDo(t, http.MethodPost, "/api/tasks/test/expire-lease",
		map[string]any{"task_id": fmt.Sprintf("TK-%d", taskInt)}, nil)
	if resp.StatusCode >= 400 {
		t.Fatalf("expire-lease: %d", resp.StatusCode)
	}
}
