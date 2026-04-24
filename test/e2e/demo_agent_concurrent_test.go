package e2e

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

// TestDemoAPI_ConcurrentTaskClaim is the demo for spec 68: multiple agents
// calling POST /api/tasks/claim concurrently each receive a distinct task.
// The database uses FOR UPDATE SKIP LOCKED so concurrent claimants skip rows
// already locked by a peer transaction, preventing double-assignment.
func TestDemoAPI_ConcurrentTaskClaim(t *testing.T) {
	rec := newApiRecorder(t, "concurrent-task-claim")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_agent_concurrent_test.go", Note: "concurrent claim demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/db/tasks.sql.go", Note: "ClaimTask uses FOR UPDATE SKIP LOCKED"})
	t.Cleanup(rec.Save)

	const agentCount = 3

	// Create a project and an issue with enough ready tasks for every agent.
	var proj struct {
		Slug string `json:"slug"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-concurrent-claim",
		"name":           "Demo Concurrent Claim",
		"classification": "service",
	}, &proj))

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       proj.Slug,
		"title":      "Implement concurrent claim feature",
		"context":    "three agents will claim tasks simultaneously",
		"auto_ready": true,
	}, &issue))

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/owner/triage", map[string]any{
		"slug":     proj.Slug,
		"id":       issue.ID,
		"priority": 2,
	}, nil))

	issueRef := fmt.Sprintf("IS-%d", issue.ID)
	for i := range agentCount {
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
			"slug":       proj.Slug,
			"text":       fmt.Sprintf("Concurrent claim task %d", i+1),
			"issue":      issueRef,
			"auto_ready": true,
			"force":      true,
		}, nil))
	}

	// Register one agent per goroutine.
	agentIDs := make([]string, agentCount)
	for i := range agentCount {
		agentIDs[i] = fmt.Sprintf("concurrent-demo-agent-%d", i+1)
		mustOK(t, rec.Do(http.MethodPost, "/api/agents/register", map[string]any{
			"slug":            proj.Slug,
			"id":              agentIDs[i],
			"session_id":      fmt.Sprintf("demo-session-%d", i+1),
			"worktree_path":   "",
			"worktree_branch": "",
			"pid":             0,
			"status":          "active",
			"task_group":      "",
			"compose_project": "",
			"server_port":     0,
			"database_url":    "",
			"valkey_url":      "",
		}, nil))
	}

	// All agents claim concurrently. Each must receive a distinct task ID.
	claimed := make([]string, agentCount)
	statuses := make([]int, agentCount)
	var wg sync.WaitGroup
	for i := range agentCount {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var result struct {
				ID string `json:"id"`
			}
			resp := apiDo(t, http.MethodPost, "/api/tasks/claim", map[string]any{
				"slug":               proj.Slug,
				"agent_id":           agentIDs[idx],
				"task_group":         "",
				"issue":              issueRef,
				"lease_duration_min": 5,
			}, &result)
			claimed[idx] = result.ID
			statuses[idx] = resp.StatusCode
		}(i)
	}
	wg.Wait()

	for i, st := range statuses {
		if st != http.StatusOK {
			t.Errorf("agent %d: claim returned status %d, want 200", i+1, st)
		}
	}

	seen := map[string]int{}
	for i, id := range claimed {
		if id == "" {
			t.Errorf("agent %d: claimed empty task ID", i+1)
			continue
		}
		if prev, dup := seen[id]; dup {
			t.Errorf("agents %d and %d both claimed the same task %s (double-assignment)", prev+1, i+1, id)
		}
		seen[id] = i
	}
}
