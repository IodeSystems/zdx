package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_AgentLeaseRenewal is the demo for spec 71 on feature
// dx-todo-agent: given an agent holding an active task lease, when it calls
// renew-lease with its agent-id, then lease_expires_at is extended — and only
// the claiming agent can renew.
//
// The test drives POST /api/tasks/{id}/renew-lease twice: once as the owner
// (lease must extend) and once as a non-owner (lease must not change).
func TestDemoAPI_AgentLeaseRenewal(t *testing.T) {
	rec := newApiRecorder(t, "agent-lease-renewal")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_agent_lease_renewal_test.go", Note: "lease-renewal demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_agents.go", Note: "POST /api/tasks/{id}/renew-lease handler"})
	rec.AddCoderef(coderef{FilePath: "queries/tasks.sql", Note: "RenewTaskReservation filters UPDATE on claimed_by"})
	t.Cleanup(rec.Save)

	const slug = "demo-lease-renewal"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           slug,
		"name":           "Demo Agent Lease Renewal",
		"classification": "service",
	}, nil))

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       slug,
		"title":      "Implement lease renewal ownership check",
		"context":    "agent must extend its own lease; others must not interfere",
		"auto_ready": true,
	}, &issue))

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/owner/triage", map[string]any{
		"slug":     slug,
		"id":       issue.ID,
		"priority": 2,
	}, nil))

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
		"slug":       slug,
		"text":       "Verify renew-lease ownership invariant",
		"issue":      fmt.Sprintf("IS-%d", issue.ID),
		"auto_ready": true,
		"force":      true,
	}, nil))

	// Agent A claims the task with a 1-minute lease.
	var claimed struct {
		ID             string `json:"id"`
		LeaseExpiresAt string `json:"lease_expires_at"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/tasks/claim", map[string]any{
		"slug":               slug,
		"agent_id":           "demo-agent-A",
		"task_group":         "",
		"issue":              fmt.Sprintf("IS-%d", issue.ID),
		"lease_duration_min": 1,
	}, &claimed))
	taskID := claimed.ID

	initial, err := time.Parse(time.RFC3339Nano, claimed.LeaseExpiresAt)
	if err != nil {
		t.Fatalf("parse initial lease_expires_at %q: %v", claimed.LeaseExpiresAt, err)
	}

	// Owner (agent-A) renews with a longer lease — lease_expires_at must extend.
	mustNoContent(t, rec.Do(http.MethodPost, fmt.Sprintf("/api/tasks/%s/renew-lease", taskID), map[string]any{
		"agent_id":           "demo-agent-A",
		"lease_duration_min": 30,
	}, nil))

	afterOwner := getLeaseExpiresAt(t, rec, slug, taskID)
	if !afterOwner.After(initial) {
		t.Fatalf("owner renew did not extend lease: initial=%s after=%s",
			initial.Format(time.RFC3339Nano), afterOwner.Format(time.RFC3339Nano))
	}

	// Non-owner (agent-B) calls renew-lease on the same task.
	// The UPDATE filters on claimed_by = agent-A, so 0 rows mutate.
	mustNoContent(t, rec.Do(http.MethodPost, fmt.Sprintf("/api/tasks/%s/renew-lease", taskID), map[string]any{
		"agent_id":           "demo-agent-B",
		"lease_duration_min": 30,
	}, nil))

	afterNonOwner := getLeaseExpiresAt(t, rec, slug, taskID)
	if !afterNonOwner.Equal(afterOwner) {
		t.Fatalf("non-owner renew mutated lease: before=%s after=%s",
			afterOwner.Format(time.RFC3339Nano), afterNonOwner.Format(time.RFC3339Nano))
	}
}

func getLeaseExpiresAt(t *testing.T, rec *ApiDemoRecorder, slug, taskID string) time.Time {
	t.Helper()
	var resp struct {
		Tasks []struct {
			ID             string `json:"id"`
			LeaseExpiresAt string `json:"lease_expires_at"`
		} `json:"tasks"`
	}
	mustOK(t, rec.Do(http.MethodGet,
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
