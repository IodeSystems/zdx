package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TodoItem mirrors the server response shape for solo/claim.
type TodoItem struct {
	ID              int32  `json:"id"`
	Kind            string `json:"kind"`
	Text            string `json:"text"`
	Persona         string `json:"persona"`
	Priority        int32  `json:"priority"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	IssueRef        string `json:"issue_ref"`
	ProjectSlug     string `json:"project_slug"`
	SuggestedAction string `json:"suggested_action"`
	ClaimedBy       string `json:"claimed_by"`
}

func soloClaimNext(t *testing.T, slug, agentID string) (TodoItem, int) {
	t.Helper()
	var item TodoItem
	resp := apiDo(t, http.MethodPost, "/api/dx/solo/claim",
		map[string]any{"slug": slug, "agent_id": agentID, "lease_minutes": 5}, &item)
	return item, resp.StatusCode
}

func soloApply(t *testing.T, slug string, items []SoloQueueItem) {
	t.Helper()
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/apply",
		map[string]any{"slug": slug, "items": items}, nil))
}

// TestTodoTakeDelegatesToServerQueue verifies that dx todo take (solo/claim)
// generates and persists the queue server-side then atomically returns the next item.
// The returned item must carry the requesting agent's ID, confirming server-side ownership.
func TestTodoTakeDelegatesToServerQueue(t *testing.T) {
	d := NewApiDriver(t, "td-take", "Todo Take Delegation")
	Given(d).
		Issue("Take delegation test", "server generates queue on claim").
		Build()

	item, status := soloClaimNext(t, d.Slug, "test-agent")
	if status != http.StatusOK {
		t.Fatalf("solo/claim: want 200, got %d", status)
	}
	if item.Kind == "" {
		t.Error("claimed todo has empty Kind — server did not evaluate queue")
	}
	if item.ClaimedBy != "test-agent" {
		t.Errorf("expected claimed_by=test-agent, got %q", item.ClaimedBy)
	}
	if item.ProjectSlug != d.Slug {
		t.Errorf("expected project_slug=%q, got %q", d.Slug, item.ProjectSlug)
	}
}

// TestTodoTakeClaimedItemNotReclaimable verifies that solo/claim is atomic:
// an item already claimed by one agent is not returned to another until the
// lease expires or is released.
func TestTodoTakeClaimedItemNotReclaimable(t *testing.T) {
	d := NewApiDriver(t, "td-atomic", "Todo Take Atomic")
	Given(d).
		Issue("Atomic claim test", "only one agent should claim at a time").
		Build()

	// Claim everything in the queue until it's exhausted.
	var lastStatus int
	for i := 0; i < 10; i++ {
		_, lastStatus = soloClaimNext(t, d.Slug, fmt.Sprintf("agent-%d", i))
		if lastStatus == http.StatusNotFound {
			break
		}
	}
	if lastStatus != http.StatusNotFound {
		t.Errorf("expected queue to become exhausted (404) after all items claimed, last status=%d", lastStatus)
	}
}

// TestTodoListDelegatesToServer verifies that dx todo list returns server-persisted tasks.
func TestTodoListDelegatesToServer(t *testing.T) {
	d := NewApiDriver(t, "td-list", "Todo List Delegation")
	sc := Given(d).
		TriagedIssue("List delegation", "verify server list", 2).
		Task(0, "task for list test").
		Build()

	issueID := sc.Issues[0]
	tasks := d.ListTasks(issueID)
	if len(tasks) == 0 {
		t.Fatal("expected at least one task from server list")
	}
	found := false
	for _, tk := range tasks {
		if tk.Text == "task for list test" {
			found = true
		}
	}
	if !found {
		t.Error("task created via server API not visible in server list")
	}
}

// TestTodoShowDelegatesToServer verifies that dx todo show reads issue state from the server.
func TestTodoShowDelegatesToServer(t *testing.T) {
	d := NewApiDriver(t, "td-show", "Todo Show Delegation")
	issueID := d.AddIssue("Show delegation issue", "verify server show")

	var resp struct {
		Issue struct {
			ID    int32  `json:"id"`
			Title string `json:"title"`
		} `json:"issue"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/show?slug=%s&id=IS-%d", d.Slug, issueID),
		nil, &resp))
	if resp.Issue.ID != issueID {
		t.Errorf("show returned id=%d, want %d", resp.Issue.ID, issueID)
	}
	if resp.Issue.Title != "Show delegation issue" {
		t.Errorf("show returned title=%q, want %q", resp.Issue.Title, "Show delegation issue")
	}
}

// TestTodoSoloEvaluateDelegatesToServer verifies that dx todo solo --evaluate calls the
// server evaluate endpoint, which generates queue candidates server-side.
func TestTodoSoloEvaluateDelegatesToServer(t *testing.T) {
	d := NewApiDriver(t, "td-eval", "Todo Solo Evaluate")
	sc := Given(d).
		Issue("Untriaged solo evaluate", "server should surface triage").
		Build()

	items := d.EvaluateQueue("")
	if len(items) == 0 {
		t.Fatal("expected at least one item from server evaluate")
	}
	item := requireKind(t, items, "triage")
	if item.TargetType != "issue" {
		t.Errorf("expected target_type=issue, got %q", item.TargetType)
	}

	_ = sc
}

// TestTodoSoloApplyPersistsServerSide verifies that dx todo solo --apply (solo/apply) writes
// queue state to the server so subsequent evaluations reflect the persisted queue.
func TestTodoSoloApplyPersistsServerSide(t *testing.T) {
	d := NewApiDriver(t, "td-apply", "Todo Solo Apply")
	sc := Given(d).
		Issue("Apply persistence test", "queue apply must write server-side").
		Build()

	// Evaluate to get proposed items.
	items := d.EvaluateQueue("")
	if len(items) == 0 {
		t.Fatal("expected items from evaluate before apply")
	}

	// Apply the queue.
	soloApply(t, d.Slug, items)

	// Re-evaluate: unchanged items should now appear in Unchanged (not Added).
	var evalResp struct {
		Added     []SoloQueueItem `json:"added"`
		Unchanged []SoloQueueItem `json:"unchanged"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/evaluate",
		map[string]any{"slug": d.Slug, "issue": ""}, &evalResp))
	if len(evalResp.Added) != 0 {
		t.Errorf("after apply, expected 0 added items, got %d — server state not persisted", len(evalResp.Added))
	}
	if len(evalResp.Unchanged) == 0 {
		t.Error("after apply, expected unchanged items — server state not persisted")
	}

	_ = sc
}

// TestSharedTodoQueueSemantics is the demo for spec 101: given both humans
// (dx todo solo) and agents (dx todo take) consuming the queue, both callers
// operate on the same persisted todo records with no separate agent-only queue.
// The test verifies:
//  1. Human evaluate+apply and agent claim both write/read the same zdx_todos rows.
//  2. An item claimed by an agent is visible to a subsequent human evaluate as
//     Unchanged (still open in DB), not Added (which would mean a separate set).
//  3. Once all items are claimed, subsequent claim calls return 404.
func TestSharedTodoQueueSemantics(t *testing.T) {
	d := NewApiDriver(t, "td-shared", "Shared Todo Queue Semantics")
	Given(d).
		Goal("Test goal").
		Constraint("No breaking changes").
		Issue("Shared queue test issue", "verify shared records").
		Build()

	// Step 1: Human evaluates the queue.
	humanItems := d.EvaluateQueue("")
	if len(humanItems) == 0 {
		t.Fatal("expected at least one item from human evaluate")
	}

	// Step 2: Human applies — persists the queue server-side.
	soloApply(t, d.Slug, humanItems)

	// Step 3: Agent claims the next item.
	claimed, status := soloClaimNext(t, d.Slug, "agent-shared")
	if status != http.StatusOK {
		t.Fatalf("agent claim: want 200, got %d", status)
	}
	if claimed.Kind == "" {
		t.Fatal("claimed todo has empty Kind")
	}

	// The claimed item must correspond to one of the items the human evaluated —
	// same key signals same persisted row, no separate agent-only queue.
	foundInHuman := false
	for _, it := range humanItems {
		if it.TargetType == claimed.TargetType && it.TargetID == claimed.TargetID && it.Kind == claimed.Kind {
			foundInHuman = true
			break
		}
	}
	if !foundInHuman {
		t.Errorf("agent claimed %q (target_type=%s target_id=%s) which was not in the human evaluate set — separate queues detected",
			claimed.Kind, claimed.TargetType, claimed.TargetID)
	}

	// Step 4: Human re-evaluates. The claimed item is still open in the DB
	// (claim does not change status), so it must appear in Unchanged, not Added.
	var reEvalResp struct {
		Added     []SoloQueueItem `json:"added"`
		Unchanged []SoloQueueItem `json:"unchanged"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/solo/evaluate",
		map[string]any{"slug": d.Slug, "issue": ""}, &reEvalResp))

	if len(reEvalResp.Added) != 0 {
		t.Errorf("after agent claim, human re-evaluate shows %d Added items — expected 0 (all rows persisted)",
			len(reEvalResp.Added))
	}

	// Step 5: Exhaust the queue; subsequent claims must return 404.
	for i := 0; i < 20; i++ {
		_, s := soloClaimNext(t, d.Slug, fmt.Sprintf("agent-exhaust-%d", i))
		if s == http.StatusNotFound {
			return
		}
	}
	t.Error("expected queue exhaustion (404) after all items claimed, but never reached it")
}

// TestClaimedTodoItemCarriesRequiredFields verifies spec 102:
// a claimed todo item carries owner tier (persona), target entity reference
// (target_type + target_id), priority, and suggested CLI action.
func TestClaimedTodoItemCarriesRequiredFields(t *testing.T) {
	d := NewApiDriver(t, "td-fields", "Todo Item Fields")
	Given(d).
		Goal("Test goal").
		Constraint("Test constraint").
		Feature("td-fields-feat", "feature for fields test").
		Spec("td-fields-feat", "must", "fields test spec").
		Build()

	item, status := soloClaimNext(t, d.Slug, "test-agent")
	if status != http.StatusOK {
		t.Fatalf("solo/claim: want 200, got %d", status)
	}

	if item.Persona == "" {
		t.Error("claimed todo missing persona (owner tier)")
	}
	if item.TargetType == "" {
		t.Error("claimed todo missing target_type (entity reference)")
	}
	if item.TargetID == "" {
		t.Error("claimed todo missing target_id (entity reference)")
	}
	if item.Priority == 0 {
		t.Error("claimed todo missing priority")
	}
	if item.SuggestedAction == "" {
		t.Errorf("claimed todo missing suggested_action for kind=%q target_type=%q target_id=%q", item.Kind, item.TargetType, item.TargetID)
	}
}
