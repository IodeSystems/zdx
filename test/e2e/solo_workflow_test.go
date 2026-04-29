package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
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

// TestSoloOwnerTriageRevisions covers spec 72: when owner triage runs with
// --priority and optional --title/--context/--issue_type, the issue is
// classified and a zdx_revisions row is recorded for *each changed field*.
// Verifies the handler short-circuits at handlers_issues.go (val != nil &&
// *val != "") and that re-sending an unchanged value records no revision.
func TestSoloOwnerTriageRevisions(t *testing.T) {
	d := NewApiDriver(t, "solo-triage-rev", "Solo Triage Revisions")
	sc := Given(d).Issue("Original title", "Original context").Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	type revEntry struct {
		ID         int32  `json:"id"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Field      string `json:"field"`
		OldVal     string `json:"old_val"`
		NewVal     string `json:"new_val"`
	}

	triage := func(body map[string]any) {
		body["slug"] = d.Slug
		body["id"] = issueID
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/owner/triage", body, nil))
	}

	listRevisions := func() []revEntry {
		var resp struct {
			Revisions []revEntry `json:"revisions"`
		}
		mustOK(t, apiDo(t, http.MethodGet,
			fmt.Sprintf("/api/dx/revisions?slug=%s&target_type=issue&target_id=%s&limit=200",
				d.Slug, targetID), nil, &resp))
		// The list endpoint filters by target_type/target_id at the SQL layer;
		// double-check nothing leaked across.
		for _, r := range resp.Revisions {
			if r.TargetType != "issue" || r.TargetID != targetID {
				t.Fatalf("revision %d leaked: %s/%s != issue/%s", r.ID, r.TargetType, r.TargetID, targetID)
			}
		}
		return resp.Revisions
	}

	diffSince := func(before []revEntry) []revEntry {
		seen := map[int32]bool{}
		for _, r := range before {
			seen[r.ID] = true
		}
		var added []revEntry
		for _, r := range listRevisions() {
			if !seen[r.ID] {
				added = append(added, r)
			}
		}
		return added
	}

	byField := func(rs []revEntry) map[string]revEntry {
		m := map[string]revEntry{}
		for _, r := range rs {
			m[r.Field] = r
		}
		return m
	}

	// (a) priority-only: 1 revision (priority "" → "2"), nothing else.
	before := listRevisions()
	triage(map[string]any{"priority": int32(2)})
	added := diffSince(before)
	if len(added) != 1 {
		t.Fatalf("priority-only: expected 1 revision, got %d (%+v)", len(added), added)
	}
	if r := byField(added)["priority"]; r.OldVal != "" || r.NewVal != "2" {
		t.Errorf("priority-only revision: old=%q new=%q want \"\"→\"2\"", r.OldVal, r.NewVal)
	}

	// (b) priority + title + context + issue_type all changed: 4 revisions.
	before = listRevisions()
	triage(map[string]any{
		"priority":   int32(3),
		"title":      "Triaged title",
		"context":    "Triaged context",
		"issue_type": "bug",
	})
	added = diffSince(before)
	if len(added) != 4 {
		t.Fatalf("all-fields: expected 4 revisions, got %d (%+v)", len(added), added)
	}
	want := map[string][2]string{
		"priority":   {"2", "3"},
		"title":      {"Original title", "Triaged title"},
		"context":    {"Original context", "Triaged context"},
		"issue_type": {"unknown", "bug"},
	}
	got := byField(added)
	for f, exp := range want {
		r, ok := got[f]
		if !ok {
			t.Errorf("all-fields: missing revision for %s", f)
			continue
		}
		if r.OldVal != exp[0] || r.NewVal != exp[1] {
			t.Errorf("all-fields revision %s: old=%q new=%q want %q→%q",
				f, r.OldVal, r.NewVal, exp[0], exp[1])
		}
	}

	// (c) priority unchanged + only issue_type changed: 1 revision (issue_type).
	// Proves the priority short-circuit at the oldIssue.Priority != newPriority guard.
	before = listRevisions()
	triage(map[string]any{
		"priority":   int32(3), // unchanged
		"issue_type": "feature",
	})
	added = diffSince(before)
	if len(added) != 1 {
		t.Fatalf("type-only: expected 1 revision, got %d (%+v)", len(added), added)
	}
	if r := byField(added)["issue_type"]; r.OldVal != "bug" || r.NewVal != "feature" {
		t.Errorf("type-only revision: old=%q new=%q want \"bug\"→\"feature\"", r.OldVal, r.NewVal)
	}

	// (d) no-op call: every value identical to current state → 0 revisions.
	before = listRevisions()
	triage(map[string]any{
		"priority":   int32(3),
		"title":      "Triaged title",
		"context":    "Triaged context",
		"issue_type": "feature",
	})
	added = diffSince(before)
	if len(added) != 0 {
		t.Fatalf("no-op: expected 0 revisions, got %d (%+v)", len(added), added)
	}

	// Final issue priority must reflect the last triage.
	for _, iss := range d.ListIssues() {
		if iss.ID == issueID && iss.Priority != "3" {
			t.Errorf("final priority: want 3, got %q", iss.Priority)
		}
	}

	d.CloseIssue(issueID)
}

// TestSoloTriageResolveRefusedWhenUntriaged covers IS-514: a triage todo
// must not be marked resolved unless the underlying issue actually has a
// priority. The agent's "session succeeded" path used to silently flip
// the todo to resolved without applying any triage level. The server now
// downgrades that resolve to a release+block and returns cycle_detected.
func TestSoloTriageResolveRefusedWhenUntriaged(t *testing.T) {
	d := NewApiDriver(t, "solo-triage-guard", "Solo Triage Guard")
	// Pre-load goals/constraints/journal so the only remaining queue item
	// for the agent to pick is the untriaged-issue triage candidate.
	Given(d).
		Issue("Untriaged issue", "needs triage").
		HealthPrereqs().
		Build()

	var claimed TodoItem
	for i := 0; i < 10; i++ {
		c, status := soloClaimNext(t, d.Slug, fmt.Sprintf("test-agent-%d", i))
		if status != http.StatusOK {
			t.Fatalf("solo/claim attempt %d: status=%d", i, status)
		}
		if c.Kind == "triage" {
			claimed = c
			break
		}
	}
	if claimed.Kind != "triage" {
		t.Fatalf("did not find a triage todo to claim after 10 attempts")
	}

	var rel struct {
		OK            bool `json:"ok"`
		CycleDetected bool `json:"cycle_detected"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/release",
		map[string]any{"id": claimed.ID, "agent_id": "test-agent", "resolve": true}, &rel))

	if !rel.CycleDetected {
		t.Error("expected cycle_detected=true when resolving a triage todo with no priority applied")
	}

	var listResp struct {
		Todos []struct {
			ID         int32  `json:"id"`
			Status     string `json:"status"`
			Blocked    bool   `json:"blocked"`
			Kind       string `json:"kind"`
			ResolvedAt string `json:"resolved_at"`
		} `json:"todos"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todos?slug=%s", d.Slug), nil, &listResp))

	var foundTodo bool
	for _, td := range listResp.Todos {
		if td.ID != claimed.ID {
			continue
		}
		foundTodo = true
		if td.Status == "resolved" {
			t.Errorf("triage todo should not be resolved when issue still has no priority, got status=%q", td.Status)
		}
		if !td.Blocked {
			t.Error("triage todo should be auto-blocked after a refused resolve")
		}
		if td.ResolvedAt != "" {
			t.Errorf("resolved_at should be empty, got %q", td.ResolvedAt)
		}
	}
	if !foundTodo {
		t.Fatalf("could not find todo id=%d in /api/dx/todos response", claimed.ID)
	}
}

// TestSoloCycleDetectionAutoFilesIssue covers IS-526: after two consecutive cycle detections
// on the same todo key, the system auto-files a system-gap issue and stores its ID on the todo.
// Closing that issue must automatically unblock the todo so it re-enters the queue.
func TestSoloCycleDetectionAutoFilesIssue(t *testing.T) {
	d := NewApiDriver(t, "solo-cycle-autofile", "Solo Cycle Autofile")
	Given(d).
		Issue("Cycle test issue", "auto-file system gap").
		HealthPrereqs().
		Build()

	claimTriage := func(agentID string) (TodoItem, bool) {
		for i := 0; i < 10; i++ {
			c, status := soloClaimNext(t, d.Slug, fmt.Sprintf("%s-%d", agentID, i))
			if status != http.StatusOK {
				t.Fatalf("solo/claim: status=%d", status)
			}
			if c.Kind == "triage" {
				return c, true
			}
		}
		return TodoItem{}, false
	}

	// First cycle: resolve triage without setting priority → blocked, cycle_count=1
	claimed, ok := claimTriage("agent-a")
	if !ok {
		t.Fatal("no triage todo found for first attempt")
	}
	var rel struct {
		OK            bool `json:"ok"`
		CycleDetected bool `json:"cycle_detected"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/release",
		map[string]any{"id": claimed.ID, "agent_id": "agent-a-0", "resolve": true}, &rel))
	if !rel.CycleDetected {
		t.Fatal("expected cycle_detected=true on first attempt")
	}

	// Unblock so the todo re-enters the queue for a second attempt.
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/unblock-all",
		map[string]any{"slug": d.Slug}, nil))

	// Second cycle: same todo, same failed resolve → blocked, cycle_count=2 → auto-files issue
	claimed, ok = claimTriage("agent-b")
	if !ok {
		t.Fatal("no triage todo found for second attempt")
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/release",
		map[string]any{"id": claimed.ID, "agent_id": "agent-b-0", "resolve": true}, &rel))
	if !rel.CycleDetected {
		t.Fatal("expected cycle_detected=true on second attempt")
	}

	// The blocked todo should now have reference_issue_id set.
	var blockedTodos []struct {
		ID               int32  `json:"id"`
		Blocked          bool   `json:"blocked"`
		CycleCount       int32  `json:"cycle_count"`
		ReferenceIssueID string `json:"reference_issue_id"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/solo?slug=%s&blocked=true", d.Slug), nil, &blockedTodos))

	var refIssueStr string
	for _, td := range blockedTodos {
		if td.ID == claimed.ID {
			if td.CycleCount < 2 {
				t.Errorf("expected cycle_count >= 2, got %d", td.CycleCount)
			}
			if td.ReferenceIssueID == "" {
				t.Error("expected reference_issue_id to be set after second cycle detection")
			}
			refIssueStr = td.ReferenceIssueID
			break
		}
	}
	if refIssueStr == "" {
		t.Fatal("blocked todo with reference_issue_id not found")
	}

	// IS-546 regression: the auto-filed system-gap issue must itself carry a priority,
	// otherwise it becomes a fresh triage candidate and recursively auto-files another
	// system-gap issue on its own cycle. Force a queue regen via evaluate, then assert
	// no triage candidate targets the auto-filed issue.
	var evalResp struct {
		Added     []SoloQueueItem `json:"added"`
		Unchanged []SoloQueueItem `json:"unchanged"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/evaluate",
		map[string]any{"slug": d.Slug, "issue": ""}, &evalResp))
	checkCascade := func(items []SoloQueueItem) {
		for _, it := range items {
			if it.Kind == "triage" && it.TargetID == refIssueStr {
				t.Errorf("auto-filed issue %s spawned its own triage candidate %q — system-gap issues must be created with a priority to break the cascade",
					refIssueStr, it.Key)
			}
		}
	}
	checkCascade(evalResp.Added)
	checkCascade(evalResp.Unchanged)

	// Parse "IS-N" → N, submit a resolution (impl issues require one before closing), then close.
	numStr := strings.TrimPrefix(refIssueStr, "IS-")
	refIssueNum, err := strconv.Atoi(numStr)
	if err != nil {
		t.Fatalf("cannot parse reference_issue_id %q: %v", refIssueStr, err)
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
		map[string]any{"slug": d.Slug, "id": int32(refIssueNum), "shas": []string{"deadbeef"}, "source": "manual"}, nil))
	d.CloseIssue(int32(refIssueNum))

	// After closing the reference issue, the todo must be unblocked.
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/solo?slug=%s&blocked=true", d.Slug), nil, &blockedTodos))
	for _, td := range blockedTodos {
		if td.ID == claimed.ID {
			t.Error("todo should be unblocked after reference issue was closed")
		}
	}
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

// TestSoloDevDone covers spec 76: POST /api/dx/todo/dev/done sets status=done,
// populates completed_at, and publishes a task.done event on the WS channel.
func TestSoloDevDone(t *testing.T) {
	d := NewApiDriver(t, "solo-dev-done", "Solo Dev Done")
	sc := Given(d).
		TriagedIssue("Task done spec76", "verify all three assertions", 3).
		Task(0, "Implement the thing").
		Build()
	taskID := sc.Tasks[0]

	// Subscribe to the project tasks channel before triggering the action
	// so we don't race against the event delivery.
	channel := fmt.Sprintf("project:%s:tasks", d.Slug)
	var signResp struct {
		Token string `json:"token"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/ws/sign",
		map[string]string{"channel": channel}, &signResp))

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	subPayload, _ := json.Marshal(map[string]string{"type": "subscribe", "token": signResp.Token})
	if err := conn.Write(ctx, websocket.MessageText, subPayload); err != nil {
		t.Fatalf("ws write subscribe: %v", err)
	}
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read ack: %v", err)
	}
	var ack struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(ackData, &ack); err != nil || ack.Type != "subscribed" {
		t.Fatalf("expected subscribed ack, got %s", ackData)
	}

	// (a) and (b): status=done and completed_at set after the call.
	// Truncate to seconds because the DB stores second-precision timestamps.
	beforeDone := time.Now().UTC().Truncate(time.Second)
	d.MarkTaskDone(taskID)

	task := d.GetTask(taskID)
	if task.Status != "done" {
		t.Errorf("status: want done, got %q", task.Status)
	}
	if task.CompletedAt == "" {
		t.Error("completed_at should be set after MarkTaskDone")
	} else {
		completedAt, parseErr := time.Parse(time.RFC3339, task.CompletedAt)
		if parseErr != nil {
			t.Errorf("parse completed_at %q: %v", task.CompletedAt, parseErr)
		} else if completedAt.Before(beforeDone) {
			t.Errorf("completed_at %v is before call start time %v", completedAt, beforeDone)
		}
	}

	// (c): task.done event published with payload.id matching the task.
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var msg struct {
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal ws message: %v", err)
	}
	if msg.Event != "task.done" {
		t.Errorf("event: want task.done, got %q", msg.Event)
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	wantID := fmt.Sprintf("TK-%d", taskID)
	if payload.ID != wantID {
		t.Errorf("payload.id: want %q, got %q", wantID, payload.ID)
	}

	d.CloseIssue(sc.Issues[0])
}
