package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoAPI_ReclaimExpiredTask is the demo for spec 69: given a task claimed
// by an agent whose lease has expired, when POST /api/tasks/reclaim-expired
// runs, then the task is reset to ready with claimed_by cleared and becomes
// available for a new claim.
//
// Lease expiry is simulated by backdating the reservation directly in the DB.
// If the test database is unavailable the test is skipped.
func TestDemoAPI_ReclaimExpiredTask(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; skipping (needs direct DB access to backdate lease)")
	}

	rec := newApiRecorder(t, "reclaim-expired-task")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_reclaim_expired_test.go", Note: "reclaim-expired demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_agents.go", Note: "POST /api/tasks/reclaim-expired handler"})
	rec.AddCoderef(coderef{FilePath: "internal/db/tasks.sql.go", Note: "ReclaimExpiredTasks resets active tasks with expired leases"})
	t.Cleanup(rec.Save)

	// Set up a project with one triaged issue and one ready task.
	var proj struct {
		Slug string `json:"slug"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-reclaim-expired",
		"name":           "Demo Reclaim Expired",
		"classification": "service",
	}, &proj))

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       proj.Slug,
		"title":      "Implement reclaim-expired recovery",
		"context":    "agent lease may expire before work completes",
		"auto_ready": true,
	}, &issue))

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/owner/triage", map[string]any{
		"slug":     proj.Slug,
		"id":       issue.ID,
		"priority": 2,
	}, nil))

	issueRef := fmt.Sprintf("IS-%d", issue.ID)
	var task struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
		"slug":       proj.Slug,
		"text":       "Handle reclaim-expired recovery scenario",
		"issue":      issueRef,
		"auto_ready": true,
		"force":      true,
	}, &task))

	// Agent A claims the task.
	var claimed struct {
		ID string `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/tasks/claim", map[string]any{
		"slug":               proj.Slug,
		"agent_id":           "demo-agent-A",
		"issue":              issueRef,
		"lease_duration_min": 30,
	}, &claimed))
	taskID := claimed.ID

	// Backdate the lease so reclaim-expired sees it as expired.
	taskIntID := task.ID
	expireTaskLease(t, taskIntID)

	// POST /api/tasks/reclaim-expired resets the task to ready.
	var reclaimResp struct {
		Reclaimed int `json:"reclaimed"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp))
	if reclaimResp.Reclaimed < 1 {
		t.Fatalf("reclaim-expired returned reclaimed=%d, want >= 1", reclaimResp.Reclaimed)
	}

	// The task must now be ready (claimed_by cleared).
	var taskDetail struct {
		Status string `json:"status"`
	}
	mustOK(t, rec.Do(http.MethodGet, fmt.Sprintf("/api/task?slug=%s&id=%s", proj.Slug, taskID), nil, &taskDetail))
	if taskDetail.Status != "ready" {
		t.Fatalf("task status=%q after reclaim, want %q", taskDetail.Status, "ready")
	}

	// Agent B can now claim the same task.
	var reclaimed struct {
		ID string `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/tasks/claim", map[string]any{
		"slug":               proj.Slug,
		"agent_id":           "demo-agent-B",
		"issue":              issueRef,
		"lease_duration_min": 30,
	}, &reclaimed))
	if reclaimed.ID != taskID {
		t.Fatalf("agent-B claimed id=%q, want %q", reclaimed.ID, taskID)
	}
}
