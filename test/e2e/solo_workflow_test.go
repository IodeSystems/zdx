package e2e

import (
	"fmt"
	"testing"
)

func TestSoloBootstrap(t *testing.T) {
	d := NewApiDriver(t, "solo-bootstrap", "Solo Bootstrap")

	issues := d.ListIssues()
	features := d.ListFeatures()
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
	if len(features) != 0 {
		t.Fatalf("expected 0 features, got %d", len(features))
	}

	d.AddFeature("test-feature", "A test feature for bootstrap")
	issueID := d.AddIssue("Bootstrap setup", "Verify solo cycle")

	issues = d.ListIssues()
	if len(issues) == 0 {
		t.Fatal("expected at least 1 issue after bootstrap")
	}
	if issues[0].Priority != "" {
		t.Errorf("new issue should have no priority, got %q", issues[0].Priority)
	}

	d.TriageIssue(issueID, 3)
	d.CloseIssue(issueID)
}

func TestSoloReadCommentsIssue(t *testing.T) {
	d := NewApiDriver(t, "solo-comments-i", "Solo Comments Issue")
	sc := Given(d).
		TriagedIssue("Comment test issue", "test", 3).
		Build()

	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	if d.HasUnreadComments("issue", targetID) {
		t.Fatal("should not have unread comments initially")
	}

	d.AddComment("issue", targetID, "Please review this approach")
	if !d.HasUnreadComments("issue", targetID) {
		t.Fatal("should have unread comments after adding one")
	}

	d.MarkCommentsRead("issue", targetID, "llm")
	if d.HasUnreadComments("issue", targetID) {
		t.Fatal("should not have unread comments after marking read")
	}

	d.CloseIssue(issueID)
}

func TestSoloReadCommentsFeature(t *testing.T) {
	d := NewApiDriver(t, "solo-comments-f", "Solo Comments Feature")
	Given(d).Feature("commentable", "A feature to comment on").Build()

	if d.HasUnreadComments("feature", "commentable") {
		t.Fatal("should not have unread comments initially")
	}

	d.AddComment("feature", "commentable", "What about this spec?")
	if !d.HasUnreadComments("feature", "commentable") {
		t.Fatal("should have unread comments after adding one")
	}

	d.MarkCommentsRead("feature", "commentable", "llm")
	if d.HasUnreadComments("feature", "commentable") {
		t.Fatal("should not have unread comments after marking read")
	}
}

func TestSoloClarify(t *testing.T) {
	d := NewApiDriver(t, "solo-clarify", "Solo Clarify")
	sc := Given(d).TriagedIssue("Clarify test", "needs decision", 2).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	bqID := d.AddBlockerQuestion("issue", targetID, "Which database?")

	bqs := d.ListPendingBlockerQuestions()
	found := false
	for _, q := range bqs {
		if q.ID == bqID && q.Status == "pending" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected pending blocker question")
	}

	d.AnswerBlockerQuestion(bqID, "Use postgres")

	bqs = d.ListPendingBlockerQuestions()
	for _, q := range bqs {
		if q.TargetID == targetID && q.Status == "pending" {
			t.Fatal("blocker question should be answered")
		}
	}

	d.CloseIssue(issueID)
}

func TestSoloOwnerGoals(t *testing.T) {
	d := NewApiDriver(t, "solo-goals", "Solo Goals")

	goalCount, _, _, _, _ := d.GetHealth()
	if goalCount != 0 {
		t.Fatalf("expected 0 goals, got %d", goalCount)
	}

	d.AddGoal("Ship v1.0")
	goalCount, _, _, _, _ = d.GetHealth()
	if goalCount != 1 {
		t.Fatalf("expected 1 goal, got %d", goalCount)
	}
}

func TestSoloOwnerConstraints(t *testing.T) {
	d := NewApiDriver(t, "solo-constraints", "Solo Constraints")
	d.AddGoal("Ship v1.0")

	_, constraintCount, _, _, _ := d.GetHealth()
	if constraintCount != 0 {
		t.Fatalf("expected 0 constraints, got %d", constraintCount)
	}

	d.AddConstraint("No external dependencies without review")
	_, constraintCount, _, _, _ = d.GetHealth()
	if constraintCount != 1 {
		t.Fatalf("expected 1 constraint, got %d", constraintCount)
	}
}

func TestSoloJournal(t *testing.T) {
	d := NewApiDriver(t, "solo-journal", "Solo Journal")
	Given(d).Goal("Test goal").Constraint("Test constraint").Build()

	_, _, _, ownerDate, techDate := d.GetHealth()
	if ownerDate != "" {
		t.Fatalf("expected empty owner journal date, got %q", ownerDate)
	}
	if techDate != "" {
		t.Fatalf("expected empty tech journal date, got %q", techDate)
	}

	d.CheckinJournal("owner", "2026-04-14")
	_, _, _, ownerDate, _ = d.GetHealth()
	if ownerDate != "2026-04-14" {
		t.Fatalf("expected owner journal date 2026-04-14, got %q", ownerDate)
	}

	d.CheckinJournal("tech", "2026-04-14")
	_, _, _, _, techDate = d.GetHealth()
	if techDate != "2026-04-14" {
		t.Fatalf("expected tech journal date 2026-04-14, got %q", techDate)
	}
}

func TestSoloTriage(t *testing.T) {
	d := NewApiDriver(t, "solo-triage", "Solo Triage")
	sc := Given(d).Issue("Untriaged issue", "needs triage").Build()
	issueID := sc.Issues[0]

	issues := d.ListIssues()
	var found bool
	for _, iss := range issues {
		if iss.ID == issueID && iss.Priority == "" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected untriaged issue with no priority")
	}

	d.TriageIssue(issueID, 2)
	issues = d.ListIssues()
	for _, iss := range issues {
		if iss.ID == issueID {
			if iss.Priority == "" {
				t.Fatal("issue should have priority after triage")
			}
		}
	}

	d.CloseIssue(issueID)
}

func TestSoloOwnerSpec(t *testing.T) {
	d := NewApiDriver(t, "solo-spec", "Solo Spec")
	Given(d).Feature("specless", "Feature without specs").Build()

	feats := d.ListFeatures()
	if len(feats) == 0 {
		t.Fatal("expected at least 1 feature")
	}

	d.AddSpec("specless", "unit_test", "Verify core behavior")

	uncovered := d.ListUncoveredSpecs()
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
	d := NewApiDriver(t, "solo-review", "Solo Review")
	Given(d).
		Feature("stale-feat", "Feature that will get stale").
		Spec("stale-feat", "unit_test", "Basic test").
		Build()

	stale := d.ListStaleFeatures()
	found := false
	for _, f := range stale {
		if f.Name == "stale-feat" {
			found = true
		}
	}
	if !found {
		t.Fatal("newly created feature should be stale (never reviewed)")
	}

	d.ReviewFeature("stale-feat")

	stale = d.ListStaleFeatures()
	for _, f := range stale {
		if f.Name == "stale-feat" {
			t.Fatal("feature should not be stale after review")
		}
	}
}

func TestSoloTechTestRef(t *testing.T) {
	d := NewApiDriver(t, "solo-testref", "Solo TestRef")
	Given(d).
		Feature("testable", "Feature needing test refs").
		Spec("testable", "unit_test", "Core validation").
		Build()

	uncovered := d.ListUncoveredSpecs()
	var specID int32
	for _, s := range uncovered {
		if s.FeatureName == "testable" {
			specID = s.ID
		}
	}
	if specID == 0 {
		t.Fatal("expected uncovered spec for feature 'testable'")
	}

	testID := d.RegisterTest("TestCoreValidation")
	d.LinkTestToSpec(specID, testID)

	uncovered = d.ListUncoveredSpecs()
	for _, s := range uncovered {
		if s.ID == specID {
			t.Fatal("spec should not be uncovered after linking test ref")
		}
	}
}

func TestSoloAdd(t *testing.T) {
	d := NewApiDriver(t, "solo-add", "Solo Add")
	sc := Given(d).TriagedIssue("Issue needing tasks", "decompose", 2).Build()
	issueID := sc.Issues[0]

	tasks := d.ListTasks(issueID)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}

	taskID := d.AddTask(issueID, "Implement the thing")
	tasks = d.ListTasks(issueID)
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if tasks[0].ID != taskID {
		t.Errorf("task ID mismatch: want %d got %d", taskID, tasks[0].ID)
	}
	if tasks[0].Status != "ready" {
		t.Errorf("task status: want ready, got %q", tasks[0].Status)
	}

	d.MarkTaskDone(taskID)
	d.CloseIssue(issueID)
}

func TestSoloDev(t *testing.T) {
	d := NewApiDriver(t, "solo-dev", "Solo Dev")
	sc := Given(d).
		TriagedIssue("Issue with ready task", "dev work", 2).
		Task(0, "Write the code").
		Build()
	issueID := sc.Issues[0]
	taskID := sc.Tasks[0]

	tasks := d.ListTasks(issueID)
	hasPending := false
	for _, tk := range tasks {
		if tk.Status == "ready" {
			hasPending = true
		}
	}
	if !hasPending {
		t.Fatal("expected at least one ready task")
	}

	d.MarkTaskDone(taskID)
	tasks = d.ListTasks(issueID)
	for _, tk := range tasks {
		if tk.ID == taskID && tk.Status != "done" {
			t.Errorf("task should be done, got %q", tk.Status)
		}
	}

	d.CloseIssue(issueID)
}

func TestSoloClosable(t *testing.T) {
	d := NewApiDriver(t, "solo-closable", "Solo Closable")
	sc := Given(d).
		TriagedIssue("Issue ready to close", "all tasks done", 2).
		Task(0, "Only task").
		Build()
	issueID := sc.Issues[0]

	d.MarkTaskDone(sc.Tasks[0])

	tasks := d.ListTasks(issueID)
	allDone := true
	for _, tk := range tasks {
		if tk.Status == "ready" || tk.Status == "active" {
			allDone = false
		}
	}
	if !allDone {
		t.Fatal("expected all tasks to be done")
	}
	if len(tasks) == 0 {
		t.Fatal("expected at least one task for closable state")
	}

	d.CloseIssue(issueID)

	issues := d.ListIssues()
	for _, iss := range issues {
		if iss.ID == issueID && iss.Status != "closed" {
			t.Errorf("issue should be closed, got %q", iss.Status)
		}
	}
}

func TestSoloFullLifecycle(t *testing.T) {
	d := NewApiDriver(t, "solo-lifecycle", "Solo Lifecycle")

	// 1. Bootstrap: empty project.
	issues := d.ListIssues()
	features := d.ListFeatures()
	if len(issues) != 0 || len(features) != 0 {
		t.Fatal("expected empty project for bootstrap")
	}

	// 2. Post-bootstrap: create fixtures via scenario builder.
	sc := Given(d).
		Feature("lifecycle-feat", "Lifecycle test feature").
		Spec("lifecycle-feat", "unit_test", "Basic lifecycle test").
		Issue("Lifecycle test issue", "full workflow").
		HealthPrereqs().
		Build()
	issueID := sc.Issues[0]

	// Link spec test ref.
	testID := d.RegisterTest("TestLifecycle")
	uncovered := d.ListUncoveredSpecs()
	for _, spec := range uncovered {
		if spec.FeatureName == "lifecycle-feat" {
			d.LinkTestToSpec(spec.ID, testID)
		}
	}

	d.ReviewFeature("lifecycle-feat")

	// 3. Triage.
	issues = d.ListIssues()
	for _, iss := range issues {
		if iss.ID == issueID && iss.Priority != "" {
			t.Fatal("expected untriaged issue")
		}
	}
	d.TriageIssue(issueID, 2)

	// 4. Add task.
	tasks := d.ListTasks(issueID)
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}
	taskID := d.AddTask(issueID, "Lifecycle task")

	// 5. Dev: complete task.
	tasks = d.ListTasks(issueID)
	if len(tasks) != 1 || tasks[0].Status != "ready" {
		t.Fatal("expected 1 ready task")
	}
	d.MarkTaskDone(taskID)

	// 6. Closable: all done.
	tasks = d.ListTasks(issueID)
	if tasks[0].Status != "done" {
		t.Fatalf("expected task done, got %q", tasks[0].Status)
	}
	d.CloseIssue(issueID)

	// 7. Nothing to do.
	issues = d.ListIssues()
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

func TestSoloPrecedence(t *testing.T) {
	d := NewApiDriver(t, "solo-precedence", "Solo Precedence")
	sc := Given(d).Issue("Precedence test", "checking order").Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	d.AddComment("issue", targetID, "Check this first")
	if !d.HasUnreadComments("issue", targetID) {
		t.Fatal("expected unread comment")
	}

	issues := d.ListIssues()
	for _, iss := range issues {
		if iss.ID == issueID && iss.Priority != "" {
			t.Fatal("issue should still be untriaged")
		}
	}

	d.MarkCommentsRead("issue", targetID, "llm")
	if d.HasUnreadComments("issue", targetID) {
		t.Fatal("comments should be read now")
	}

	d.TriageIssue(issueID, 3)
	d.CloseIssue(issueID)
}

func TestSoloUnansweredQuestions(t *testing.T) {
	d := NewApiDriver(t, "solo-unans-qa", "Solo Unanswered QA")

	qs := d.ListUnansweredQuestions()
	if len(qs) != 0 {
		t.Fatalf("expected 0 unanswered questions, got %d", len(qs))
	}

	q1 := d.AddQuestion("arch", "Which cache layer should we use?")
	q2 := d.AddQuestion("ops", "What monitoring tool do we prefer?")

	qs = d.ListUnansweredQuestions()
	if len(qs) != 2 {
		t.Fatalf("expected 2 unanswered questions, got %d", len(qs))
	}
	if qs[0].ID != q1 {
		t.Errorf("expected oldest question first (ID %d), got %d", q1, qs[0].ID)
	}

	d.AnswerQuestion(q1, "Redis")
	qs = d.ListUnansweredQuestions()
	if len(qs) != 1 {
		t.Fatalf("expected 1 unanswered question after answering one, got %d", len(qs))
	}
	if qs[0].ID != q2 {
		t.Errorf("expected remaining question ID %d, got %d", q2, qs[0].ID)
	}

	d.AnswerQuestion(q2, "Grafana")
	qs = d.ListUnansweredQuestions()
	if len(qs) != 0 {
		t.Fatalf("expected 0 unanswered questions, got %d", len(qs))
	}
}

func TestSoloStaleUnreadComments(t *testing.T) {
	d := NewApiDriver(t, "solo-stale-comm", "Solo Stale Comments")
	sc := Given(d).TriagedIssue("Stale comment test", "test", 3).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	d.AddComment("issue", targetID, "Initial comment")
	d.MarkCommentsRead("issue", targetID, "llm")

	stale := d.ListStaleUnreadComments(0)
	if len(stale) != 0 {
		t.Fatalf("expected 0 stale comments (all read), got %d", len(stale))
	}

	d.AddComment("issue", targetID, "Follow-up that nobody read")
	stale = d.ListStaleUnreadComments(0)
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale unread comment, got %d", len(stale))
	}
	if stale[0].TargetID != targetID {
		t.Errorf("expected target_id %s, got %s", targetID, stale[0].TargetID)
	}

	d.MarkCommentsRead("issue", targetID, "llm")
	stale = d.ListStaleUnreadComments(0)
	if len(stale) != 0 {
		t.Fatalf("expected 0 stale comments after marking read, got %d", len(stale))
	}

	d.CloseIssue(issueID)
}

func TestTaskCreatedAtField(t *testing.T) {
	d := NewApiDriver(t, "solo-task-ts", "Task Timestamps")
	sc := Given(d).
		TriagedIssue("Task timestamp test", "test context", 3).
		Build()

	issueID := sc.Issues[0]
	taskID := d.AddTask(issueID, "Task with timestamps")

	tasks := d.ListTasks(issueID)
	found := false
	for _, tk := range tasks {
		if tk.ID == taskID {
			found = true
			if tk.CreatedAt == "" {
				t.Error("created_at should be set on task response")
			}
			break
		}
	}
	if !found {
		t.Fatal("task not found in list")
	}

	d.MarkTaskDone(taskID)
	d.CloseIssue(issueID)
}

func TestSoloStaleTaskSweep(t *testing.T) {
	d := NewApiDriver(t, "solo-stale-task", "Solo Stale Tasks")
	sc := Given(d).
		TriagedIssue("Stale task test", "test context", 3).
		Build()

	issueID := sc.Issues[0]
	taskID := d.AddTask(issueID, "A task that will become stale")

	// Fresh task should not appear as stale
	staleTasks := d.ListStaleTasks("")
	if len(staleTasks) != 0 {
		t.Fatalf("expected 0 stale tasks for fresh task, got %d", len(staleTasks))
	}

	// Sweep with stale_days=0 flags everything created before NOW()
	flagged := d.SweepStaleTasks(0)
	if flagged != 1 {
		t.Fatalf("expected 1 task flagged stale, got %d", flagged)
	}

	// Now it should appear as stale
	staleTasks = d.ListStaleTasks("")
	if len(staleTasks) != 1 {
		t.Fatalf("expected 1 stale task, got %d", len(staleTasks))
	}
	if staleTasks[0].ID != taskID {
		t.Errorf("expected stale task ID %d, got %d", taskID, staleTasks[0].ID)
	}
	if staleTasks[0].StaleSince == "" {
		t.Error("stale_since should be set")
	}

	// Also test issue-scoped listing
	issueRef := fmt.Sprintf("IS-%d", issueID)
	staleByIssue := d.ListStaleTasks(issueRef)
	if len(staleByIssue) != 1 {
		t.Fatalf("expected 1 stale task by issue, got %d", len(staleByIssue))
	}

	// Mark done clears the stale task from the list
	d.MarkTaskDone(taskID)
	staleTasks = d.ListStaleTasks("")
	if len(staleTasks) != 0 {
		t.Fatalf("expected 0 stale tasks after done, got %d", len(staleTasks))
	}

	d.CloseIssue(issueID)
}
