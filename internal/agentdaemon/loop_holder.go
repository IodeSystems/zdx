package agentdaemon

import (
	"context"
	"sync"
)

// LoopTaskHolder is a thread-safe TaskHolder + LeaseRenewer for an in-process
// work loop. The loop calls Set when a session begins and Clear when it ends;
// the daemon's pause hold-loop reads CurrentTask to know which session/issue
// is being held and calls Renew to keep the underlying claim's lease alive.
//
// Renew is plumbed as a closure rather than a typed dependency so the holder
// stays decoupled from the loop's HTTP layer (renewTodoLease, scoped tokens,
// etc.) and remains unit-testable in isolation.
type LoopTaskHolder struct {
	mu       sync.Mutex
	current  *RunningTask
	done     chan struct{}
	renewFn  func()
	drainCh  chan struct{}
	pausedCh chan struct{} // closed when paused; recreated on resume
}

// NewLoopTaskHolder returns a holder ready to be passed to Daemon.Holder.
func NewLoopTaskHolder() *LoopTaskHolder {
	return &LoopTaskHolder{
		drainCh: make(chan struct{}),
	}
}

// Set marks task as the live work item. Replaces any previous task without
// signaling completion — callers must Clear the old task first.
func (h *LoopTaskHolder) Set(task RunningTask, renewFn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = &task
	h.renewFn = renewFn
	h.done = make(chan struct{})
}

// Clear marks the current task as finished. Safe to call when no task is
// set. Closes the per-task done channel so WaitForCompletion observers wake.
func (h *LoopTaskHolder) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.current = nil
	h.renewFn = nil
	if h.done != nil {
		close(h.done)
		h.done = nil
	}
}

// CurrentTask returns a snapshot of the live task, or nil if none. The
// caller may keep the pointer; the holder stores a fresh value on Set so
// later mutations don't race.
func (h *LoopTaskHolder) CurrentTask() *RunningTask {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.current == nil {
		return nil
	}
	snap := *h.current
	return &snap
}

// WaitForCompletion blocks until the live task transitions to Clear, or
// until ctx is cancelled. Returns nil immediately when no task is set.
func (h *LoopTaskHolder) WaitForCompletion(ctx context.Context) error {
	h.mu.Lock()
	ch := h.done
	h.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Renew implements LeaseRenewer by invoking the closure passed to Set.
// Safe to call when no task is held — the closure is nil and Renew is a no-op.
func (h *LoopTaskHolder) Renew(_ context.Context, _, _ string) {
	h.mu.Lock()
	fn := h.renewFn
	h.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// SignalDrain closes the drain channel so DrainSignaled returns true. The
// loop's claim step polls DrainSignaled and exits cleanly after the current
// session releases. Safe to call concurrently; subsequent calls are no-ops.
func (h *LoopTaskHolder) SignalDrain() {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.drainCh:
		return // already closed
	default:
		close(h.drainCh)
	}
}

// DrainSignaled returns true once SignalDrain has been called.
func (h *LoopTaskHolder) DrainSignaled() bool {
	select {
	case <-h.drainCh:
		return true
	default:
		return false
	}
}

// SetPaused toggles the paused state. Pausing creates a fresh blocked channel
// that WaitWhilePaused observes; resuming closes it. Idempotent — repeated
// pauses keep the same channel; repeated resumes are no-ops.
func (h *LoopTaskHolder) SetPaused(paused bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if paused {
		if h.pausedCh == nil {
			h.pausedCh = make(chan struct{})
		}
		return
	}
	if h.pausedCh != nil {
		close(h.pausedCh)
		h.pausedCh = nil
	}
}

// WaitWhilePaused blocks until SetPaused(false) is called or ctx is
// cancelled. Returns immediately when not paused. Used by the loop's claim
// step to gate new claims while a server-pushed pause is active.
func (h *LoopTaskHolder) WaitWhilePaused(ctx context.Context) error {
	h.mu.Lock()
	ch := h.pausedCh
	h.mu.Unlock()
	if ch == nil {
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Paused reports whether the holder is currently paused. Non-blocking
// snapshot for heartbeat/observability — operators can poll this without
// gating the loop. WaitWhilePaused is the gating equivalent.
func (h *LoopTaskHolder) Paused() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.pausedCh != nil
}
