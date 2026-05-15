package agent

import (
	"errors"
	"testing"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// TestFromAgentClaimBody_NilBody covers the transport-miss path: a nil
// response body collapses to (nil todo, nil scope) so callers can treat it
// as transient idle.
func TestFromAgentClaimBody_NilBody(t *testing.T) {
	todo, scope := fromAgentClaimBody(nil)
	if todo != nil {
		t.Errorf("nil body must return nil todo, got %+v", todo)
	}
	if scope != nil {
		t.Errorf("nil body must return nil scope, got %+v", scope)
	}
}

// TestFromAgentClaimBody_EmptyTodoWithClosedScope covers the IS-1234
// hot-fire shape: server returns a 200 with Id==0 and a populated
// Scope (state=closed) when the scope issue itself is closed. The client
// MUST surface (nil todo, scope) so the supervisor loop exits instead of
// running a no-op session against the empty payload.
func TestFromAgentClaimBody_EmptyTodoWithClosedScope(t *testing.T) {
	body := &dxclient.AgentClaimBody{
		Id: 0,
		Scope: &dxclient.AgentClaimScope{
			IssueId: "IS-42",
			State:   "closed",
		},
	}
	todo, scope := fromAgentClaimBody(body)
	if todo != nil {
		t.Errorf("Id==0 must collapse to nil todo, got %+v", todo)
	}
	if scope == nil {
		t.Fatalf("Scope must be surfaced, got nil")
	}
	if scope.State != "closed" || scope.IssueID != "IS-42" {
		t.Errorf("scope mismatch: %+v", scope)
	}
	if !scope.terminal() {
		t.Errorf("state=closed must be terminal")
	}
}

// TestFromAgentClaimBody_EmptyTodoWithTrackerClosable: scope.state=stalled
// with reason=tracker-closable is the second terminal case (descendants all
// done, only the tracker itself is open). Loop should exit cleanly.
func TestFromAgentClaimBody_EmptyTodoWithTrackerClosable(t *testing.T) {
	reason := "tracker-closable"
	body := &dxclient.AgentClaimBody{
		Id: 0,
		Scope: &dxclient.AgentClaimScope{
			IssueId: "IS-7",
			State:   "stalled",
			Reason:  &reason,
		},
	}
	todo, scope := fromAgentClaimBody(body)
	if todo != nil {
		t.Errorf("Id==0 must collapse to nil todo, got %+v", todo)
	}
	if !scope.terminal() {
		t.Errorf("stalled+tracker-closable must be terminal: %+v", scope)
	}
}

// TestFromAgentClaimBody_EmptyTodoWithAllBlocked: stalled+all-blocked is
// transient — another worker may release a lease or a blocker may close.
// Loop should idle-poll, NOT exit.
func TestFromAgentClaimBody_EmptyTodoWithAllBlocked(t *testing.T) {
	reason := "all-blocked"
	body := &dxclient.AgentClaimBody{
		Id: 0,
		Scope: &dxclient.AgentClaimScope{
			IssueId: "IS-99",
			State:   "stalled",
			Reason:  &reason,
		},
	}
	todo, scope := fromAgentClaimBody(body)
	if todo != nil {
		t.Errorf("Id==0 must collapse to nil todo, got %+v", todo)
	}
	if scope == nil || scope.Reason != "all-blocked" {
		t.Fatalf("scope reason must round-trip: %+v", scope)
	}
	if scope.terminal() {
		t.Errorf("stalled+all-blocked must NOT be terminal: %+v", scope)
	}
}

// TestFromAgentClaimBody_RealClaim: a populated TodoItem (Id != 0) plus
// scope=claimed surfaces a real claimedTodo and the scope info — the loop
// proceeds normally.
func TestFromAgentClaimBody_RealClaim(t *testing.T) {
	branch := "wip/agent-1"
	body := &dxclient.AgentClaimBody{
		Id:           42,
		Text:         "do the thing",
		Key:          "dev-TK-42",
		Kind:         "dev",
		TargetType:   "task",
		TargetId:     "TK-42",
		IssueRef:     "IS-1",
		Priority:     100,
		TargetBranch: &branch,
		Scope: &dxclient.AgentClaimScope{
			IssueId: "IS-1",
			State:   "claimed",
		},
	}
	todo, scope := fromAgentClaimBody(body)
	if todo == nil {
		t.Fatalf("Id=42 must surface a claimedTodo")
	}
	if todo.ID != 42 || todo.Key != "dev-TK-42" || todo.TargetBranch != branch {
		t.Errorf("todo fields not projected: %+v", todo)
	}
	if scope == nil || scope.State != "claimed" {
		t.Errorf("scope must surface claimed state: %+v", scope)
	}
	if scope.terminal() {
		t.Errorf("state=claimed must not be terminal")
	}
}

// TestFromAgentClaimBody_NoScope: a global (claim-any) response carries no
// Scope field. The nil-receiver terminal() guard handles this safely.
func TestFromAgentClaimBody_NoScope(t *testing.T) {
	body := &dxclient.AgentClaimBody{Id: 0}
	todo, scope := fromAgentClaimBody(body)
	if todo != nil {
		t.Errorf("Id==0 must collapse to nil todo, got %+v", todo)
	}
	if scope != nil {
		t.Errorf("absent Scope must surface as nil, got %+v", scope)
	}
	// nil *scopeInfo must be safe to call .terminal() on.
	if scope.terminal() {
		t.Errorf("nil scope must NOT be terminal")
	}
}

// TestScopeExhaustedError_IsErrScopeExhausted: errors.Is must match the
// sentinel so callers can detect the family without type-asserting.
func TestScopeExhaustedError_IsErrScopeExhausted(t *testing.T) {
	err := &ScopeExhaustedError{IssueID: "IS-1", State: "closed"}
	if !errors.Is(err, ErrScopeExhausted) {
		t.Errorf("errors.Is must match ErrScopeExhausted sentinel")
	}
}
