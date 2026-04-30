package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestReclaimExpiredTaskExemptsPausedAgent covers TK-1363 / IS-602: a task
// claimed by an agent whose status is 'paused' must NOT be reclaimed even
// when its lease has expired. A paused agent intentionally stops renewing
// its lease and the resume handler is expected to bring it back; reclaiming
// out from under it would discard the worktree context. After the same agent
// transitions back to 'active', the next reclaim sweep DOES pick the task up,
// proving the gate is purely status-driven and the rest of the recovery flow
// is unaffected.
func TestReclaimExpiredTaskExemptsPausedAgent(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to backdate lease")
	}

	d := NewApiDriver(t, "task-reclaim-paused", "Task Reclaim Paused Agent")
	sc := Given(d).
		TriagedIssue("Reclaim paused exemption", "TK-1363 paused agent must hold lease", 2).
		Task(0, "Task held by paused agent").
		Build()
	taskInt := sc.Tasks[0]
	taskID := fmt.Sprintf("TK-%d", taskInt)
	issueID := fmt.Sprintf("IS-%d", sc.Issues[0])

	agentID := registerAgent(t, d.Slug, "agent-paused", "session-paused", "")

	claim := claimTaskWithLease(t, d.Slug, agentID, issueID, 30)
	if claim.ID != taskID {
		t.Fatalf("claimed id=%q, want %q", claim.ID, taskID)
	}

	expireTaskLease(t, taskInt)

	// Re-register the same agent with status='paused'. The register handler
	// upserts on conflict so this only flips the status field.
	registerAgentWithStatus(t, d.Slug, agentID, "session-paused", "", "paused")

	var reclaimResp struct {
		Reclaimed int `json:"reclaimed"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp))
	if reclaimResp.Reclaimed != 0 {
		t.Fatalf("paused-agent task was reclaimed: got reclaimed=%d, want 0", reclaimResp.Reclaimed)
	}
	if got := d.GetTask(taskInt).Status; got != "active" {
		t.Fatalf("task status after paused-reclaim=%q, want %q", got, "active")
	}

	// Flip the agent back to 'active' (simulating resume); reclaim should now
	// succeed because the lease is still expired.
	registerAgentWithStatus(t, d.Slug, agentID, "session-paused", "", "active")

	mustOK(t, apiDo(t, http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp))
	if reclaimResp.Reclaimed < 1 {
		t.Fatalf("active-agent reclaim returned reclaimed=%d, want >= 1", reclaimResp.Reclaimed)
	}
	if got := d.GetTask(taskInt).Status; got != "ready" {
		t.Fatalf("task status after active-reclaim=%q, want %q", got, "ready")
	}
}

func registerAgentWithStatus(t *testing.T, slug, id, sessionID, taskGroup, status string) {
	t.Helper()
	var agent struct {
		ID string `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/agents/register",
		map[string]any{
			"slug":            slug,
			"id":              id,
			"session_id":      sessionID,
			"worktree_path":   "",
			"worktree_branch": "",
			"pid":             0,
			"status":          status,
			"task_group":      taskGroup,
			"compose_project": "",
			"server_port":     0,
			"database_url":    "",
			"valkey_url":      "",
		}, &agent))
}
