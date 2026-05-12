package e2e

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

// scopedClaimResponse mirrors the agentClaimBody shape extended for scoped
// claims. Fields are pointers/optional where appropriate so we can
// distinguish "field unset by server" from zero values.
type scopedClaimResponse struct {
	TodoItem
	Scope *struct {
		IssueID      string   `json:"issue_id"`
		State        string   `json:"state"`
		Reason       string   `json:"reason,omitempty"`
		OpenBlockers []string `json:"open_blockers,omitempty"`
	} `json:"scope,omitempty"`
}

func agentClaimScoped(t *testing.T, slug, agentID, scopeIssue string) (scopedClaimResponse, int) {
	t.Helper()
	var item scopedClaimResponse
	resp := apiDo(t, http.MethodPost, "/api/dx/agent/claim",
		map[string]any{
			"slug":           slug,
			"agent_id":       agentID,
			"lease_minutes":  5,
			"scope_issue_id": scopeIssue,
		}, nil)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusOK && len(body) > 0 {
		if err := json.Unmarshal(body, &item); err != nil {
			t.Fatalf("decode scoped claim: %v body=%s", err, body)
		}
	}
	return item, resp.StatusCode
}

// addTracker creates a tracker issue and returns its IS-N ref.
func addTracker(t *testing.T, slug, title string) string {
	t.Helper()
	var resp struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{
			"slug":       slug,
			"title":      title,
			"context":    "tracker for scope test",
			"issue_type": "tracker",
			"auto_ready": true,
		}, &resp))
	return fmt.Sprintf("IS-%d", resp.ID)
}

// addCompositionChild creates an issue and wires it as a composition child
// of parentRef. Returns the new issue's id.
func addCompositionChild(t *testing.T, slug, parentRef, title string) int32 {
	t.Helper()
	var add struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": title, "context": "scope child", "auto_ready": true}, &add))
	parentNum := mustParseIssue(t, parentRef)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add-block",
		map[string]any{
			"slug":       slug,
			"id":         parentNum,
			"blocked_by": fmt.Sprintf("IS-%d", add.ID),
			"kind":       "composition",
		}, nil))
	return add.ID
}

func mustParseIssue(t *testing.T, ref string) int32 {
	t.Helper()
	var n int32
	if _, err := fmt.Sscanf(ref, "IS-%d", &n); err != nil {
		t.Fatalf("parse %q: %v", ref, err)
	}
	return n
}

// TestAgentClaimScopeHappyPath: a tracker with two composition children, each
// with a dev task. A scoped claim returns one of the children's tasks; the
// second scoped claim returns the other; the third stalls.
func TestAgentClaimScopeHappyPath(t *testing.T) {
	d := NewApiDriver(t, "scope-happy", "Scope Happy Path")

	parent := addTracker(t, d.Slug, "Tracker")
	childA := addCompositionChild(t, d.Slug, parent, "Child A")
	childB := addCompositionChild(t, d.Slug, parent, "Child B")
	d.TriageIssue(childA, 2)
	d.TriageIssue(childB, 2)
	d.AddTask(childA, "task in A")
	d.AddTask(childB, "task in B")

	// Out-of-scope sibling: a third issue with a task that must NOT be claimed
	// under the scope.
	outsideID := d.AddIssue("Outside scope", "unrelated work")
	d.TriageIssue(outsideID, 2)
	d.AddTask(outsideID, "task outside scope")

	first, status := agentClaimScoped(t, d.Slug, "scope-agent-1", parent)
	if status != http.StatusOK {
		t.Fatalf("first scoped claim: status %d", status)
	}
	if first.Scope == nil || first.Scope.State != "claimed" {
		t.Fatalf("first claim: want scope.state=claimed, got %+v", first.Scope)
	}
	childARef := fmt.Sprintf("IS-%d", childA)
	childBRef := fmt.Sprintf("IS-%d", childB)
	if first.IssueRef != childARef && first.IssueRef != childBRef {
		t.Errorf("first claim issue_ref %q must be one of the scoped children", first.IssueRef)
	}

	second, status := agentClaimScoped(t, d.Slug, "scope-agent-2", parent)
	if status != http.StatusOK {
		t.Fatalf("second scoped claim: status %d", status)
	}
	if second.Scope == nil || second.Scope.State != "claimed" {
		t.Fatalf("second claim: want scope.state=claimed, got %+v", second.Scope)
	}
	if second.IssueRef == first.IssueRef {
		t.Errorf("second claim re-issued same scoped item %q", second.IssueRef)
	}

	// Third claim must stall (both scoped tasks are now reserved with live
	// leases) and must not leak the out-of-scope task.
	third, status := agentClaimScoped(t, d.Slug, "scope-agent-3", parent)
	if status != http.StatusOK {
		t.Fatalf("third scoped claim: status %d", status)
	}
	if third.Scope == nil || third.Scope.State != "stalled" {
		t.Fatalf("third claim: want scope.state=stalled, got %+v", third.Scope)
	}
	if third.Scope.Reason != "all-blocked" {
		t.Errorf("third claim reason: want %q got %q", "all-blocked", third.Scope.Reason)
	}
	if third.ID != 0 {
		t.Errorf("stalled claim must not return a TodoItem (got id=%d)", third.ID)
	}
	wantBlockers := map[string]bool{childARef: true, childBRef: true}
	for _, b := range third.Scope.OpenBlockers {
		if !wantBlockers[b] {
			t.Errorf("unexpected open_blocker %q (must be one of the scoped children)", b)
		}
	}
	if len(third.Scope.OpenBlockers) != 2 {
		t.Errorf("open_blockers: want both children listed, got %v", third.Scope.OpenBlockers)
	}
}

// TestAgentClaimScopeEmptyTrackerHasDecomposeWork: an empty tracker has a
// `tech:decompose-tracker` candidate, so a scoped claim returns it as work
// to do — the loop is NOT stalled. This confirms tracker scope sees todos
// whose issue_ref is the tracker itself (descendant set always includes the
// seed).
func TestAgentClaimScopeEmptyTrackerHasDecomposeWork(t *testing.T) {
	d := NewApiDriver(t, "scope-empty", "Scope Empty Tracker")

	emptyTracker := addTracker(t, d.Slug, "Empty Tracker")
	got, status := agentClaimScoped(t, d.Slug, "scope-empty-agent", emptyTracker)
	if status != http.StatusOK {
		t.Fatalf("empty tracker scoped claim: status %d", status)
	}
	if got.Scope == nil || got.Scope.State != "claimed" {
		t.Fatalf("empty tracker: want claimed (decompose-tracker), got %+v", got.Scope)
	}
	if got.IssueRef != emptyTracker {
		t.Errorf("decompose-tracker issue_ref: want %q got %q", emptyTracker, got.IssueRef)
	}
	if got.Kind != "tech:decompose-tracker" {
		t.Errorf("kind: want %q got %q", "tech:decompose-tracker", got.Kind)
	}
}

// TestAgentClaimScopeClosedIssue: a scope issue that is already closed
// reports state=closed; the agent loop reads this as "exit".
func TestAgentClaimScopeClosedIssue(t *testing.T) {
	d := NewApiDriver(t, "scope-closed", "Scope Closed")

	issueID := d.AddIssue("Standalone closed", "will be closed before claim")
	d.TriageIssue(issueID, 2)
	taskID := d.AddTask(issueID, "single task")
	d.MarkTaskDone(taskID)
	d.CloseIssue(issueID)

	got, status := agentClaimScoped(t, d.Slug, "scope-closed-agent", fmt.Sprintf("IS-%d", issueID))
	if status != http.StatusOK {
		t.Fatalf("closed-scope claim: status %d", status)
	}
	if got.Scope == nil || got.Scope.State != "closed" {
		t.Fatalf("closed scope: want state=closed, got %+v", got.Scope)
	}
}

// TestAgentClaimScopeMidLoopDecomposition: a scoped claim sees descendants
// that were added AFTER the first claim. The descendant walk runs every call,
// so newly-decomposed children become claimable on the next request.
func TestAgentClaimScopeMidLoopDecomposition(t *testing.T) {
	d := NewApiDriver(t, "scope-decomp", "Scope Mid-Loop Decomposition")

	parent := addTracker(t, d.Slug, "Tracker")
	childA := addCompositionChild(t, d.Slug, parent, "Child A (early)")
	d.TriageIssue(childA, 2)
	d.AddTask(childA, "task in A")

	first, status := agentClaimScoped(t, d.Slug, "decomp-agent-1", parent)
	if status != http.StatusOK || first.Scope == nil || first.Scope.State != "claimed" {
		t.Fatalf("first claim: status=%d scope=%+v", status, first.Scope)
	}

	// Tree decomposed mid-loop: new child + task appears.
	childB := addCompositionChild(t, d.Slug, parent, "Child B (late)")
	d.TriageIssue(childB, 2)
	d.AddTask(childB, "task in B")

	second, status := agentClaimScoped(t, d.Slug, "decomp-agent-2", parent)
	if status != http.StatusOK {
		t.Fatalf("second claim: status %d", status)
	}
	if second.Scope == nil || second.Scope.State != "claimed" {
		t.Fatalf("second claim: want claimed (post-decomposition), got %+v", second.Scope)
	}
	if second.IssueRef != fmt.Sprintf("IS-%d", childB) {
		t.Errorf("second claim should pick up Child B's new task; got issue_ref=%q", second.IssueRef)
	}
}

// TestAgentClaimScopeUnscopedUnchanged: omitting scope_issue_id preserves
// the existing global-claim behavior — the scope field must NOT appear in
// the response.
func TestAgentClaimScopeUnscopedUnchanged(t *testing.T) {
	d := NewApiDriver(t, "scope-unscoped", "Scope Unscoped Unchanged")

	Given(d).TriagedIssue("Plain dev work", "no scope", 2).Task(0, "do it").Build()

	first, status := agentClaimNext(t, d.Slug, "plain-agent")
	if status != http.StatusOK {
		t.Fatalf("unscoped claim: status %d", status)
	}
	if first.ID == 0 {
		t.Errorf("unscoped claim should return a TodoItem")
	}
}
