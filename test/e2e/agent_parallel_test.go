package e2e

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
)

func TestAgentRegisterAndClaim(t *testing.T) {
	ss := newSoloState(t, "agent-claim", "Agent Claim")
	issueID := ss.addIssue("Agent claim test", "test concurrent agent claims")
	ss.triageIssue(issueID, 2)
	taskID := ss.addTask(issueID, "Task for agent claim")

	var agent struct {
		ID string `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/agents/register",
		map[string]any{
			"slug":       ss.slug,
			"id":         "agent-claim-1",
			"session_id": "test-session-1",
			"task_group": "",
		}, &agent))
	if agent.ID == "" {
		t.Fatal("agent registration returned empty ID")
	}

	var claimed struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	}
	resp := apiDo(t, http.MethodPost, "/api/tasks/claim",
		map[string]any{
			"slug":       ss.slug,
			"agent_id":   agent.ID,
			"task_group": "",
			"issue":      fmt.Sprintf("IS-%d", issueID),
		}, &claimed)
	mustOK(t, resp)
	if claimed.ID == "" {
		t.Fatal("claim returned empty task ID")
	}

	_ = taskID
}

func TestTwoAgentsConcurrentClaim(t *testing.T) {
	ss := newSoloState(t, "agent-concurrent", "Agent Concurrent")
	issueID := ss.addIssue("Concurrent claim test", "two agents claim different tasks")
	ss.triageIssue(issueID, 2)
	ss.addTask(issueID, "Task A for concurrent claim")
	ss.addTask(issueID, "Task B for concurrent claim")

	var agent1, agent2 struct {
		ID string `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/agents/register",
		map[string]any{
			"slug":       ss.slug,
			"id":         "concurrent-agent-a",
			"session_id": "session-a",
			"task_group": "",
		}, &agent1))
	mustOK(t, apiDo(t, http.MethodPost, "/api/agents/register",
		map[string]any{
			"slug":       ss.slug,
			"id":         "concurrent-agent-b",
			"session_id": "session-b",
			"task_group": "",
		}, &agent2))

	var wg sync.WaitGroup
	claimed := make([]string, 2)
	errors := make([]error, 2)

	claimTask := func(idx int, agentID string) {
		defer wg.Done()
		var result struct {
			ID string `json:"id"`
		}
		resp := apiDo(t, http.MethodPost, "/api/tasks/claim",
			map[string]any{
				"slug":       ss.slug,
				"agent_id":   agentID,
				"task_group": "",
				"issue":      fmt.Sprintf("IS-%d", issueID),
			}, &result)
		if resp.StatusCode != http.StatusOK {
			errors[idx] = fmt.Errorf("claim failed with status %d", resp.StatusCode)
			return
		}
		claimed[idx] = result.ID
	}

	wg.Add(2)
	go claimTask(0, agent1.ID)
	go claimTask(1, agent2.ID)
	wg.Wait()

	for i, err := range errors {
		if err != nil {
			t.Fatalf("agent %d: %v", i, err)
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
	ss := newSoloState(t, "agent-affinity", "Agent Affinity")
	issueID := ss.addIssue("Task group affinity test", "agents claim tasks from their group")
	ss.triageIssue(issueID, 2)

	addGroupTask := func(text, group string) int32 {
		t.Helper()
		issue := fmt.Sprintf("IS-%d", issueID)
		var task struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
			map[string]any{"slug": ss.slug, "text": text, "issue": issue, "task_group": group, "auto_ready": true}, &task))
		return task.ID
	}

	addGroupTask("Frontend task", "frontend")
	addGroupTask("Backend task", "backend")

	var feAgent struct {
		ID string `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/agents/register",
		map[string]any{
			"slug":       ss.slug,
			"id":         "fe-agent",
			"session_id": "fe-session",
			"task_group": "frontend",
		}, &feAgent))

	var claimed struct {
		ID        string `json:"id"`
		Text      string `json:"text"`
		TaskGroup string `json:"task_group"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/tasks/claim",
		map[string]any{
			"slug":       ss.slug,
			"agent_id":   feAgent.ID,
			"task_group": "frontend",
			"issue":      fmt.Sprintf("IS-%d", issueID),
		}, &claimed))

	if claimed.TaskGroup != "frontend" {
		t.Errorf("expected frontend task, got group %q (task: %s)", claimed.TaskGroup, claimed.Text)
	}
}

func TestReviewAfterDone(t *testing.T) {
	ss := newSoloState(t, "agent-review", "Agent Review")
	issueID := ss.addIssue("Review workflow test", "test review emission")
	ss.triageIssue(issueID, 2)
	taskID := ss.addTask(issueID, "Task to review")
	ss.markTaskDone(taskID)

	var unreviewed struct {
		Tasks []struct {
			ID   int32  `json:"id"`
			Text string `json:"text"`
		} `json:"tasks"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/dev/unreviewed?slug=%s&issue=IS-%d", ss.slug, issueID),
		nil, &unreviewed))
	if len(unreviewed.Tasks) != 1 {
		t.Fatalf("expected 1 unreviewed task, got %d", len(unreviewed.Tasks))
	}
	if unreviewed.Tasks[0].ID != taskID {
		t.Errorf("expected task %d, got %d", taskID, unreviewed.Tasks[0].ID)
	}

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/review",
		map[string]any{
			"slug":    ss.slug,
			"id":      taskID,
			"verdict": "approve",
			"comment": "LGTM",
		}, nil))

	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/dev/unreviewed?slug=%s&issue=IS-%d", ss.slug, issueID),
		nil, &unreviewed))
	if len(unreviewed.Tasks) != 0 {
		t.Fatalf("expected 0 unreviewed tasks after review, got %d", len(unreviewed.Tasks))
	}
}
