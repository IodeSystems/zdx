package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestReclaimExpiredTask covers spec 69: /api/tasks/reclaim-expired resets
// active tasks with an expired lease back to "ready" so they can be re-claimed.
func TestReclaimExpiredTask(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable (external runner mode); skipping reclaim-expired test (needs direct DB access to backdate lease)")
	}

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, srv.DSN)
	if err != nil {
		t.Fatalf("pgx connect: %v", err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx,
		`UPDATE zdx_reservations SET lease_expires_at = NOW() - interval '1 minute'
		 WHERE target_type = 'task' AND target_id = $1 AND released_at IS NULL`,
		fmt.Sprintf("TK-%d", taskInt)); err != nil {
		t.Fatalf("backdate lease: %v", err)
	}
}
