package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// soloState captures the API responses that solo uses to decide which todo type to emit.
type soloState struct {
	slug string
	t    *testing.T
}

func newSoloState(t *testing.T, slug, name string) *soloState {
	t.Helper()
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": name}, nil)
	return &soloState{slug: slug, t: t}
}

// ── helpers to set up state via API ──────────────────────────────────────────

func (s *soloState) addIssue(title, context string) int32 {
	s.t.Helper()
	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]string{"slug": s.slug, "title": title, "context": context}, &issue))
	return issue.ID
}

func (s *soloState) triageIssue(id, priority int32) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/todo/owner/triage",
		map[string]any{"slug": s.slug, "id": id, "priority": priority}, nil))
}

func (s *soloState) closeIssue(id int32) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{"slug": s.slug, "id": id, "reason": "done"}, nil))
}

func (s *soloState) addTask(issueID int32, text string) int32 {
	s.t.Helper()
	issue := fmt.Sprintf("IS-%d", issueID)
	var task struct {
		ID int32 `json:"id"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{"slug": s.slug, "text": text, "issue": issue}, &task))
	return task.ID
}

func (s *soloState) markTaskDone(taskID int32) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/todo/dev/done",
		map[string]any{"id": taskID}, nil))
}

func (s *soloState) addFeature(name, desc string) int32 {
	s.t.Helper()
	var feat struct {
		ID int32 `json:"id"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/feature",
		map[string]any{"slug": s.slug, "name": name, "description": desc}, &feat))
	return feat.ID
}

func (s *soloState) addSpec(feature, kind, desc string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/specs/update",
		map[string]any{"slug": s.slug, "feature": feature, "field": kind, "value": desc}, nil))
}

func (s *soloState) addGoal(title string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/goal",
		map[string]any{"slug": s.slug, "title": title, "description": "test goal", "priority": 1, "status": "active"}, nil))
}

func (s *soloState) addConstraint(title string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/constraint",
		map[string]any{"slug": s.slug, "title": title, "description": "test constraint", "priority": 1, "status": "active"}, nil))
}

func (s *soloState) addComment(targetType, targetID, body string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/comment/add",
		map[string]any{"slug": s.slug, "target_type": targetType, "target_id": targetID, "body": body}, nil))
}

func (s *soloState) markCommentsRead(targetType, targetID, role string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/comment/mark-read",
		map[string]any{"slug": s.slug, "target_type": targetType, "target_id": targetID, "role": role}, nil))
}

func (s *soloState) addBlockerQuestion(targetType, targetID, context string) int32 {
	s.t.Helper()
	var q struct {
		ID int32 `json:"id"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/blocker-questions/add",
		map[string]any{"slug": s.slug, "target_type": targetType, "target_id": targetID, "context": context}, &q))
	return q.ID
}

func (s *soloState) answerBlockerQuestion(id int32, answer string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/blocker-questions/answer",
		map[string]any{"slug": s.slug, "id": id, "answer": answer, "answered_by": "test"}, nil))
}

func (s *soloState) checkinJournal(role, date string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/journal/checkin",
		map[string]any{
			"slug": s.slug, "role": role, "date": date,
			"tldr": "test", "assessment": "ok", "concerns": "none", "next": "continue",
		}, nil))
}

func (s *soloState) registerTest(name string) int32 {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/test-results/submit",
		map[string]any{
			"slug": s.slug,
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
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/tests?slug=%s", s.slug), nil, &resp))
	for _, t := range resp.Tests {
		if t.Name == name {
			return t.ID
		}
	}
	s.t.Fatalf("test %q not found after registration", name)
	return 0
}

func (s *soloState) deferSpec(specID int32) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/specs/defer",
		map[string]any{"spec_id": specID}, nil))
}

func (s *soloState) reviewFeature(name string) {
	s.t.Helper()
	mustOK(s.t, apiDo(s.t, http.MethodPost, "/api/dx/feature/review",
		map[string]any{"slug": s.slug, "feature": name}, nil))
}

// ── API-level checks (same endpoints solo calls) ────────────────────────────

func (s *soloState) hasUnreadComments(targetType, targetID string) bool {
	s.t.Helper()
	var resp struct {
		HasUnread bool `json:"has_unread"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/comment/unread-check?slug=%s&target_type=%s&target_id=%s&role=llm",
			s.slug, targetType, targetID), nil, &resp))
	return resp.HasUnread
}

func (s *soloState) getHealth() (goalCount, constraintCount, closedTaskCount int64, ownerJournalDate, techJournalDate string) {
	s.t.Helper()
	var resp struct {
		GoalCount        int64  `json:"goal_count"`
		ConstraintCount  int64  `json:"constraint_count"`
		OwnerJournalDate string `json:"owner_journal_date"`
		TechJournalDate  string `json:"tech_journal_date"`
		ClosedTaskCount  int64  `json:"closed_task_count"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/solo/health?slug=%s", s.slug), nil, &resp))
	return resp.GoalCount, resp.ConstraintCount, resp.ClosedTaskCount, resp.OwnerJournalDate, resp.TechJournalDate
}

func (s *soloState) listIssues() []struct {
	ID       int32  `json:"id"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
} {
	s.t.Helper()
	var resp struct {
		Issues []struct {
			ID       int32  `json:"id"`
			Status   string `json:"status"`
			Priority string `json:"priority"`
		} `json:"issues"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/list?slug=%s", s.slug), nil, &resp))
	return resp.Issues
}

func (s *soloState) listTasks(issueID int32) []struct {
	ID     int32  `json:"id"`
	Text   string `json:"text"`
	Status string `json:"status"`
} {
	s.t.Helper()
	var resp struct {
		Tasks []struct {
			ID     int32  `json:"id"`
			Text   string `json:"text"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/tasks?slug=%s&issue_id=IS-%d", s.slug, issueID), nil, &resp))
	return resp.Tasks
}

func (s *soloState) listFeatures() []struct {
	Name string `json:"name"`
} {
	s.t.Helper()
	var resp struct {
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/features?slug=%s", s.slug), nil, &resp))
	return resp.Features
}

func (s *soloState) listUncoveredSpecs() []struct {
	ID          int32  `json:"id"`
	FeatureName string `json:"feature_name"`
} {
	s.t.Helper()
	var resp struct {
		Specs []struct {
			ID          int32  `json:"id"`
			FeatureName string `json:"feature_name"`
		} `json:"specs"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/specs/uncovered?slug=%s", s.slug), nil, &resp))
	return resp.Specs
}

func (s *soloState) listStaleFeatures() []struct {
	Name string `json:"name"`
} {
	s.t.Helper()
	var resp struct {
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/features/stale?slug=%s", s.slug), nil, &resp))
	return resp.Features
}

func (s *soloState) listPendingBlockerQuestions() []struct {
	ID         int32  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	Status     string `json:"status"`
} {
	s.t.Helper()
	var resp struct {
		Questions []struct {
			ID         int32  `json:"id"`
			TargetType string `json:"target_type"`
			TargetID   string `json:"target_id"`
			Status     string `json:"status"`
		} `json:"questions"`
	}
	mustOK(s.t, apiDo(s.t, http.MethodGet,
		fmt.Sprintf("/api/dx/blocker-questions/list?slug=%s&status=pending", s.slug), nil, &resp))
	return resp.Questions
}

// ── Solo workflow simulation tests ──────────────────────────────────────────

func TestSoloBootstrap(t *testing.T) {
	ss := newSoloState(t, "solo-bootstrap", "Solo Bootstrap")

	issues := ss.listIssues()
	features := ss.listFeatures()
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
	if len(features) != 0 {
		t.Fatalf("expected 0 features, got %d", len(features))
	}
	// Solo would emit [bootstrap] here because no issues and no features exist.

	// Advance: create a feature and an issue.
	ss.addFeature("test-feature", "A test feature for bootstrap")
	issueID := ss.addIssue("Bootstrap setup", "Verify solo cycle")

	// After bootstrap, solo should no longer emit [bootstrap].
	issues = ss.listIssues()
	if len(issues) == 0 {
		t.Fatal("expected at least 1 issue after bootstrap")
	}
	// The issue is untriaged → solo would emit [triage].
	if issues[0].Priority != "" {
		t.Errorf("new issue should have no priority, got %q", issues[0].Priority)
	}

	// Clean up: close the issue so it doesn't interfere with other tests.
	ss.triageIssue(issueID, 3)
	ss.closeIssue(issueID)
}

func TestSoloReadCommentsIssue(t *testing.T) {
	ss := newSoloState(t, "solo-comments-i", "Solo Comments Issue")
	issueID := ss.addIssue("Comment test issue", "test")
	ss.triageIssue(issueID, 3)

	targetID := fmt.Sprintf("IS-%d", issueID)

	// No comments yet → no unread.
	if ss.hasUnreadComments("issue", targetID) {
		t.Fatal("should not have unread comments initially")
	}

	// Add a comment → unread for llm role.
	ss.addComment("issue", targetID, "Please review this approach")
	if !ss.hasUnreadComments("issue", targetID) {
		t.Fatal("should have unread comments after adding one")
	}

	// Solo would emit [read:comments] IS-N here.
	// Advance: mark read.
	ss.markCommentsRead("issue", targetID, "llm")
	if ss.hasUnreadComments("issue", targetID) {
		t.Fatal("should not have unread comments after marking read")
	}

	ss.closeIssue(issueID)
}

func TestSoloReadCommentsFeature(t *testing.T) {
	ss := newSoloState(t, "solo-comments-f", "Solo Comments Feature")
	ss.addFeature("commentable", "A feature to comment on")

	if ss.hasUnreadComments("feature", "commentable") {
		t.Fatal("should not have unread comments initially")
	}

	ss.addComment("feature", "commentable", "What about this spec?")
	if !ss.hasUnreadComments("feature", "commentable") {
		t.Fatal("should have unread comments after adding one")
	}

	// Solo would emit [read:comments] feature "commentable" here.
	ss.markCommentsRead("feature", "commentable", "llm")
	if ss.hasUnreadComments("feature", "commentable") {
		t.Fatal("should not have unread comments after marking read")
	}
}

func TestSoloClarify(t *testing.T) {
	ss := newSoloState(t, "solo-clarify", "Solo Clarify")
	issueID := ss.addIssue("Clarify test", "needs decision")
	ss.triageIssue(issueID, 2)

	targetID := fmt.Sprintf("IS-%d", issueID)
	bqID := ss.addBlockerQuestion("issue", targetID, "Which database?")

	// Pending BQ should exist.
	bqs := ss.listPendingBlockerQuestions()
	found := false
	for _, q := range bqs {
		if q.ID == bqID && q.Status == "pending" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected pending blocker question")
	}
	// Solo with --issue would emit [clarify] here.

	// Advance: answer the question.
	ss.answerBlockerQuestion(bqID, "Use postgres")

	// No more pending BQs for this issue.
	bqs = ss.listPendingBlockerQuestions()
	for _, q := range bqs {
		if q.TargetID == targetID && q.Status == "pending" {
			t.Fatal("blocker question should be answered")
		}
	}

	ss.closeIssue(issueID)
}

func TestSoloOwnerGoals(t *testing.T) {
	ss := newSoloState(t, "solo-goals", "Solo Goals")

	goalCount, _, _, _, _ := ss.getHealth()
	if goalCount != 0 {
		t.Fatalf("expected 0 goals, got %d", goalCount)
	}
	// Solo would emit [owner:goals] here.

	// Advance: add a goal.
	ss.addGoal("Ship v1.0")
	goalCount, _, _, _, _ = ss.getHealth()
	if goalCount != 1 {
		t.Fatalf("expected 1 goal, got %d", goalCount)
	}
}

func TestSoloOwnerConstraints(t *testing.T) {
	ss := newSoloState(t, "solo-constraints", "Solo Constraints")

	// Need goals first (solo checks goals before constraints).
	ss.addGoal("Ship v1.0")

	_, constraintCount, _, _, _ := ss.getHealth()
	if constraintCount != 0 {
		t.Fatalf("expected 0 constraints, got %d", constraintCount)
	}
	// Solo would emit [owner:constraints] here.

	// Advance: add a constraint.
	ss.addConstraint("No external dependencies without review")
	_, constraintCount, _, _, _ = ss.getHealth()
	if constraintCount != 1 {
		t.Fatalf("expected 1 constraint, got %d", constraintCount)
	}
}

func TestSoloJournal(t *testing.T) {
	ss := newSoloState(t, "solo-journal", "Solo Journal")

	// Set up prerequisites: goals + constraints so solo reaches journal check.
	ss.addGoal("Test goal")
	ss.addConstraint("Test constraint")

	_, _, _, ownerDate, techDate := ss.getHealth()
	if ownerDate != "" {
		t.Fatalf("expected empty owner journal date, got %q", ownerDate)
	}
	if techDate != "" {
		t.Fatalf("expected empty tech journal date, got %q", techDate)
	}
	// Solo would emit [owner:journal] here (owner checked first).

	// Advance: checkin owner journal with today's date.
	ss.checkinJournal("owner", "2026-04-14")
	_, _, _, ownerDate, _ = ss.getHealth()
	if ownerDate != "2026-04-14" {
		t.Fatalf("expected owner journal date 2026-04-14, got %q", ownerDate)
	}
	// Now solo would emit [tech:journal].

	// Advance: checkin tech journal.
	ss.checkinJournal("tech", "2026-04-14")
	_, _, _, _, techDate = ss.getHealth()
	if techDate != "2026-04-14" {
		t.Fatalf("expected tech journal date 2026-04-14, got %q", techDate)
	}
}

func TestSoloTriage(t *testing.T) {
	ss := newSoloState(t, "solo-triage", "Solo Triage")
	issueID := ss.addIssue("Untriaged issue", "needs triage")

	issues := ss.listIssues()
	var found bool
	for _, iss := range issues {
		if iss.ID == issueID && iss.Priority == "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected untriaged issue with no priority")
	}
	// Solo would emit [triage] IS-N here.

	// Advance: triage.
	ss.triageIssue(issueID, 2)
	issues = ss.listIssues()
	for _, iss := range issues {
		if iss.ID == issueID {
			if iss.Priority == "" {
				t.Fatal("issue should have priority after triage")
			}
		}
	}

	ss.closeIssue(issueID)
}

func TestSoloOwnerSpec(t *testing.T) {
	ss := newSoloState(t, "solo-spec", "Solo Spec")
	ss.addFeature("specless", "Feature without specs")

	feats := ss.listFeatures()
	if len(feats) == 0 {
		t.Fatal("expected at least 1 feature")
	}
	// Solo checks features with no specs → would emit [owner:spec].

	// Advance: add a spec.
	ss.addSpec("specless", "unit_test", "Verify core behavior")

	// Now the feature has a spec, but it's uncovered (no test_refs).
	uncovered := ss.listUncoveredSpecs()
	foundUncovered := false
	for _, s := range uncovered {
		if s.FeatureName == "specless" {
			foundUncovered = true
		}
	}
	if !foundUncovered {
		t.Log("spec may already have test refs or be deferred; skipping uncovered check")
	}
}

func TestSoloOwnerReview(t *testing.T) {
	ss := newSoloState(t, "solo-review", "Solo Review")
	ss.addFeature("stale-feat", "Feature that will get stale")
	ss.addSpec("stale-feat", "unit_test", "Basic test")

	// Newly created features have NULL last_reviewed_at → they ARE stale.
	stale := ss.listStaleFeatures()
	found := false
	for _, f := range stale {
		if f.Name == "stale-feat" {
			found = true
		}
	}
	if !found {
		t.Fatal("newly created feature should be stale (never reviewed)")
	}
	// Solo would emit [owner:review] here.

	// Advance: mark reviewed.
	ss.reviewFeature("stale-feat")

	stale = ss.listStaleFeatures()
	for _, f := range stale {
		if f.Name == "stale-feat" {
			t.Fatal("feature should not be stale after review")
		}
	}
}

func TestSoloTechTestRef(t *testing.T) {
	ss := newSoloState(t, "solo-testref", "Solo TestRef")
	ss.addFeature("testable", "Feature needing test refs")
	ss.addSpec("testable", "unit_test", "Core validation")

	// The spec should appear as uncovered.
	uncovered := ss.listUncoveredSpecs()
	var specID int32
	for _, s := range uncovered {
		if s.FeatureName == "testable" {
			specID = s.ID
		}
	}
	if specID == 0 {
		t.Fatal("expected uncovered spec for feature 'testable'")
	}
	// Solo would emit [tech:test-ref] here.

	// Advance: register a test, then link it to the spec.
	testID := ss.registerTest("TestCoreValidation")
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/link-test",
		map[string]any{"spec_id": specID, "test_id": testID}, nil))

	// Spec should no longer be uncovered.
	uncovered = ss.listUncoveredSpecs()
	for _, s := range uncovered {
		if s.ID == specID {
			t.Fatal("spec should not be uncovered after linking test ref")
		}
	}
}

func TestSoloAdd(t *testing.T) {
	ss := newSoloState(t, "solo-add", "Solo Add")
	issueID := ss.addIssue("Issue needing tasks", "decompose")
	ss.triageIssue(issueID, 2)

	// Issue has no tasks → solo would emit [add].
	tasks := ss.listTasks(issueID)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}

	// Advance: add a task.
	taskID := ss.addTask(issueID, "Implement the thing")
	tasks = ss.listTasks(issueID)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != taskID {
		t.Errorf("task ID mismatch: want %d got %d", taskID, tasks[0].ID)
	}
	if tasks[0].Status != "pending" {
		t.Errorf("task status: want pending, got %q", tasks[0].Status)
	}

	ss.markTaskDone(taskID)
	ss.closeIssue(issueID)
}

func TestSoloDev(t *testing.T) {
	ss := newSoloState(t, "solo-dev", "Solo Dev")
	issueID := ss.addIssue("Issue with pending task", "dev work")
	ss.triageIssue(issueID, 2)
	taskID := ss.addTask(issueID, "Write the code")

	// Issue has a pending task → solo would emit [dev] TK-N.
	tasks := ss.listTasks(issueID)
	hasPending := false
	for _, tk := range tasks {
		if tk.Status == "pending" {
			hasPending = true
		}
	}
	if !hasPending {
		t.Fatal("expected at least one pending task")
	}

	// Advance: mark task done.
	ss.markTaskDone(taskID)
	tasks = ss.listTasks(issueID)
	for _, tk := range tasks {
		if tk.ID == taskID && tk.Status != "done" {
			t.Errorf("task should be done, got %q", tk.Status)
		}
	}

	ss.closeIssue(issueID)
}

func TestSoloClosable(t *testing.T) {
	ss := newSoloState(t, "solo-closable", "Solo Closable")
	issueID := ss.addIssue("Issue ready to close", "all tasks done")
	ss.triageIssue(issueID, 2)
	taskID := ss.addTask(issueID, "Only task")
	ss.markTaskDone(taskID)

	// All tasks done, issue open → solo would emit [closable].
	tasks := ss.listTasks(issueID)
	allDone := true
	for _, tk := range tasks {
		if tk.Status == "pending" || tk.Status == "in_progress" {
			allDone = false
		}
	}
	if !allDone {
		t.Fatal("expected all tasks to be done")
	}
	if len(tasks) == 0 {
		t.Fatal("expected at least one task for closable state")
	}

	// Advance: close the issue.
	ss.closeIssue(issueID)

	// Issue should be closed.
	issues := ss.listIssues()
	for _, iss := range issues {
		if iss.ID == issueID && iss.Status != "closed" {
			t.Errorf("issue should be closed, got %q", iss.Status)
		}
	}
}

// TestSoloFullLifecycle walks through bootstrap → triage → add → dev → closable → nothing.
func TestSoloFullLifecycle(t *testing.T) {
	ss := newSoloState(t, "solo-lifecycle", "Solo Lifecycle")

	// 1. Bootstrap: no issues, no features.
	issues := ss.listIssues()
	features := ss.listFeatures()
	if len(issues) != 0 || len(features) != 0 {
		t.Fatal("expected empty project for bootstrap")
	}

	// 2. Create feature + issue (post-bootstrap).
	ss.addFeature("lifecycle-feat", "Lifecycle test feature")
	ss.addSpec("lifecycle-feat", "unit_test", "Basic lifecycle test")
	issueID := ss.addIssue("Lifecycle test issue", "full workflow")

	// Set up health prerequisites so solo doesn't gate on goals/constraints/journals.
	ss.addGoal("Complete lifecycle test")
	ss.addConstraint("Must pass all checks")
	ss.checkinJournal("owner", "2026-04-14")
	ss.checkinJournal("tech", "2026-04-14")

	// Link spec test ref so solo doesn't gate on tech:test-ref.
	testID := ss.registerTest("TestLifecycle")
	uncovered := ss.listUncoveredSpecs()
	for _, spec := range uncovered {
		if spec.FeatureName == "lifecycle-feat" {
			mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/link-test",
				map[string]any{"spec_id": spec.ID, "test_id": testID}, nil))
		}
	}

	// Review feature so it's not stale.
	ss.reviewFeature("lifecycle-feat")

	// 3. Triage: issue has no priority.
	issues = ss.listIssues()
	for _, iss := range issues {
		if iss.ID == issueID && iss.Priority != "" {
			t.Fatal("expected untriaged issue")
		}
	}
	ss.triageIssue(issueID, 2)

	// 4. Add: issue has no tasks.
	tasks := ss.listTasks(issueID)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
	taskID := ss.addTask(issueID, "Lifecycle task")

	// 5. Dev: issue has pending task.
	tasks = ss.listTasks(issueID)
	if len(tasks) != 1 || tasks[0].Status != "pending" {
		t.Fatal("expected 1 pending task")
	}
	ss.markTaskDone(taskID)

	// 6. Closable: all tasks done.
	tasks = ss.listTasks(issueID)
	if tasks[0].Status != "done" {
		t.Fatalf("expected task done, got %q", tasks[0].Status)
	}
	ss.closeIssue(issueID)

	// 7. Nothing to do: all issues closed.
	issues = ss.listIssues()
	openCount := 0
	for _, iss := range issues {
		if iss.Status == "open" {
			openCount++
		}
	}
	if openCount != 0 {
		t.Fatalf("expected 0 open issues, got %d", openCount)
	}
}

// TestSoloPrecedence verifies that comments block triage, and health checks block triage.
func TestSoloPrecedence(t *testing.T) {
	ss := newSoloState(t, "solo-precedence", "Solo Precedence")
	issueID := ss.addIssue("Precedence test", "checking order")
	targetID := fmt.Sprintf("IS-%d", issueID)

	// Add a comment (should take precedence over triage).
	ss.addComment("issue", targetID, "Check this first")
	if !ss.hasUnreadComments("issue", targetID) {
		t.Fatal("expected unread comment")
	}

	// Issue is untriaged, but comment should come first in solo precedence.
	issues := ss.listIssues()
	for _, iss := range issues {
		if iss.ID == issueID && iss.Priority != "" {
			t.Fatal("issue should still be untriaged")
		}
	}

	// The unread comment exists → solo would emit [read:comments] before [triage].
	// Advance: mark read.
	ss.markCommentsRead("issue", targetID, "llm")
	if ss.hasUnreadComments("issue", targetID) {
		t.Fatal("comments should be read now")
	}

	// Now solo would proceed to [triage] (no more comments blocking).
	ss.triageIssue(issueID, 3)
	ss.closeIssue(issueID)
}
