package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestQueueKindRespondDiscussion verifies IS-500: an active discussion whose
// tail message is from the user surfaces a respond:discussion candidate, and
// the candidate disappears once an assistant reply is recorded.
func TestQueueKindRespondDiscussion(t *testing.T) {
	d := NewApiDriver(t, "q-discuss", "Queue Discussion")

	var ds struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/discussions",
		map[string]any{"slug": d.Slug, "title": "Awaiting reply"}, &ds))

	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/discussions/%d/messages", ds.ID),
		map[string]any{"slug": d.Slug, "role": "user", "content": "ping"}, nil))

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "respond:discussion")
	if item.TargetType != "discussion" {
		t.Errorf("expected target_type=discussion, got %q", item.TargetType)
	}
	wantTarget := fmt.Sprintf("DS-%d", ds.ID)
	if item.TargetID != wantTarget {
		t.Errorf("expected target_id=%s, got %s", wantTarget, item.TargetID)
	}
	if item.Persona != "dev" {
		t.Errorf("expected persona=dev, got %q", item.Persona)
	}

	// Recording an assistant turn closes the gap.
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/discussions/%d/messages", ds.ID),
		map[string]any{"slug": d.Slug, "role": "assistant", "content": "pong"}, nil))

	items = d.EvaluateQueue("")
	requireNoKind(t, items, "respond:discussion")
}

// TestQueueRespondDiscussionHintText verifies IS-518: the respond:discussion
// todo carries agent-actionable instructions (not just the raw user message),
// including the discussion ID and a dx discussion reply command.
func TestQueueRespondDiscussionHintText(t *testing.T) {
	d := NewApiDriver(t, "q-discuss-hint", "Queue Discussion Hint")

	var ds struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/discussions",
		map[string]any{"slug": d.Slug, "title": "Agent inbox"}, &ds))

	userMsg := "What is the status of IS-518?"
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/discussions/%d/messages", ds.ID),
		map[string]any{"slug": d.Slug, "role": "user", "content": userMsg}, nil))

	items := d.EvaluateQueue("")
	item := requireKind(t, items, "respond:discussion")

	dsRef := fmt.Sprintf("DS-%d", ds.ID)

	if !strings.Contains(item.Text, dsRef) {
		t.Errorf("todo text should contain %s, got: %s", dsRef, item.Text)
	}
	if !strings.Contains(item.Text, "dx discussion reply") {
		t.Errorf("todo text should contain 'dx discussion reply', got: %s", item.Text)
	}
	if !strings.Contains(item.Text, userMsg) {
		t.Errorf("todo text should include the user message, got: %s", item.Text)
	}
	if !strings.Contains(item.Text, "dx discussion show") {
		t.Errorf("todo text should include 'dx discussion show', got: %s", item.Text)
	}
}

// TestQueueRespondDiscussionSkipsClosed verifies that closed discussions —
// even with a dangling user tail — do not generate a respond candidate.
func TestQueueRespondDiscussionSkipsClosed(t *testing.T) {
	d := NewApiDriver(t, "q-discuss-closed", "Queue Discussion Closed")

	var ds struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/discussions",
		map[string]any{"slug": d.Slug, "title": "Closed thread"}, &ds))

	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/discussions/%d/messages", ds.ID),
		map[string]any{"slug": d.Slug, "role": "user", "content": "trailing question"}, nil))

	if r := apiDo(t, http.MethodPatch,
		fmt.Sprintf("/api/dx/discussions/%d/status", ds.ID),
		map[string]any{"slug": d.Slug, "status": "closed"}, nil); r.StatusCode >= 400 {
		t.Fatalf("close discussion: got %d", r.StatusCode)
	}

	items := d.EvaluateQueue("")
	requireNoKind(t, items, "respond:discussion")
}
