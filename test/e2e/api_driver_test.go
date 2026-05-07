package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// ApiDriver implements Driver using direct HTTP API calls.
type ApiDriver struct {
	Slug string
	t    *testing.T
}

func NewApiDriver(t *testing.T, slug, name string) *ApiDriver {
	t.Helper()
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": name}, nil)
	return &ApiDriver{Slug: slug, t: t}
}

func (d *ApiDriver) CreateProject(slug, name string) {
	d.t.Helper()
	apiDo(d.t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": name}, nil)
}

func (d *ApiDriver) AddIssue(title, context string) int32 {
	d.t.Helper()
	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": d.Slug, "title": title, "context": context, "auto_ready": true}, &issue))
	return issue.ID
}

func (d *ApiDriver) TriageIssue(id, priority int32) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/owner/triage",
		map[string]any{"slug": d.Slug, "id": id, "priority": priority}, nil))
}

func (d *ApiDriver) TriageIssueWithBranch(id, priority int32, branch string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/owner/triage",
		map[string]any{"slug": d.Slug, "id": id, "priority": priority, "target_branch": branch}, nil))
}

func (d *ApiDriver) CloseIssue(id int32) {
	d.t.Helper()
	// Satisfy the work-log close gate (IS-629): every close-with-reason=done
	// requires at least one substantive (non-bracketed) work-log entry.
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/issue-work",
		map[string]any{"issue_id": id, "by_role": "test", "note": "test close"}, nil))
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{"slug": d.Slug, "id": id, "reason": "done"}, nil))
}

func (d *ApiDriver) AddTask(issueID int32, text string) int32 {
	d.t.Helper()
	issue := fmt.Sprintf("IS-%d", issueID)
	var task struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{"slug": d.Slug, "text": text, "issue": issue, "auto_ready": true}, &task))
	return task.ID
}

type AddTaskResult struct {
	ID               int32 `json:"id"`
	DuplicateBlocked bool  `json:"duplicate_blocked"`
}

func (d *ApiDriver) AddTaskCustom(body map[string]any) AddTaskResult {
	d.t.Helper()
	body["slug"] = d.Slug
	var resp AddTaskResult
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/tech/add", body, &resp))
	return resp
}

func (d *ApiDriver) MarkTaskDone(taskID int32) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/dev/done",
		map[string]any{"id": taskID}, nil))
}

func (d *ApiDriver) MarkTaskUndone(taskID int32) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/dev/undone",
		map[string]any{"id": taskID}, nil))
}

func (d *ApiDriver) GetTask(taskID int32) TaskInfo {
	d.t.Helper()
	var resp TaskInfo
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/task?slug=%s&id=TK-%d", d.Slug, taskID), nil, &resp))
	return resp
}

func (d *ApiDriver) AddFeature(name, desc string) int32 {
	d.t.Helper()
	var feat struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/feature",
		map[string]any{"slug": d.Slug, "name": name, "description": desc}, &feat))
	return feat.ID
}

func (d *ApiDriver) AddSpec(feature, kind, desc string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/specs/update",
		map[string]any{"slug": d.Slug, "feature": feature, "field": kind, "value": desc}, nil))
}

func (d *ApiDriver) DeferSpec(specID int32) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/specs/defer",
		map[string]any{"spec_id": specID}, nil))
}

func (d *ApiDriver) ReviewFeature(name string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/feature/review",
		map[string]any{"slug": d.Slug, "feature": name}, nil))
}

func (d *ApiDriver) AddGoal(title string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/goal",
		map[string]any{"slug": d.Slug, "title": title, "description": "test goal", "priority": 1, "status": "active"}, nil))
}

// AddConstraint is a no-op shim. zdx_project_constraints + /api/constraint
// were removed in IS-627 (migration 147). Kept on the Driver to satisfy the
// ConstraintSteps interface and source-compat with existing tests.
func (d *ApiDriver) AddConstraint(_ string) {
	d.t.Helper()
}

func (d *ApiDriver) AddComment(targetType, targetID, body string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": d.Slug, "target_type": targetType, "target_id": targetID, "body": body}, nil))
}

// AddCommentAs posts a comment with an explicit author alias (e.g. an agent reply).
func (d *ApiDriver) AddCommentAs(targetType, targetID, body, authorAlias string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": d.Slug, "target_type": targetType, "target_id": targetID, "body": body, "author_alias": authorAlias}, nil))
}

func (d *ApiDriver) AddBlockerQuestion(targetType, targetID, context string) int32 {
	d.t.Helper()
	var q struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{"slug": d.Slug, "target_type": targetType, "target_id": targetID, "context": context}, &q))
	return q.ID
}

func (d *ApiDriver) AnswerBlockerQuestion(id int32, answer string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/blocker-questions/answer",
		map[string]any{"slug": d.Slug, "id": id, "answer": answer, "answered_by": "test"}, nil))
}

func (d *ApiDriver) ListPendingBlockerQuestions() []BlockerQuestionInfo {
	d.t.Helper()
	var resp struct {
		Questions []BlockerQuestionInfo `json:"questions"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/blocker-questions/list?slug=%s&status=pending", d.Slug), nil, &resp))
	return resp.Questions
}

func (d *ApiDriver) AddQuestion(category, question string) int32 {
	d.t.Helper()
	var resp struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/qa/add",
		map[string]any{"slug": d.Slug, "category": category, "question": question}, &resp))
	return resp.ID
}

func (d *ApiDriver) AnswerQuestion(id int32, answer string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/qa/answer",
		map[string]any{"slug": d.Slug, "id": id, "answer": answer}, nil))
}

func (d *ApiDriver) ListUnansweredQuestions() []QAQuestionInfo {
	d.t.Helper()
	var resp struct {
		Questions []QAQuestionInfo `json:"questions"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/qa/unanswered?slug=%s", d.Slug), nil, &resp))
	return resp.Questions
}

func (d *ApiDriver) CheckinJournal(role, date string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/journal/checkin",
		map[string]any{
			"slug": d.Slug, "role": role, "date": date,
			"tldr": "test", "assessment": "ok", "concerns": "none", "next": "continue",
		}, nil))
}

func (d *ApiDriver) RegisterTest(name string) int32 {
	return d.RegisterTestWithLayer(name, "")
}

func (d *ApiDriver) RegisterTestWithLayer(name, layer string) int32 {
	d.t.Helper()
	result := map[string]any{"driver": "go", "test_name": name, "feature": "test", "status": "pass", "duration_ms": 100}
	if layer != "" {
		result["layer"] = layer
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/test-results/submit",
		map[string]any{
			"slug":    d.Slug,
			"results": []map[string]any{result},
		}, nil))
	var resp struct {
		Tests []struct {
			ID   int32  `json:"id"`
			Name string `json:"name"`
		} `json:"tests"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/tests?slug=%s", d.Slug), nil, &resp))
	for _, t := range resp.Tests {
		if t.Name == name {
			return t.ID
		}
	}
	d.t.Fatalf("test %q not found after registration", name)
	return 0
}

func (d *ApiDriver) GetHealth() (goalCount, constraintCount, closedTaskCount int64, ownerJournalDate, techJournalDate string) {
	d.t.Helper()
	var resp struct {
		GoalCount        int64  `json:"goal_count"`
		ConstraintCount  int64  `json:"constraint_count"`
		OwnerJournalDate string `json:"owner_journal_date"`
		TechJournalDate  string `json:"tech_journal_date"`
		ClosedTaskCount  int64  `json:"closed_task_count"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/agent/health?slug=%s", d.Slug), nil, &resp))
	return resp.GoalCount, resp.ConstraintCount, resp.ClosedTaskCount, resp.OwnerJournalDate, resp.TechJournalDate
}

func (d *ApiDriver) ListIssues() []IssueInfo {
	d.t.Helper()
	var resp struct {
		Issues []IssueInfo `json:"issues"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/list?slug=%s", d.Slug), nil, &resp))
	return resp.Issues
}

func (d *ApiDriver) ListTasks(issueID int32) []TaskInfo {
	d.t.Helper()
	var resp struct {
		Tasks []TaskInfo `json:"tasks"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/tasks?slug=%s&issue_id=IS-%d", d.Slug, issueID), nil, &resp))
	return resp.Tasks
}

func (d *ApiDriver) ListFeatures() []FeatureInfo {
	d.t.Helper()
	var resp struct {
		Features []FeatureInfo `json:"features"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/features?slug=%s", d.Slug), nil, &resp))
	return resp.Features
}

func (d *ApiDriver) ListUncoveredSpecs() []SpecInfo {
	d.t.Helper()
	var resp struct {
		Specs []SpecInfo `json:"specs"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/specs/uncovered?slug=%s", d.Slug), nil, &resp))
	return resp.Specs
}

func (d *ApiDriver) ListStaleFeatures() []FeatureInfo {
	d.t.Helper()
	var resp struct {
		Features []FeatureInfo `json:"features"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/features/stale?slug=%s", d.Slug), nil, &resp))
	return resp.Features
}

func (d *ApiDriver) LinkTestToSpec(specID, testID int32) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/specs/link-test",
		map[string]any{"spec_id": specID, "test_id": testID}, nil))
}

func (d *ApiDriver) UnlinkTestFromSpec(specID, testID int32) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/specs/unlink-test",
		map[string]any{"spec_id": specID, "test_id": testID}, nil))
}

type SpecTestRow struct {
	ID        int32  `json:"id"`
	Component string `json:"component"`
	Name      string `json:"name"`
	Layer     string `json:"layer"`
	Status    string `json:"status"`
}

func (d *ApiDriver) ListSpecTests(specID int32) []SpecTestRow {
	d.t.Helper()
	var resp struct {
		Tests []SpecTestRow `json:"tests"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/specs/tests?spec_id=%d", specID), nil, &resp))
	return resp.Tests
}

type StaleTaskInfo struct {
	ID         int32  `json:"id"`
	Text       string `json:"text"`
	StaleSince string `json:"stale_since"`
}

func (d *ApiDriver) ListStaleTasks(issue string) []StaleTaskInfo {
	d.t.Helper()
	url := fmt.Sprintf("/api/dx/tasks/stale?slug=%s", d.Slug)
	if issue != "" {
		url += "&issue=" + issue
	}
	var resp struct {
		Tasks []StaleTaskInfo `json:"tasks"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet, url, nil, &resp))
	return resp.Tasks
}

func (d *ApiDriver) SweepStaleTasks(staleDays int32) int {
	d.t.Helper()
	var resp struct {
		Flagged int `json:"flagged"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/tasks/sweep-stale",
		map[string]any{"slug": d.Slug, "stale_days": staleDays}, &resp))
	return resp.Flagged
}

type AgentQueueItem struct {
	Key          string `json:"key"`
	Text         string `json:"text"`
	Kind         string `json:"kind"`
	TargetType   string `json:"target_type"`
	TargetID     string `json:"target_id"`
	IssueRef     string `json:"issue_ref"`
	TargetBranch string `json:"target_branch"`
	Priority     int32  `json:"priority"`
	Blocked      bool   `json:"blocked"`
	Persona      string `json:"persona"`
	Status       string `json:"status"`
}

func (d *ApiDriver) EvaluateQueue(issue string) []AgentQueueItem {
	d.t.Helper()
	body := map[string]any{"slug": d.Slug, "issue": issue}
	// Include Changed entries too: any state mutation between handler-side
	// async refreshes (e.g. issue priority bumps re-folding into the dev
	// candidate's priority) lands proposed items in Changed instead of
	// Unchanged, and dropping that bucket made queue assertions order-flaky.
	var resp struct {
		Added     []AgentQueueItem `json:"added"`
		Removed   []any            `json:"removed"`
		Changed   []EvaluateChange `json:"changed"`
		Unchanged []AgentQueueItem `json:"unchanged"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/agent/evaluate", body, &resp))
	var all []AgentQueueItem
	all = append(all, resp.Added...)
	all = append(all, resp.Unchanged...)
	for _, c := range resp.Changed {
		all = append(all, c.After)
	}
	return all
}

type EvaluateChange struct {
	Before TodoItem       `json:"before"`
	After  AgentQueueItem `json:"after"`
}

type EvaluateDiffResult struct {
	Added     []AgentQueueItem `json:"added"`
	Removed   []TodoItem       `json:"removed"`
	Changed   []EvaluateChange `json:"changed"`
	Unchanged []AgentQueueItem `json:"unchanged"`
}

func (d *ApiDriver) EvaluateDiff(issue string) EvaluateDiffResult {
	d.t.Helper()
	body := map[string]any{"slug": d.Slug, "issue": issue}
	var resp EvaluateDiffResult
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/agent/evaluate", body, &resp))
	return resp
}

func findKind(items []AgentQueueItem, kind string) *AgentQueueItem {
	for i := range items {
		if items[i].Kind == kind {
			return &items[i]
		}
	}
	return nil
}

func requireKind(t *testing.T, items []AgentQueueItem, kind string) AgentQueueItem {
	t.Helper()
	item := findKind(items, kind)
	if item == nil {
		kinds := make([]string, len(items))
		for i, it := range items {
			kinds[i] = it.Kind
		}
		t.Fatalf("expected Kind %q in queue, got kinds: %v", kind, kinds)
	}
	return *item
}

func requireNoKind(t *testing.T, items []AgentQueueItem, kind string) {
	t.Helper()
	if findKind(items, kind) != nil {
		t.Fatalf("expected Kind %q to NOT be in queue, but it was", kind)
	}
}
