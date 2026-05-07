package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestTaskLeaseRenewalOwnership covers spec 71 on feature dx-todo-agent:
// only the claiming agent can renew an active task lease. The underlying
// RenewTaskReservation UPDATE filters by claimed_by, so a non-owner call
// must leave lease_expires_at untouched.
func TestTaskLeaseRenewalOwnership(t *testing.T) {
	d := NewApiDriver(t, "task-lease", "Task Lease Renewal Ownership")
	sc := Given(d).
		TriagedIssue("Lease renewal test", "spec 71 ownership invariant", 2).
		Task(0, "Task for lease renewal").
		Build()
	taskID := fmt.Sprintf("TK-%d", sc.Tasks[0])

	claim := claimTaskWithLease(t, d.Slug, "agent-A", fmt.Sprintf("IS-%d", sc.Issues[0]), 1)
	if claim.ID != taskID {
		t.Fatalf("claimed id=%q, want %q", claim.ID, taskID)
	}
	initialLease, err := time.Parse(time.RFC3339Nano, claim.LeaseExpiresAt)
	if err != nil {
		t.Fatalf("parse initial lease %q: %v", claim.LeaseExpiresAt, err)
	}

	// Agent A (owner) renews with 30 min — lease must extend past initial.
	renewLease(t, taskID, "agent-A", 30)
	afterOwner := leaseExpiresAt(t, d.Slug, taskID)
	if !afterOwner.After(initialLease) {
		t.Fatalf("owner renew did not extend lease: initial=%s after=%s",
			initialLease.Format(time.RFC3339Nano), afterOwner.Format(time.RFC3339Nano))
	}

	// Agent B (not the owner) renews the same task. Endpoint returns success
	// but the UPDATE filters on claimed_by = agent-A, so 0 rows mutate.
	renewLease(t, taskID, "agent-B", 30)
	afterNonOwner := leaseExpiresAt(t, d.Slug, taskID)
	if !afterNonOwner.Equal(afterOwner) {
		t.Fatalf("non-owner renew mutated lease: before=%s after=%s",
			afterOwner.Format(time.RFC3339Nano), afterNonOwner.Format(time.RFC3339Nano))
	}
}

type claimedTask struct {
	ID             string `json:"id"`
	LeaseExpiresAt string `json:"lease_expires_at"`
}

func claimTaskWithLease(t *testing.T, slug, agentID, issue string, leaseMin int32) claimedTask {
	t.Helper()
	var out claimedTask
	resp := apiDo(t, http.MethodPost, "/api/tasks/claim",
		map[string]any{
			"slug":               slug,
			"agent_id":           agentID,
			"task_group":         "",
			"issue":              issue,
			"lease_duration_min": leaseMin,
		}, &out)
	mustOK(t, resp)
	return out
}

func renewLease(t *testing.T, taskID, agentID string, leaseMin int32) {
	t.Helper()
	resp := apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/tasks/%s/renew-lease", taskID),
		map[string]any{"agent_id": agentID, "lease_duration_min": leaseMin}, nil)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("renew-lease want 2xx, got %d", resp.StatusCode)
	}
}

func leaseExpiresAt(t *testing.T, slug, taskID string) time.Time {
	t.Helper()
	var resp struct {
		Tasks []struct {
			ID             string `json:"id"`
			LeaseExpiresAt string `json:"lease_expires_at"`
		} `json:"tasks"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/agent/claims?slug=%s", slug), nil, &resp))
	for _, task := range resp.Tasks {
		if task.ID == taskID {
			ts, err := time.Parse(time.RFC3339Nano, task.LeaseExpiresAt)
			if err != nil {
				t.Fatalf("parse lease_expires_at %q: %v", task.LeaseExpiresAt, err)
			}
			return ts
		}
	}
	t.Fatalf("task %s not found in active claims", taskID)
	return time.Time{}
}
