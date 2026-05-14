package handlers

import (
	"reflect"
	"testing"
)

// TestSpinLockTraceArgs_KVShape pins the kv contract that traceEvent receives
// when emitting session.aborted_spin_lock. Downstream consumers (reviewer
// IS-1172, churn-hint pipeline, `dx log tail --tag`) rely on these keys; if
// the shape ever drifts, this test fails fast.
func TestSpinLockTraceArgs_KVShape(t *testing.T) {
	got := spinLockTraceArgs("sid-xyz", "smoke4-0", "IS-1116", SpinLockAbort{
		Tool:        "read_file",
		ArgsDigest:  "abc123def",
		RepeatCount: 3,
		LastTurn:    60,
	})

	want := []any{
		"sid", "sid-xyz",
		"alias", "smoke4-0",
		"tool", "read_file",
		"args_digest", "abc123def",
		"repeat_count", int32(3),
		"last_turn", int32(60),
		"issue_id", "IS-1116",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kv mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

// TestSpinLockTraceArgs_EvenLength guards traceEvent's "odd-length kv slices
// drop the dangling key" rule: the helper MUST emit pairs, never a stray key.
// Drift here would silently lose a value when the trace tag hits jsonb @>.
func TestSpinLockTraceArgs_EvenLength(t *testing.T) {
	args := spinLockTraceArgs("sid", "alias", "IS-1", SpinLockAbort{})
	if len(args)%2 != 0 {
		t.Fatalf("kv slice must be even-length pairs; got %d entries: %#v", len(args), args)
	}
}

// TestIneffectiveTraceArgs_KVShape pins the kv contract for session.ineffective
// (IS-1100). The dashboard at IS-1101 aggregates by every key listed here;
// if the shape drifts, the dashboard silently loses a column.
func TestIneffectiveTraceArgs_KVShape(t *testing.T) {
	got := ineffectiveTraceArgs("sid-7", "smoke4-1", "claude-sonnet-4-6", "high", "dev", "IS-1100", 23)
	want := []any{
		"sid", "sid-7",
		"alias", "smoke4-1",
		"model", "claude-sonnet-4-6",
		"complexity", "high",
		"persona", "dev",
		"issue_id", "IS-1100",
		"turn_count", int32(23),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("kv mismatch:\n got=%#v\nwant=%#v", got, want)
	}
}

// TestIneffectiveTraceArgs_EvenLength: same parity guarantee as SpinLockArgs.
func TestIneffectiveTraceArgs_EvenLength(t *testing.T) {
	args := ineffectiveTraceArgs("sid", "alias", "", "", "", "IS-1", 0)
	if len(args)%2 != 0 {
		t.Fatalf("kv slice must be even-length pairs; got %d entries: %#v", len(args), args)
	}
}
