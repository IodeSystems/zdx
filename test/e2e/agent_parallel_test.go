package e2e

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func registerAgent(t *testing.T, slug, id, sessionID, taskGroup string) string {
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
			"status":          "active",
			"task_group":      taskGroup,
			"compose_project": "",
			"server_port":     0,
			"database_url":    "",
			"valkey_url":      "",
		}, &agent))
	return agent.ID
}

func claimTask(t *testing.T, slug, agentID, taskGroup, issue string) (string, string, int) {
	t.Helper()
	var result struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		TaskGroup string `json:"task_group"`
	}
	resp := apiDo(t, http.MethodPost, "/api/tasks/claim",
		map[string]any{
			"slug":               slug,
			"agent_id":           agentID,
			"task_group":         taskGroup,
			"issue":              issue,
			"lease_duration_min": 30,
		}, &result)
	return result.ID, result.TaskGroup, resp.StatusCode
}

func TestAgentRegisterAndClaim(t *testing.T) {
	d := NewApiDriver(t, "agent-claim", "Agent Claim")
	sc := Given(d).
		TriagedIssue("Agent claim test", "test concurrent agent claims", 2).
		Task(0, "Task for agent claim").
		Build()
	issueID := sc.Issues[0]

	agentID := registerAgent(t, d.Slug, "agent-claim-1", "test-session-1", "")
	if agentID == "" {
		t.Fatal("agent registration returned empty ID")
	}

	claimedID, _, status := claimTask(t, d.Slug, agentID, "", fmt.Sprintf("IS-%d", issueID))
	if status != http.StatusOK {
		t.Fatalf("claim failed with status %d", status)
	}
	if claimedID == "" {
		t.Fatal("claim returned empty task ID")
	}
}

func TestTwoAgentsConcurrentClaim(t *testing.T) {
	d := NewApiDriver(t, "agent-concurrent", "Agent Concurrent")
	sc := Given(d).
		TriagedIssue("Concurrent claim test", "two agents claim different tasks", 2).
		Task(0, "Task A for concurrent claim").
		Task(0, "Task B for concurrent claim").
		Build()
	issueID := sc.Issues[0]

	agent1ID := registerAgent(t, d.Slug, "concurrent-agent-a", "session-a", "")
	agent2ID := registerAgent(t, d.Slug, "concurrent-agent-b", "session-b", "")

	var wg sync.WaitGroup
	claimed := make([]string, 2)
	statuses := make([]int, 2)

	doClaim := func(idx int, agentID string) {
		defer wg.Done()
		id, _, st := claimTask(t, d.Slug, agentID, "", fmt.Sprintf("IS-%d", issueID))
		claimed[idx] = id
		statuses[idx] = st
	}

	wg.Add(2)
	go doClaim(0, agent1ID)
	go doClaim(1, agent2ID)
	wg.Wait()

	for i, st := range statuses {
		if st != http.StatusOK {
			t.Fatalf("agent %d: claim failed with status %d", i, st)
		}
	}

	if claimed[0] == "" || claimed[1] == "" {
		t.Fatal("one or both agents failed to claim a task")
	}
	if claimed[0] == claimed[1] {
		t.Fatalf("both agents claimed the same task: %s", claimed[0])
	}
}

func TestTaskGroupAffinity(t *testing.T) {
	d := NewApiDriver(t, "agent-affinity", "Agent Affinity")
	sc := Given(d).
		TriagedIssue("Task group affinity test", "agents claim tasks from their group", 2).
		Build()
	issueID := sc.Issues[0]

	addGroupTask := func(text, group string) int32 {
		t.Helper()
		issue := fmt.Sprintf("IS-%d", issueID)
		var task struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
			map[string]any{"slug": d.Slug, "text": text, "issue": issue, "task_group": group, "auto_ready": true}, &task))
		return task.ID
	}

	addGroupTask("Frontend task", "frontend")
	addGroupTask("Backend task", "backend")

	feAgentID := registerAgent(t, d.Slug, "fe-agent", "fe-session", "frontend")

	claimedID, claimedGroup, status := claimTask(t, d.Slug, feAgentID, "frontend", fmt.Sprintf("IS-%d", issueID))
	if status != http.StatusOK {
		t.Fatalf("claim failed with status %d", status)
	}

	if claimedGroup != "frontend" {
		t.Errorf("expected frontend task group, got %q (task: %s)", claimedGroup, claimedID)
	}
}

func TestReviewAfterDone(t *testing.T) {
	d := NewApiDriver(t, "agent-review", "Agent Review")
	sc := Given(d).
		TriagedIssue("Review workflow test", "test review emission", 2).
		Task(0, "Task to review").
		Build()
	issueID := sc.Issues[0]
	taskID := sc.Tasks[0]

	d.MarkTaskDone(taskID)

	var unreviewed struct {
		Tasks []struct {
			ID   int32  `json:"id"`
			Text string `json:"text"`
		} `json:"tasks"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/dev/unreviewed?slug=%s&issue=IS-%d", d.Slug, issueID),
		nil, &unreviewed))
	if len(unreviewed.Tasks) != 1 {
		t.Fatalf("expected 1 unreviewed task, got %d", len(unreviewed.Tasks))
	}
	if unreviewed.Tasks[0].ID != taskID {
		t.Errorf("expected task %d, got %d", taskID, unreviewed.Tasks[0].ID)
	}

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/review",
		map[string]any{
			"slug":    d.Slug,
			"id":      taskID,
			"verdict": "approve",
		}, nil))

	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/dev/unreviewed?slug=%s&issue=IS-%d", d.Slug, issueID),
		nil, &unreviewed))
	if len(unreviewed.Tasks) != 0 {
		t.Fatalf("expected 0 unreviewed tasks after review, got %d", len(unreviewed.Tasks))
	}

	// Review must not produce a comment on the task — would otherwise feed
	// back into solo's stale-unread-comment queue and risk an auto-loop.
	taskIDStr := fmt.Sprintf("TK-%d", taskID)
	if d.HasUnreadComments("task", taskIDStr) {
		t.Fatal("review created an unread comment on the task; reviewer cycle should not use comments")
	}
}
