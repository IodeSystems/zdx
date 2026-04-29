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

func (d *ApiDriver) CloseIssue(id int32) {
	d.t.Helper()
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

func (d *ApiDriver) AddConstraint(title string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/constraint",
		map[string]any{"slug": d.Slug, "title": title, "description": "test constraint", "priority": 1, "status": "active"}, nil))
}

func (d *ApiDriver) AddComment(targetType, targetID, body string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": d.Slug, "target_type": targetType, "target_id": targetID, "body": body}, nil))
}

func (d *ApiDriver) MarkCommentsRead(targetType, targetID, role string) {
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/comment/mark-read",
		map[string]any{"slug": d.Slug, "target_type": targetType, "target_id": targetID, "role": role}, nil))
}

func (d *ApiDriver) HasUnreadComments(targetType, targetID string) bool {
	d.t.Helper()
	var resp struct {
		HasUnread bool `json:"has_unread"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/comment/unread-check?slug=%s&target_type=%s&target_id=%s&role=llm",
			d.Slug, targetType, targetID), nil, &resp))
	return resp.HasUnread
}

func (d *ApiDriver) ListStaleUnreadComments(ageHours int) []StaleCommentInfo {
	d.t.Helper()
	var resp struct {
		Comments []StaleCommentInfo `json:"comments"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodGet,
		fmt.Sprintf("/api/dx/comment/stale-unread?slug=%s&role=llm&age_hours=%d", d.Slug, ageHours), nil, &resp))
	return resp.Comments
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
	d.t.Helper()
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/test-results/submit",
		map[string]any{
			"slug": d.Slug,
			"results": []map[string]any{
				{"driver": "go", "test_name": name, "feature": "test", "status": "pass", "duration_ms": 100},
			},
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
		fmt.Sprintf("/api/dx/solo/health?slug=%s", d.Slug), nil, &resp))
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

type SoloQueueItem struct {
	Key        string `json:"key"`
	Text       string `json:"text"`
	Kind       string `json:"kind"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	IssueRef   string `json:"issue_ref"`
	Priority   int32  `json:"priority"`
	Blocked    bool   `json:"blocked"`
	Persona    string `json:"persona"`
	Status     string `json:"status"`
}

func (d *ApiDriver) EvaluateQueue(issue string) []SoloQueueItem {
	d.t.Helper()
	body := map[string]any{"slug": d.Slug, "issue": issue}
	var resp struct {
		Added     []SoloQueueItem `json:"added"`
		Removed   []any           `json:"removed"`
		Changed   []any           `json:"changed"`
		Unchanged []SoloQueueItem `json:"unchanged"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/solo/evaluate", body, &resp))
	var all []SoloQueueItem
	all = append(all, resp.Added...)
	all = append(all, resp.Unchanged...)
	return all
}

type EvaluateChange struct {
	Before TodoItem      `json:"before"`
	After  SoloQueueItem `json:"after"`
}

type EvaluateDiffResult struct {
	Added     []SoloQueueItem  `json:"added"`
	Removed   []TodoItem       `json:"removed"`
	Changed   []EvaluateChange `json:"changed"`
	Unchanged []SoloQueueItem  `json:"unchanged"`
}

func (d *ApiDriver) EvaluateDiff(issue string) EvaluateDiffResult {
	d.t.Helper()
	body := map[string]any{"slug": d.Slug, "issue": issue}
	var resp EvaluateDiffResult
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/solo/evaluate", body, &resp))
	return resp
}

func findKind(items []SoloQueueItem, kind string) *SoloQueueItem {
	for i := range items {
		if items[i].Kind == kind {
			return &items[i]
		}
	}
	return nil
}

func requireKind(t *testing.T, items []SoloQueueItem, kind string) SoloQueueItem {
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

func requireNoKind(t *testing.T, items []SoloQueueItem, kind string) {
	t.Helper()
	if findKind(items, kind) != nil {
		t.Fatalf("expected Kind %q to NOT be in queue, but it was", kind)
	}
}
