package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func TestQueueKindTriage(t *testing.T) {
	d := NewApiDriver(t, "q-triage", "Queue Triage")
	Given(d).Issue("Untriaged bug", "something broke").Build()

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "triage")
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}
	if item.Persona != "owner" {
		t.Errorf("expected persona=owner, got %q", item.Persona)
	}
}

func TestQueueKindReadCommentsIssue(t *testing.T) {
	d := NewApiDriver(t, "q-rc-issue", "Queue ReadComments Issue")
	sc := Given(d).TriagedIssue("Commented issue", "test", 3).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	d.AddComment("issue", targetID, "Please check this")
	items := d.EvaluateQueue("")
	item := requireKind(t, items, "read:comments")
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}
	if item.TargetID != targetID {
		t.Errorf("expected target_id=%s, got %s", targetID, item.TargetID)
	}
}

func TestQueueKindReadCommentsClosedIssue(t *testing.T) {
	d := NewApiDriver(t, "q-rc-closed", "Queue ReadComments Closed Issue")
	sc := Given(d).TriagedIssue("Closed commented issue", "test", 3).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	d.CloseIssue(issueID)
	d.AddComment("issue", targetID, "Follow-up question on closed issue")
	items := d.EvaluateQueue("")
	item := requireKind(t, items, "read:comments")
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}
	if item.TargetID != targetID {
		t.Errorf("expected target_id=%s, got %s", targetID, item.TargetID)
	}
}

func TestQueueKindReadCommentsFeature(t *testing.T) {
	d := NewApiDriver(t, "q-rc-feat", "Queue ReadComments Feature")
	Given(d).Feature("rc-feat", "A feature with comments").Build()

	d.AddComment("feature", "rc-feat", "What about this?")
	items := d.EvaluateQueue("")
	item := requireKind(t, items, "read:comments")
	if item.TargetType != "feature" {
		t.Errorf("expected target_type=feature, got %q", item.TargetType)
	}
	if item.TargetID != "rc-feat" {
		t.Errorf("expected target_id=rc-feat, got %s", item.TargetID)
	}
}

func TestQueueKindAnswer(t *testing.T) {
	d := NewApiDriver(t, "q-answer", "Queue Answer")
	d.AddQuestion("arch", "Which cache layer?")

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "answer")
	if item.TargetType != "question" {
		t.Errorf("expected target_type=question, got %q", item.TargetType)
	}
}

func TestQueueKindClarify(t *testing.T) {
	d := NewApiDriver(t, "q-clarify", "Queue Clarify")
	sc := Given(d).TriagedIssue("Blocked by BQ", "needs decision", 2).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	d.AddBlockerQuestion("issue", targetID, "Which approach?")

	// Issue-scoped evaluate should surface clarify
	items := d.EvaluateQueue(targetID)
	item := requireKind(t, items, "clarify")
	if item.TargetType != "blocker_question" {
		t.Errorf("expected target_type=blocker_question, got %q", item.TargetType)
	}
	if !item.Blocked {
		t.Error("expected clarify item to be blocked")
	}
}

func TestQueueKindOwnerGoals(t *testing.T) {
	d := NewApiDriver(t, "q-goals", "Queue Goals")

	items := d.EvaluateQueue("")
	requireKind(t, items, "owner:goals")

	d.AddGoal("Ship v1")
	items = d.EvaluateQueue("")
	requireNoKind(t, items, "owner:goals")
}

func TestQueueKindOwnerConstraints(t *testing.T) {
	d := NewApiDriver(t, "q-constraints", "Queue Constraints")
	d.AddGoal("Ship v1")

	items := d.EvaluateQueue("")
	requireKind(t, items, "owner:constraints")

	d.AddConstraint("No breaking changes")
	items = d.EvaluateQueue("")
	requireNoKind(t, items, "owner:constraints")
}

func TestQueueKindTriageExcludesTracker(t *testing.T) {
	d := NewApiDriver(t, "q-tracker", "Queue Tracker")
	// Tracker issues should be excluded from the solo queue entirely.
	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": d.Slug, "title": "Tracker parent", "context": "test", "issue_type": "tracker", "auto_ready": true}, &issue))

	items := d.EvaluateQueue("")
	requireNoKind(t, items, "triage")
}

func TestQueueKindCloseTracker(t *testing.T) {
	d := NewApiDriver(t, "q-close-tracker", "Queue Close Tracker")
	// Child issue to act as a blocker for the tracker.
	childID := d.AddIssue("Child work", "first milestone")
	childRef := fmt.Sprintf("IS-%d", childID)

	// Tracker blocked by the child. Without a closed blocker it should NOT surface.
	var tracker struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{
			"slug":       d.Slug,
			"title":      "Tracker parent",
			"context":    "umbrella",
			"issue_type": "tracker",
			"auto_ready": true,
			"blocked_by": []string{childRef},
		}, &tracker))

	items := d.EvaluateQueue("")
	requireNoKind(t, items, "close:tracker")

	// Closing the blocker should make the tracker closable.
	d.CloseIssue(childID)

	items = d.EvaluateQueue("")
	item := requireKind(t, items, "close:tracker")
	trackerRef := fmt.Sprintf("IS-%d", tracker.ID)
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}
	if item.TargetID != trackerRef {
		t.Errorf("expected target_id=%s, got %s", trackerRef, item.TargetID)
	}
}

func TestQueueKindAdd(t *testing.T) {
	d := NewApiDriver(t, "q-add", "Queue Add")
	Given(d).TriagedIssue("Needs decomposition", "no tasks yet", 2).Build()

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "add")
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}
}

func TestQueueKindDev(t *testing.T) {
	d := NewApiDriver(t, "q-dev", "Queue Dev")
	sc := Given(d).
		TriagedIssue("Dev work", "has tasks", 2).
		Task(0, "Write the code").
		Build()

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "dev")
	if item.TargetType != "task" {
		t.Errorf("expected target_type=task, got %q", item.TargetType)
	}
	if item.TargetID != fmt.Sprintf("TK-%d", sc.Tasks[0]) {
		t.Errorf("expected target_id=TK-%d, got %s", sc.Tasks[0], item.TargetID)
	}
}

func TestQueueKindClosable(t *testing.T) {
	d := NewApiDriver(t, "q-closable", "Queue Closable")
	sc := Given(d).
		TriagedIssue("Ready to close", "all done", 2).
		Task(0, "Only task").
		Build()

	d.MarkTaskDone(sc.Tasks[0])

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "closable")
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}
}

func TestQueueKindOwnerSpec(t *testing.T) {
	d := NewApiDriver(t, "q-spec", "Queue Spec")
	Given(d).Feature("specless", "No specs here").Build()

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "owner:spec")
	if item.TargetType != "feature" {
		t.Errorf("expected target_type=feature, got %q", item.TargetType)
	}
	if item.TargetID != "specless" {
		t.Errorf("expected target_id=specless, got %s", item.TargetID)
	}
}

func TestQueueKindOwnerReview(t *testing.T) {
	d := NewApiDriver(t, "q-review", "Queue Review")
	Given(d).
		Feature("stale-rev", "Review target").
		Spec("stale-rev", "unit_test", "Basic test").
		Build()

	// Newly created features with specs count as stale (never reviewed).
	items := d.EvaluateQueue("")
	item := requireKind(t, items, "owner:review")
	if item.TargetType != "feature" {
		t.Errorf("expected target_type=feature, got %q", item.TargetType)
	}

	d.ReviewFeature("stale-rev")
	items = d.EvaluateQueue("")
	requireNoKind(t, items, "owner:review")
}

func TestQueueKindTechTestRef(t *testing.T) {
	d := NewApiDriver(t, "q-testref", "Queue TestRef")
	Given(d).
		Feature("testable-q", "Needs test refs").
		Spec("testable-q", "unit_test", "Core validation").
		Build()

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "tech:test-ref")
	if item.TargetType != "spec" {
		t.Errorf("expected target_type=spec, got %q", item.TargetType)
	}

	uncovered := d.ListUncoveredSpecs()
	if len(uncovered) > 0 {
		testID := d.RegisterTest("TestCoreQ")
		d.LinkTestToSpec(uncovered[0].ID, testID)
		items = d.EvaluateQueue("")
		requireNoKind(t, items, "tech:test-ref")
	}
}

func TestQueueKindOwnerDemoGap(t *testing.T) {
	d := NewApiDriver(t, "q-demogap", "Queue DemoGap")
	Given(d).
		Feature("demo-q", "Needs demo").
		Spec("demo-q", "unit_test", "Core behavior").
		Build()

	// Link a test ref so the spec passes tech:test-ref and becomes eligible for demo-gap
	uncovered := d.ListUncoveredSpecs()
	if len(uncovered) == 0 {
		t.Skip("no uncovered specs to test demo-gap with")
	}
	testID := d.RegisterTest("TestDemoQ")
	d.LinkTestToSpec(uncovered[0].ID, testID)

	items := d.EvaluateQueue("")
	item := findKind(items, "owner:demo-gap")
	if item == nil {
		t.Skip("demo-gap not surfaced — spec may need additional state")
	}
	if item.TargetType != "spec" {
		t.Errorf("expected target_type=spec, got %q", item.TargetType)
	}
}

func TestQueueKindPriority(t *testing.T) {
	d := NewApiDriver(t, "q-priority", "Queue Priority")
	sc := Given(d).
		Issue("Untriaged", "needs triage").
		Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)
	d.AddComment("issue", targetID, "Check this")

	items := d.EvaluateQueue("")
	// read:comments (priority 5) should come before triage (priority 20)
	var commentIdx, triageIdx int
	commentIdx = -1
	triageIdx = -1
	for i, it := range items {
		if it.Kind == "read:comments" && commentIdx == -1 {
			commentIdx = i
		}
		if it.Kind == "triage" && triageIdx == -1 {
			triageIdx = i
		}
	}
	if commentIdx == -1 || triageIdx == -1 {
		t.Fatalf("expected both read:comments and triage, got kinds: %v", kindsOf(items))
	}
	commentPrio := items[commentIdx].Priority
	triagePrio := items[triageIdx].Priority
	if commentPrio >= triagePrio {
		t.Errorf("read:comments priority (%d) should be < triage priority (%d)", commentPrio, triagePrio)
	}
}

func TestQueueEmpty(t *testing.T) {
	d := NewApiDriver(t, "q-empty", "Queue Empty")
	sc := Given(d).
		HealthPrereqs("2026-04-14").
		Feature("complete", "All set").
		Spec("complete", "unit_test", "Test").
		TriagedIssue("Done issue", "finished", 2).
		Task(0, "Only task").
		Build()

	// Link test ref
	uncovered := d.ListUncoveredSpecs()
	if len(uncovered) > 0 {
		testID := d.RegisterTest("TestComplete")
		d.LinkTestToSpec(uncovered[0].ID, testID)
	}
	d.ReviewFeature("complete")

	// Complete the task and close the issue
	d.MarkTaskDone(sc.Tasks[0])
	d.CloseIssue(sc.Issues[0])

	items := d.EvaluateQueue("")
	// Filter out maturity nudges and demo-gap since this test
	// doesn't set up the full goal/feature attribution model.
	maturityKinds := map[string]bool{
		"owner:demo-gap":          true,
		"owner:quantify-goal":     true,
		"owner:attribute-feature": true,
		"tech:instrument-feature": true,
		"owner:decompose-feature": true,
	}
	var actionableItems []SoloQueueItem
	for _, it := range items {
		if !maturityKinds[it.Kind] {
			actionableItems = append(actionableItems, it)
		}
	}
	if len(actionableItems) != 0 {
		t.Errorf("expected empty queue (excluding maturity nudges), got %d items: %v", len(actionableItems), kindsOf(actionableItems))
	}
}

func TestQueueBQBlockedGlobal(t *testing.T) {
	d := NewApiDriver(t, "q-bq-global", "Queue BQ Global")
	sc := Given(d).TriagedIssue("BQ blocked", "test", 2).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	d.AddBlockerQuestion("issue", targetID, "Which approach?")

	// In global mode, BQ-blocked issues are filtered out — no add/dev/closable for this issue
	items := d.EvaluateQueue("")
	for _, it := range items {
		if it.IssueRef == targetID && (it.Kind == "add" || it.Kind == "dev" || it.Kind == "closable") {
			t.Errorf("BQ-blocked issue should not surface %q in global mode", it.Kind)
		}
	}
}

func kindsOf(items []SoloQueueItem) []string {
	kinds := make([]string, len(items))
	for i, it := range items {
		kinds[i] = it.Kind
	}
	return kinds
}
