package handlers

import (
	"context"
	"encoding/json"
	"testing"
)

// TestAuditNoteTagShape verifies the tag-permeation helper packs the
// expected keys into context_json: alias + agent_id from CtxAgentID,
// session_id from CtxSessionID, user_id from CtxUserID, plus arbitrary
// caller-supplied kv pairs. `dx log tail --tag alias=X` matches via
// jsonb @> on this payload, so the contract is "alias key MUST be
// present when ctx carries an agent ID".
//
// We exercise the marshal step directly (no DB) by re-implementing the
// helper's tag-build path here — kept separate from auditNote so a DB
// mock isn't required. If the helper's tag shape ever drifts from this
// expectation, this test fails fast.
func TestAuditNoteTagShape(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() context.Context
		kv   []any
		want map[string]any // expected subset of keys
	}{
		{
			name: "agent + session attribution",
			ctx: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, CtxAgentID, "smoke4-0")
				ctx = context.WithValue(ctx, CtxSessionID, "sid-xyz")
				return ctx
			},
			kv: []any{"task_id", "TK-100", "verdict", "approve"},
			want: map[string]any{
				"alias":      "smoke4-0",
				"agent_id":   "smoke4-0",
				"session_id": "sid-xyz",
				"task_id":    "TK-100",
				"verdict":    "approve",
			},
		},
		{
			name: "user attribution",
			ctx: func() context.Context {
				return context.WithValue(context.Background(), CtxUserID, int32(42))
			},
			kv: []any{"todo_id", int32(7)},
			want: map[string]any{
				"user_id": float64(42), // JSON marshals int32 → float64 on roundtrip
				"todo_id": float64(7),
			},
		},
		{
			name: "no attribution — kv only",
			ctx:  func() context.Context { return context.Background() },
			kv:   []any{"verdict", "reject"},
			want: map[string]any{"verdict": "reject"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tags := buildAuditTags(tc.ctx(), tc.kv)
			payload, err := json.Marshal(tags)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var got map[string]any
			if err := json.Unmarshal(payload, &got); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for k, want := range tc.want {
				if got[k] != want {
					t.Errorf("key %q: want %v (%T), got %v (%T)", k, want, want, got[k], got[k])
				}
			}
			// alias must appear iff agent_id is set on ctx
			if v, ok := got["agent_id"]; ok {
				if got["alias"] != v {
					t.Errorf("alias and agent_id should match; alias=%v agent_id=%v", got["alias"], v)
				}
			} else if _, hasAlias := got["alias"]; hasAlias {
				t.Errorf("alias present without agent_id: got=%v", got)
			}
		})
	}
}

func TestAuditNoteOddLengthKv(t *testing.T) {
	// odd-length kv must not panic and must drop the dangling key.
	tags := buildAuditTags(context.Background(), []any{"task_id", "TK-1", "stray"})
	if _, ok := tags["stray"]; ok {
		t.Errorf("stray dangling key %q should have been dropped", "stray")
	}
	if tags["task_id"] != "TK-1" {
		t.Errorf("task_id: want TK-1 got %v", tags["task_id"])
	}
}

func TestAuditNoteNonStringKey(t *testing.T) {
	// non-string keys are skipped, not panicked on.
	tags := buildAuditTags(context.Background(), []any{42, "ignored", "task_id", "TK-1"})
	if _, ok := tags["ignored"]; ok {
		t.Errorf("non-string-keyed pair should have been dropped")
	}
	if tags["task_id"] != "TK-1" {
		t.Errorf("task_id: want TK-1 got %v", tags["task_id"])
	}
}
