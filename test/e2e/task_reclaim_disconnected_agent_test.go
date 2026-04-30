package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestReclaimExpiredTaskDisconnectGrace covers TK-1365 / IS-602: a task
// claimed by a 'disconnected' agent must NOT be reclaimed while its
// disconnect_at is within the grace window, giving the daemon time to
// reconnect without losing its worktree context. Once the grace window
// expires the task is reclaim-eligible again.
func TestReclaimExpiredTaskDisconnectGrace(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to set agent state")
	}

	d := NewApiDriver(t, "task-reclaim-disconnect", "Task Reclaim Disconnected Agent")
	sc := Given(d).
		TriagedIssue("Disconnect grace test", "TK-1365 disconnected agent grace window", 2).
		Task(0, "Task held by disconnected agent").
		Build()
	taskInt := sc.Tasks[0]
	taskID := fmt.Sprintf("TK-%d", taskInt)
	issueID := fmt.Sprintf("IS-%d", sc.Issues[0])

	agentID := registerAgent(t, d.Slug, "agent-disconnect-grace", "session-disconnect", "")

	claim := claimTaskWithLease(t, d.Slug, agentID, issueID, 30)
	if claim.ID != taskID {
		t.Fatalf("claimed id=%q, want %q", claim.ID, taskID)
	}

	expireTaskLease(t, taskInt)

	// Set agent to disconnected with disconnect_at=now()-30s (within default 60s grace).
	setAgentDisconnectAt(t, agentID, -30*time.Second)

	var reclaimResp struct {
		Reclaimed int `json:"reclaimed"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp))
	if reclaimResp.Reclaimed != 0 {
		t.Fatalf("disconnected-agent task within grace was reclaimed: got reclaimed=%d, want 0", reclaimResp.Reclaimed)
	}
	if got := d.GetTask(taskInt).Status; got != "active" {
		t.Fatalf("task status after in-grace reclaim=%q, want active", got)
	}

	// Move disconnect_at outside the grace window (90s ago > 60s default).
	setAgentDisconnectAt(t, agentID, -90*time.Second)

	mustOK(t, apiDo(t, http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp))
	if reclaimResp.Reclaimed < 1 {
		t.Fatalf("post-grace reclaim returned reclaimed=%d, want >= 1", reclaimResp.Reclaimed)
	}
	if got := d.GetTask(taskInt).Status; got != "ready" {
		t.Fatalf("task status after post-grace reclaim=%q, want ready", got)
	}
}

// TestReclaimExpiredTaskPausedIgnoresDisconnectAt confirms that a paused agent
// is exempt from reclaim regardless of disconnect_at.
func TestReclaimExpiredTaskPausedIgnoresDisconnectAt(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to set agent state")
	}

	d := NewApiDriver(t, "task-reclaim-paused-dc", "Task Reclaim Paused With DisconnectAt")
	sc := Given(d).
		TriagedIssue("Paused + disconnect_at", "TK-1365 paused always exempt", 2).
		Task(0, "Task held by paused+disconnected agent").
		Build()
	taskInt := sc.Tasks[0]
	taskID := fmt.Sprintf("TK-%d", taskInt)
	issueID := fmt.Sprintf("IS-%d", sc.Issues[0])

	agentID := registerAgent(t, d.Slug, "agent-paused-dc", "session-paused-dc", "")

	claim := claimTaskWithLease(t, d.Slug, agentID, issueID, 30)
	if claim.ID != taskID {
		t.Fatalf("claimed id=%q, want %q", claim.ID, taskID)
	}

	expireTaskLease(t, taskInt)
	registerAgentWithStatus(t, d.Slug, agentID, "session-paused-dc", "", "paused")
	// Set a stale disconnect_at that would otherwise trigger reclaim.
	setAgentDisconnectAt(t, agentID, -90*time.Second)

	var reclaimResp struct {
		Reclaimed int `json:"reclaimed"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/tasks/reclaim-expired", nil, &reclaimResp))
	if reclaimResp.Reclaimed != 0 {
		t.Fatalf("paused agent task was reclaimed: got reclaimed=%d, want 0", reclaimResp.Reclaimed)
	}
	if got := d.GetTask(taskInt).Status; got != "active" {
		t.Fatalf("task status=%q after paused-reclaim, want active", got)
	}
}

// setAgentDisconnectAt directly sets status='disconnected' and disconnect_at
// to now()+offset in the test database. offset is typically negative.
func setAgentDisconnectAt(t *testing.T, agentID string, offset time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, srv.DSN)
	if err != nil {
		t.Fatalf("pgx connect: %v", err)
	}
	defer conn.Close(ctx)
	disconnectAt := time.Now().Add(offset)
	if _, err := conn.Exec(ctx,
		`UPDATE zdx_agents SET status = 'disconnected', disconnect_at = $2 WHERE id = $1`,
		agentID, disconnectAt); err != nil {
		t.Fatalf("set agent disconnect_at: %v", err)
	}
}
