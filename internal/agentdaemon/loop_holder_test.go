package agentdaemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoopTaskHolder_SetClearSnapshot(t *testing.T) {
	h := NewLoopTaskHolder()
	if h.CurrentTask() != nil {
		t.Fatal("expected no task on a fresh holder")
	}
	h.Set(RunningTask{ID: "42", SessionID: "sid-x", IssueID: "IS-1", Started: time.Now()}, nil)

	got := h.CurrentTask()
	if got == nil || got.ID != "42" || got.SessionID != "sid-x" {
		t.Fatalf("CurrentTask snapshot wrong: %+v", got)
	}
	// Mutating the snapshot must not affect the holder's stored task.
	got.ID = "mutated"
	if again := h.CurrentTask(); again.ID != "42" {
		t.Errorf("snapshot mutation leaked back to holder: %+v", again)
	}

	h.Clear()
	if h.CurrentTask() != nil {
		t.Error("CurrentTask must be nil after Clear")
	}
}

func TestLoopTaskHolder_WaitForCompletion(t *testing.T) {
	h := NewLoopTaskHolder()
	// No task → returns immediately.
	if err := h.WaitForCompletion(context.Background()); err != nil {
		t.Errorf("empty holder WaitForCompletion: %v", err)
	}

	h.Set(RunningTask{ID: "1"}, nil)
	woke := make(chan struct{})
	go func() {
		_ = h.WaitForCompletion(context.Background())
		close(woke)
	}()
	select {
	case <-woke:
		t.Fatal("WaitForCompletion returned before Clear")
	case <-time.After(20 * time.Millisecond):
	}
	h.Clear()
	select {
	case <-woke:
	case <-time.After(time.Second):
		t.Fatal("WaitForCompletion did not return after Clear")
	}
}

func TestLoopTaskHolder_RenewInvokesClosure(t *testing.T) {
	h := NewLoopTaskHolder()
	var calls atomic.Int32
	h.Set(RunningTask{ID: "7"}, func() { calls.Add(1) })

	h.Renew(context.Background(), "sid", "issue")
	h.Renew(context.Background(), "sid", "issue")
	if got := calls.Load(); got != 2 {
		t.Errorf("Renew should invoke closure each call; got %d, want 2", got)
	}

	h.Clear()
	// After Clear, Renew must be a no-op even though the daemon may still
	// fire its hold-loop ticker briefly during teardown.
	h.Renew(context.Background(), "sid", "issue")
	if got := calls.Load(); got != 2 {
		t.Errorf("Renew after Clear should be a no-op; calls=%d, want 2", got)
	}
}

func TestLoopTaskHolder_SignalDrainIsIdempotent(t *testing.T) {
	h := NewLoopTaskHolder()
	if h.DrainSignaled() {
		t.Fatal("DrainSignaled should be false initially")
	}
	h.SignalDrain()
	if !h.DrainSignaled() {
		t.Fatal("DrainSignaled should be true after SignalDrain")
	}
	// Second call must not panic on a re-close.
	h.SignalDrain()
	if !h.DrainSignaled() {
		t.Fatal("DrainSignaled stayed false after second SignalDrain")
	}
}

func TestLoopTaskHolder_PauseGatesWaitWhilePaused(t *testing.T) {
	h := NewLoopTaskHolder()
	// Not paused → returns immediately.
	if err := h.WaitWhilePaused(context.Background()); err != nil {
		t.Errorf("unpaused WaitWhilePaused: %v", err)
	}

	h.SetPaused(true)
	woke := make(chan struct{})
	go func() {
		_ = h.WaitWhilePaused(context.Background())
		close(woke)
	}()
	select {
	case <-woke:
		t.Fatal("WaitWhilePaused returned before SetPaused(false)")
	case <-time.After(20 * time.Millisecond):
	}
	h.SetPaused(false)
	select {
	case <-woke:
	case <-time.After(time.Second):
		t.Fatal("WaitWhilePaused did not return after SetPaused(false)")
	}

	// Idempotent pause-false on an unpaused holder should not panic.
	h.SetPaused(false)
}

func TestLoopTaskHolder_PauseCancellable(t *testing.T) {
	h := NewLoopTaskHolder()
	h.SetPaused(true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := h.WaitWhilePaused(ctx); err == nil {
		t.Error("expected ctx error when WaitWhilePaused observes a cancelled ctx")
	}
}
