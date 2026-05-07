package handlers

import (
	"context"
	"encoding/json"
	"testing"
)

// TestTraceTagShape verifies buildTraceTags packs the expected keys into
// context_json: trace_id from CtxTraceID, alias + agent_id from
// CtxAgentID, session_id from CtxSessionID, user_id from CtxUserID, plus
// arbitrary caller-supplied kv pairs. `dx log tail --tag trace_id=X`
// matches via jsonb @> on this payload, so the contract is "the
// correlation keys MUST be present whenever ctx carries them."
//
// Tests run buildTraceTags directly (no DB) so they don't need a Queries
// mock. If the tag shape ever drifts from this expectation, this test
// fails fast.
func TestTraceTagShape(t *testing.T) {
	tests := []struct {
		name string
		ctx  func() context.Context
		kv   []any
		want map[string]any // expected subset of keys
	}{
		{
			name: "agent + session + trace correlation",
			ctx: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, CtxAgentID, "smoke4-0")
				ctx = context.WithValue(ctx, CtxSessionID, "sid-xyz")
				ctx = context.WithValue(ctx, CtxTraceID, "trace-abc-123")
				return ctx
			},
			kv: []any{"task_id", "TK-100", "verdict", "approve"},
			want: map[string]any{
				"trace_id":   "trace-abc-123",
				"alias":      "smoke4-0",
				"agent_id":   "smoke4-0",
				"session_id": "sid-xyz",
				"task_id":    "TK-100",
				"verdict":    "approve",
			},
		},
		{
			name: "user-only attribution (UI request)",
			ctx: func() context.Context {
				return context.WithValue(context.Background(), CtxUserID, int32(42))
			},
			kv: []any{"todo_id", int32(7)},
			want: map[string]any{
				"user_id": float64(42),
				"todo_id": float64(7),
			},
		},
		{
			name: "no attribution — kv only",
			ctx:  func() context.Context { return context.Background() },
			kv:   []any{"verdict", "reject"},
			want: map[string]any{"verdict": "reject"},
		},
		{
			name: "trace_id without agent_id (server-internal call)",
			ctx: func() context.Context {
				return context.WithValue(context.Background(), CtxTraceID, "trace-internal")
			},
			kv: []any{},
			want: map[string]any{
				"trace_id": "trace-internal",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tags := buildTraceTags(tc.ctx(), tc.kv)
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

func TestTraceTagOddLengthKv(t *testing.T) {
	tags := buildTraceTags(context.Background(), []any{"task_id", "TK-1", "stray"})
	if _, ok := tags["stray"]; ok {
		t.Errorf("stray dangling key %q should have been dropped", "stray")
	}
	if tags["task_id"] != "TK-1" {
		t.Errorf("task_id: want TK-1 got %v", tags["task_id"])
	}
}

func TestTraceTagNonStringKey(t *testing.T) {
	tags := buildTraceTags(context.Background(), []any{42, "ignored", "task_id", "TK-1"})
	if _, ok := tags["ignored"]; ok {
		t.Errorf("non-string-keyed pair should have been dropped")
	}
	if tags["task_id"] != "TK-1" {
		t.Errorf("task_id: want TK-1 got %v", tags["task_id"])
	}
}
