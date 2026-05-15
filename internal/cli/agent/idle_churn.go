package agent

import "time"

// tightLoopDetector signals when the supervisor loop is spinning faster
// than its declared idle interval — symptom-level guard against any bug
// class (transport flap, malformed claim shape, future no-claim payload)
// that makes Take return quickly without the 60s idle sleep paying out.
//
// Defense-in-depth: the existing per-todo churn backoff in runLoop
// (claude.go) targets a specific todo retrying with no progress; this
// detector targets the loop itself iterating too fast regardless of
// cause. See TK-1836 + IS-1234.
type tightLoopDetector struct {
	window    []time.Time
	capacity  int           // sliding-window size (e.g. 5)
	span      time.Duration // threshold (e.g. 60s)
	triggered int           // consecutive trips, for exponential backoff
}

func newTightLoopDetector(capacity int, span time.Duration) *tightLoopDetector {
	return &tightLoopDetector{
		capacity: capacity,
		span:     span,
		window:   make([]time.Time, 0, capacity),
	}
}

// record appends an iteration-start timestamp and returns a backoff
// duration when the window is full and spans less than the threshold.
// Returns 0 when no backoff should be applied. After a trip, the window
// is cleared and the consecutive trip counter is incremented so the next
// trip yields a larger backoff (1m, 2m, 4m, 8m, 16m cap).
func (d *tightLoopDetector) record(now time.Time) time.Duration {
	d.window = append(d.window, now)
	if len(d.window) > d.capacity {
		d.window = d.window[len(d.window)-d.capacity:]
	}
	if len(d.window) < d.capacity {
		return 0
	}
	if d.window[len(d.window)-1].Sub(d.window[0]) >= d.span {
		return 0
	}
	backoff := time.Duration(1<<min(d.triggered, 4)) * time.Minute
	d.triggered++
	d.window = d.window[:0]
	return backoff
}

// reset clears the window and consecutive-trip counter. Call when a real
// session runs to completion (real todo claimed and a session ran).
func (d *tightLoopDetector) reset() {
	d.window = d.window[:0]
	d.triggered = 0
}
