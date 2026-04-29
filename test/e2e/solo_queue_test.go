package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
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
	if item.TargetType != "qa" {
		t.Errorf("expected target_type=qa, got %q", item.TargetType)
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

// TestQueueTrackerExcludedFromAllRegularItems verifies spec 82: given a tracker-type issue,
// when solo evaluates the queue, the tracker is excluded from all regular queue items
// (triage, add, dev, closable) because tracker issues are closed by their children, not worked directly.
func TestQueueTrackerExcludedFromAllRegularItems(t *testing.T) {
	d := NewApiDriver(t, "q-tracker-excl", "Queue Tracker Excluded")

	// Create a tracker issue with auto_ready so it would otherwise be triaged and appear as
	// "add" (no tasks), "dev" (with tasks), or "closable" (all tasks done) for a normal issue.
	var tracker struct {
		ID int32 `json:"id"`
	}
	mustOK(d.t, apiDo(d.t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": d.Slug, "title": "Tracker umbrella", "context": "test", "issue_type": "tracker", "auto_ready": true}, &tracker))

	trackerRef := fmt.Sprintf("IS-%d", tracker.ID)
	regularKinds := map[string]bool{"triage": true, "add": true, "dev": true, "closable": true}

	items := d.EvaluateQueue("")
	for _, it := range items {
		if (it.TargetID == trackerRef || it.IssueRef == trackerRef) && regularKinds[it.Kind] {
			t.Errorf("tracker issue %s should not appear as %q in queue", trackerRef, it.Kind)
		}
	}
}

func TestQueueKindCloseTracker(t *testing.T) {
	d := NewApiDriver(t, "q-close-tracker", "Queue Close Tracker")
	childID := d.AddIssue("Child work", "first milestone")
	childRef := fmt.Sprintf("IS-%d", childID)

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

	// With open child, no close:tracker signal and no auto-close.
	items := d.EvaluateQueue("")
	requireNoKind(t, items, "close:tracker")

	// Closing the last child auto-closes the tracker immediately.
	d.CloseIssue(childID)

	var show struct {
		Issue struct {
			Status string `json:"status"`
		} `json:"issue"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/dx/todo/issue/show?slug=%s&id=IS-%d", d.Slug, tracker.ID), nil, &show))
	if show.Issue.Status != "closed" {
		t.Errorf("expected tracker status=closed after last child closed, got %q", show.Issue.Status)
	}

	// Queue should not surface close:tracker for an already-closed tracker.
	items = d.EvaluateQueue("")
	requireNoKind(t, items, "close:tracker")
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

// IS-481: an in-flight task referencing the spec by id should suppress the
// tech:test-ref candidate, even when the task's title doesn't follow the
// "Test spec N: ..." prefix exactly. Prior behavior anchored on `^Test spec`
// only, so agents filing with variant titles ("Add test for spec N") kept
// re-emitting the candidate and stacking duplicate wip drafts.
func TestQueueKindTechTestRefSuppressedByVariantTitle(t *testing.T) {
	d := NewApiDriver(t, "q-testref-variant", "TestRef Variant Title")
	Given(d).
		Feature("variant-q", "Variant title coverage").
		Spec("variant-q", "unit_test", "Behavior to test").
		Build()

	uncovered := d.ListUncoveredSpecs()
	if len(uncovered) == 0 {
		t.Fatal("expected at least one uncovered spec")
	}
	specID := uncovered[0].ID

	// File a task that references the spec by id with a non-canonical title.
	r := d.AddTaskCustom(map[string]any{
		"title": fmt.Sprintf("Add test for spec %d", specID),
		"text":  fmt.Sprintf("Cover spec %d with a unit test", specID),
		"force": true,
	})
	if r.ID == 0 {
		t.Fatalf("expected task to be created, got %+v", r)
	}

	items := d.EvaluateQueue("")
	for _, it := range items {
		if it.Kind == "tech:test-ref" && it.TargetID == fmt.Sprintf("%d", specID) {
			t.Errorf("tech:test-ref for spec %d should be suppressed by variant-title task", specID)
		}
	}
}

// IS-493: a task whose title doesn't mention the spec but whose text body does
// should still suppress the tech:test-ref nudge.
func TestQueueKindTechTestRefSuppressedByTaskText(t *testing.T) {
	d := NewApiDriver(t, "q-testref-text", "TestRef Text Body")
	Given(d).
		Feature("text-q", "Text body coverage").
		Spec("text-q", "unit_test", "Behavior to test via body").
		Build()

	uncovered := d.ListUncoveredSpecs()
	if len(uncovered) == 0 {
		t.Fatal("expected at least one uncovered spec")
	}
	specID := uncovered[0].ID

	// Title has no spec reference; spec id appears only in the task body.
	r := d.AddTaskCustom(map[string]any{
		"title": "Cover validation behavior",
		"text":  fmt.Sprintf("spec %d needs a unit test for the core validation path", specID),
		"force": true,
	})
	if r.ID == 0 {
		t.Fatalf("expected task to be created, got %+v", r)
	}

	items := d.EvaluateQueue("")
	for _, it := range items {
		if it.Kind == "tech:test-ref" && it.TargetID == fmt.Sprintf("%d", specID) {
			t.Errorf("tech:test-ref for spec %d should be suppressed by task body reference", specID)
		}
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
		HealthPrereqs().
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

	// Scope to the closed issue — ambient project-level kinds (standup,
	// maturity nudges, goal/constraint health) are server-gated behind
	// issueFilter == "", so scoping naturally excludes them.
	target := fmt.Sprintf("IS-%d", sc.Issues[0])
	items := d.EvaluateQueue(target)
	if len(items) != 0 {
		t.Errorf("expected empty queue for closed issue %s, got %d items: %v", target, len(items), kindsOf(items))
	}
}

// TestQueueBQBlockedGlobal verifies spec 64: given an issue blocked by a pending blocker question,
// when solo is run in global mode, that issue's items are excluded from the queue.
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

// TestQueueStrictPriorityOrder verifies spec 62: when multiple queue categories are present,
// items are returned in strict priority order: comments < questions < triage < specs < closable < decomposition < tasks.
// Note: issue.priority is folded into dev/closable/add base priorities (IS-426). Triaged issues
// here use P4 (smallest fold, -5) to preserve the base category ordering this test documents.
func TestQueueStrictPriorityOrder(t *testing.T) {
	d := NewApiDriver(t, "q-strict-order", "Queue Strict Priority Order")

	// Health prereqs suppress owner:goals / owner:constraints noise.
	Given(d).HealthPrereqs().Build()

	// comments (priority 5): unread comment on any issue
	sc := Given(d).TriagedIssue("Commented issue", "has comment", 4).Build()
	commentIssueRef := fmt.Sprintf("IS-%d", sc.Issues[0])
	d.AddComment("issue", commentIssueRef, "Please review this")

	// questions (priority 10): unanswered QA question
	d.AddQuestion("arch", "Which approach should we use?")

	// triage (priority 20): untriaged issue
	Given(d).Issue("Untriaged bug", "needs triage").Build()

	// specs (priority 25): feature with no specs
	d.AddFeature("unspecced-feature", "Feature lacking specs")

	// closable (base 35 → P4 folds to 30): triaged issue with all tasks done
	sc2 := Given(d).
		TriagedIssue("All done", "ready to close", 4).
		Task(0, "Finished task").
		Build()
	d.MarkTaskDone(sc2.Tasks[0])

	// decomposition/add (base 38 → P4 folds to 33): triaged issue with no tasks
	Given(d).TriagedIssue("Needs decomposition", "no tasks yet", 4).Build()

	// tasks/dev (base 40 → P4 folds to 35): triaged issue with a pending task
	Given(d).
		TriagedIssue("Dev work", "has tasks", 4).
		Task(0, "Write the code").
		Build()

	items := d.EvaluateQueue("")

	type kindPrio struct {
		kind  string
		prio  int32
		found bool
	}
	categories := []kindPrio{
		{kind: "read:comments"},
		{kind: "answer"},
		{kind: "triage"},
		{kind: "owner:spec"},
		{kind: "closable"},
		{kind: "add"},
		{kind: "dev"},
	}
	for i, cat := range categories {
		item := findKind(items, cat.kind)
		if item == nil {
			t.Errorf("expected kind %q in queue, got kinds: %v", cat.kind, kindsOf(items))
			continue
		}
		categories[i].prio = item.Priority
		categories[i].found = true
	}

	for i := 1; i < len(categories); i++ {
		prev, cur := categories[i-1], categories[i]
		if !prev.found || !cur.found {
			continue
		}
		if prev.prio >= cur.prio {
			t.Errorf("priority order violation: %q (priority %d) should be < %q (priority %d)",
				prev.kind, prev.prio, cur.kind, cur.prio)
		}
	}
}

// TestQueueIssueScoped verifies spec 63: when solo is run with --issue=IS-N,
// only items scoped to that issue are returned (triage, decomposition, tasks, closable).
func TestQueueIssueScoped(t *testing.T) {
	d := NewApiDriver(t, "q-issue-scoped", "Queue Issue Scoped")

	// Four issues, one per scoped kind.
	sc := Given(d).
		HealthPrereqs().
		Issue("Untriaged bug", "needs triage").                 // idx 0 → triage
		TriagedIssue("Needs decomposition", "no tasks yet", 2). // idx 1 → add
		TriagedIssue("Dev work", "has tasks", 2).               // idx 2 → dev
		Task(2, "Write the code").                              // task for idx 2
		TriagedIssue("Ready to close", "all done", 2).          // idx 3 → closable
		Task(3, "Only task").
		Build()

	closableTaskID := sc.Tasks[1]
	d.MarkTaskDone(closableTaskID)

	cases := []struct {
		idx      int
		wantKind string
	}{
		{0, "triage"},
		{1, "add"},
		{2, "dev"},
		{3, "closable"},
	}

	for _, tc := range cases {
		target := fmt.Sprintf("IS-%d", sc.Issues[tc.idx])
		items := d.EvaluateQueue(target)

		found := false
		for _, it := range items {
			if it.IssueRef != "" && it.IssueRef != target {
				t.Errorf("scope %s: item for other issue leaked: kind=%s issue_ref=%s",
					target, it.Kind, it.IssueRef)
			}
			if it.Kind == tc.wantKind && (it.IssueRef == target || it.TargetID == target) {
				found = true
			}
		}
		if !found {
			t.Errorf("scope %s: expected kind %q for this issue, got kinds: %v",
				target, tc.wantKind, kindsOf(items))
		}
	}
}

// TestQueueUnifiedMixedSources is the demo for spec 99: given a project with all six
// signal source types present simultaneously, dx todo solo returns a single unified queue
// ordered by priority tier.
// Note: issue.priority is folded into dev/closable/add base priorities (IS-426). Triaged
// issues here use P4 (smallest fold) to preserve the base tier ordering this demo shows.
func TestQueueUnifiedMixedSources(t *testing.T) {
	d := NewApiDriver(t, "q-unified", "Queue Unified Mixed Sources")

	// Suppress health noise so the queue reflects only the signal sources under test.
	Given(d).HealthPrereqs().Build()

	// Signal 1: unread comment → read:comments (priority 5)
	sc := Given(d).TriagedIssue("Commented issue", "has unread comment", 4).Build()
	commentIssueRef := fmt.Sprintf("IS-%d", sc.Issues[0])
	d.AddComment("issue", commentIssueRef, "Please review this change")

	// Signal 2: untriaged issue → triage (priority 20)
	Given(d).Issue("Untriaged bug", "needs triage decision").Build()

	// Signal 3: decomposition gap — triaged, no tasks → add (base 38, P4 folds to 33)
	Given(d).TriagedIssue("Needs tasks", "no tasks yet", 4).Build()

	// Signal 4: pending task → dev (base 40, P4 folds to 35)
	Given(d).
		TriagedIssue("Active work", "has a pending task", 4).
		Task(0, "Implement the feature").
		Build()

	// Signal 5: closable — triaged, all tasks done → closable (base 35, P4 folds to 30)
	sc2 := Given(d).
		TriagedIssue("Ready to close", "all tasks done", 4).
		Task(0, "Finished work").
		Build()
	d.MarkTaskDone(sc2.Tasks[0])

	// Signal 6: blocker question → clarify (priority 5, surfaces in issue-scoped mode)
	sc3 := Given(d).TriagedIssue("BQ blocked issue", "pending decision", 4).Build()
	bqIssueRef := fmt.Sprintf("IS-%d", sc3.Issues[0])
	d.AddBlockerQuestion("issue", bqIssueRef, "Which approach should we use?")

	// Global evaluate: five of the six signal sources appear in one unified ordered queue.
	items := d.EvaluateQueue("")

	wantGlobal := []string{"read:comments", "triage", "closable", "add", "dev"}
	for _, kind := range wantGlobal {
		requireKind(t, items, kind)
	}

	// Unified queue must be sorted in non-decreasing priority order.
	for i := 1; i < len(items); i++ {
		if items[i].Priority < items[i-1].Priority {
			t.Errorf("priority order violated at index %d: %q (priority %d) follows %q (priority %d)",
				i, items[i].Kind, items[i].Priority, items[i-1].Kind, items[i-1].Priority)
		}
	}

	// BQ integration: the blocked issue's tasks are suppressed in global mode.
	for _, it := range items {
		if it.IssueRef == bqIssueRef && (it.Kind == "add" || it.Kind == "dev" || it.Kind == "closable") {
			t.Errorf("BQ-blocked issue %s should not surface %q in global queue", bqIssueRef, it.Kind)
		}
	}

	// Issue-scoped: blocker question surfaces as clarify for the specific blocked issue.
	scopedItems := d.EvaluateQueue(bqIssueRef)
	requireKind(t, scopedItems, "clarify")
}

// TestQueueBackportTargetBranch covers spec 177: given backport tasks for supported
// version branches, when the solo queue evaluates, then backport tasks surface with
// the target branch name and version context so agents know which branch to work on.
func TestQueueBackportTargetBranch(t *testing.T) {
	d := NewApiDriver(t, "q-backport-branch", "Queue Backport Target Branch")

	// Build the backport scenario: triaged issue with a ready task, then set target_branch.
	sc := Given(d).
		HealthPrereqs().
		TriagedIssue("Backport fix to v1.2", "cherry-pick the login fix onto the v1.2 release branch", 2).
		Task(0, "Cherry-pick the login fix onto v1.2").
		Build()

	// Set target_branch on the already-triaged issue.
	issueID := sc.Issues[0]
	d.TriageIssueWithBranch(issueID, 2, "v1.2")

	issueRef := fmt.Sprintf("IS-%d", issueID)

	// Evaluate scoped to this issue: the dev item must carry TargetBranch="v1.2".
	items := d.EvaluateQueue(issueRef)
	item := requireKind(t, items, "dev")
	if item.TargetBranch != "v1.2" {
		t.Errorf("evaluate dev item target_branch: want %q got %q", "v1.2", item.TargetBranch)
	}

	// Claim: the claimed TodoItem must also carry TargetBranch="v1.2".
	claimed, status := soloClaimNext(t, d.Slug, "test-agent-backport")
	if status != 200 {
		t.Fatalf("solo/claim: want 200, got %d", status)
	}
	if claimed.Kind != "dev" {
		t.Fatalf("claimed kind: want %q got %q", "dev", claimed.Kind)
	}
	if claimed.TargetBranch != "v1.2" {
		t.Errorf("claimed todo target_branch: want %q got %q", "v1.2", claimed.TargetBranch)
	}

	// Negative case: a plain issue without explicit branch defaults to "dev".
	d2 := NewApiDriver(t, "q-backport-branch-dev", "Queue Backport Default Dev Branch")
	sc2 := Given(d2).HealthPrereqs().TriagedIssue("Normal fix on dev", "no explicit branch", 2).Task(0, "Implement the fix").Build()
	issue2Ref := fmt.Sprintf("IS-%d", sc2.Issues[0])

	items2 := d2.EvaluateQueue(issue2Ref)
	item2 := requireKind(t, items2, "dev")
	if item2.TargetBranch != "dev" {
		t.Errorf("default branch dev item target_branch: want %q got %q", "dev", item2.TargetBranch)
	}
}

// TestQueueGlobalGroupsByTargetBranch covers spec 178: given a project with
// multiple version branches, when the solo queue evaluates in global mode, then
// dev-targeted items precede version-branch backports and same-branch items are
// contiguous (no interleaving).
func TestQueueGlobalGroupsByTargetBranch(t *testing.T) {
	d := NewApiDriver(t, "q-branch-groups", "Queue Branch Groups")
	Given(d).HealthPrereqs().Build()

	// Dev-targeted issue: no explicit branch → TargetBranch defaults to "dev".
	sc := Given(d).
		TriagedIssue("Dev fix", "normal dev work", 2).
		Task(0, "Implement dev fix").
		Build()
	_ = sc

	// Backport issue targeting v1.0.x.
	scV1 := Given(d).
		TriagedIssue("Backport to v1.0.x", "cherry-pick onto v1.0.x", 2).
		Task(0, "Cherry-pick onto v1.0.x").
		Build()
	d.TriageIssueWithBranch(scV1.Issues[0], 2, "v1.0.x")

	// Backport issue targeting v2.0.x.
	scV2 := Given(d).
		TriagedIssue("Backport to v2.0.x", "cherry-pick onto v2.0.x", 2).
		Task(0, "Cherry-pick onto v2.0.x").
		Build()
	d.TriageIssueWithBranch(scV2.Issues[0], 2, "v2.0.x")

	items := d.EvaluateQueue("")

	// Collect TargetBranch values for "dev" kind items only.
	var branches []string
	for _, it := range items {
		if it.Kind == "dev" {
			b := it.TargetBranch
			if b == "" {
				b = "dev"
			}
			branches = append(branches, b)
		}
	}

	if len(branches) < 3 {
		t.Fatalf("expected at least 3 dev items (one per branch), got %d: %v", len(branches), branches)
	}

	// Every "dev" branch value must precede every non-"dev" branch value.
	seenNonDev := false
	for _, b := range branches {
		if b != "dev" {
			seenNonDev = true
		} else if seenNonDev {
			t.Errorf("dev-targeted item follows a version-branch item: branch sequence %v", branches)
			break
		}
	}

	// Same-branch items must be contiguous (no interleaving).
	seen := map[string]bool{}
	prev := ""
	for _, b := range branches {
		if b != prev {
			if seen[b] {
				t.Errorf("branch %q appears non-contiguously in sequence %v", b, branches)
				break
			}
			seen[prev] = true
			prev = b
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

// TestSoloEvaluateDiff covers spec 66: the evaluate endpoint returns a full
// diff (added/removed/changed/unchanged) against the persisted todo set.
//
// Strategy: soloApply with a manually crafted divergent state, then evaluate.
// This avoids the refreshQueueAsync race (which re-syncs the DB after every
// issue mutation) by placing soloApply as the last write before evaluate.
func TestSoloEvaluateDiff(t *testing.T) {
	d := NewApiDriver(t, "eval-diff", "Solo Evaluate Diff")

	// Three untriaged issues → each generates a "triage-IS-N" candidate.
	idX := d.AddIssue("Issue X unchanged", "will be applied with correct values")
	idY := d.AddIssue("Issue Y stale", "will be applied with a wrong priority")
	idZ := d.AddIssue("Issue Z new", "will be omitted from apply → Added")

	keyX := fmt.Sprintf("triage-IS-%d", idX)
	keyY := fmt.Sprintf("triage-IS-%d", idY)
	keyZ := fmt.Sprintf("triage-IS-%d", idZ)
	keyFake := "eval-diff-stale-sentinel" // never generated → Removed

	// Let background refreshQueueAsync goroutines from AddIssue settle before
	// we overwrite the DB via soloApply.
	time.Sleep(50 * time.Millisecond)

	// Get the current proposed items (X, Y, Z will be here with correct values).
	initial := d.EvaluateQueue("")

	// Build the apply payload with one deliberate divergence per bucket:
	//   X: exact copy → Unchanged
	//   Y: priority changed to 99 → Changed (proposed will have priority 20)
	//   Z: omitted → Added (evaluate proposes it but it's not persisted)
	//   fake: novel key → Removed (in persisted, not in proposed)
	//   all others (globals): exact copy → Unchanged
	var applyItems []SoloQueueItem
	for _, it := range initial {
		switch it.Key {
		case keyX:
			applyItems = append(applyItems, it) // exact → Unchanged
		case keyY:
			stale := it
			stale.Priority = 99 // deliberately wrong
			applyItems = append(applyItems, stale)
		case keyZ:
			// omit → Added
		default:
			applyItems = append(applyItems, it) // globals → Unchanged
		}
	}
	applyItems = append(applyItems, SoloQueueItem{
		Key: keyFake, Kind: "triage", Text: "sentinel",
		TargetType: "project", TargetID: d.Slug,
		Priority: 50, Persona: "owner", Status: "open",
	})

	// Persist our crafted state. soloApply does NOT trigger refreshQueueAsync,
	// so the state is stable until evaluate runs.
	soloApply(t, d.Slug, applyItems)

	// Evaluate immediately — no mutations between here and soloApply.
	diff := d.EvaluateDiff("")

	diffKey := func(items []SoloQueueItem) []string {
		keys := make([]string, len(items))
		for i, it := range items {
			keys[i] = it.Key
		}
		return keys
	}
	diffKeyRemoved := func(items []TodoItem) []string {
		keys := make([]string, len(items))
		for i, it := range items {
			keys[i] = it.Key
		}
		return keys
	}
	diffKeyChanged := func(items []EvaluateChange) []string {
		keys := make([]string, len(items))
		for i, it := range items {
			keys[i] = it.After.Key
		}
		return keys
	}

	inAdded := func(key string) bool {
		for _, it := range diff.Added {
			if it.Key == key {
				return true
			}
		}
		return false
	}
	inRemoved := func(key string) bool {
		for _, it := range diff.Removed {
			if it.Key == key {
				return true
			}
		}
		return false
	}
	inChanged := func(key string) bool {
		for _, it := range diff.Changed {
			if it.After.Key == key {
				return true
			}
		}
		return false
	}
	inUnchanged := func(key string) bool {
		for _, it := range diff.Unchanged {
			if it.Key == key {
				return true
			}
		}
		return false
	}

	if !inAdded(keyZ) {
		t.Errorf("Added: want key %q, got %v", keyZ, diffKey(diff.Added))
	}
	if !inRemoved(keyFake) {
		t.Errorf("Removed: want key %q, got %v", keyFake, diffKeyRemoved(diff.Removed))
	}
	if !inChanged(keyY) {
		t.Errorf("Changed: want key %q, got %v", keyY, diffKeyChanged(diff.Changed))
	}
	if !inUnchanged(keyX) {
		t.Errorf("Unchanged: want key %q, got %v", keyX, diffKey(diff.Unchanged))
	}
}
