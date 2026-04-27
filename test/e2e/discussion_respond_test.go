package e2e

import (
	"fmt"
	"net/http"
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
