package agent

import "testing"

// TestArgsDigest_KeyOrderInsensitive proves canonical-JSON normalization:
// the same payload with reordered top-level and nested keys collapses to
// one digest. This is the core spin-lock equality contract — without it, a
// model that reshuffles arg keys each turn would silently bypass the
// detector.
func TestArgsDigest_KeyOrderInsensitive(t *testing.T) {
	a := argsDigest(`{"path":"foo.go","limit":100}`)
	b := argsDigest(`{"limit":100,"path":"foo.go"}`)
	if a != b {
		t.Fatalf("key reorder should not change digest:\n a=%s\n b=%s", a, b)
	}

	// Nested case — the canonicalizer recurses into objects.
	c := argsDigest(`{"opts":{"a":1,"b":2},"path":"foo.go"}`)
	d := argsDigest(`{"path":"foo.go","opts":{"b":2,"a":1}}`)
	if c != d {
		t.Fatalf("nested key reorder should not change digest:\n c=%s\n d=%s", c, d)
	}
}

// TestArgsDigest_DistinctOnDifferentValues guards the inverse: payloads
// that DIFFER must produce different digests, otherwise the detector would
// trip on legitimate variation.
func TestArgsDigest_DistinctOnDifferentValues(t *testing.T) {
	a := argsDigest(`{"path":"foo.go"}`)
	b := argsDigest(`{"path":"bar.go"}`)
	if a == b {
		t.Fatalf("different paths should produce different digests, got both=%s", a)
	}
}

// TestArgsDigest_FallbackOnInvalidJSON proves the unmarshal-fail branch
// returns a stable digest of the raw bytes — bit-identical retries (the
// only case spin-lock cares about) still collapse to one digest even when
// the payload is junk.
func TestArgsDigest_FallbackOnInvalidJSON(t *testing.T) {
	a := argsDigest(`not json {{{`)
	b := argsDigest(`not json {{{`)
	if a != b {
		t.Fatalf("bit-identical invalid JSON must produce same digest:\n a=%s\n b=%s", a, b)
	}
	c := argsDigest(`also not json`)
	if a == c {
		t.Fatalf("different invalid payloads should differ, got both=%s", a)
	}
}

// TestSpinTracker_TripsOnConsecutiveIdentical is the happy-path: feeding
// the same (tool, digest) `threshold` times in a row trips the detector
// with repeat_count == threshold and the matching tuple.
func TestSpinTracker_TripsOnConsecutiveIdentical(t *testing.T) {
	tr := newSpinTracker(3)
	digest := argsDigest(`{"path":"x"}`)

	for i := 0; i < 2; i++ {
		tr.Push("read_file", digest)
		if _, _, ok := tr.Tripped(); ok {
			t.Fatalf("should not trip until threshold reached (i=%d)", i)
		}
	}
	tr.Push("read_file", digest)
	entry, count, ok := tr.Tripped()
	if !ok {
		t.Fatalf("should trip on third identical push")
	}
	if entry.tool != "read_file" || entry.digest != digest {
		t.Fatalf("unexpected entry: %+v", entry)
	}
	if count != 3 {
		t.Fatalf("repeat_count: want 3 got %d", count)
	}
}

// TestSpinTracker_NonConsecutiveDoesNotTrip proves the consecutive-only
// rule: an interleaved different call resets the run, so (A, A, B, A, A)
// must NOT trip a threshold-3 tracker. Non-consecutive churn is IS-1172's
// reviewer-side concern, not the prevention circuit.
func TestSpinTracker_NonConsecutiveDoesNotTrip(t *testing.T) {
	tr := newSpinTracker(3)
	a := argsDigest(`{"x":1}`)
	b := argsDigest(`{"x":2}`)

	tr.Push("t", a)
	tr.Push("t", a)
	tr.Push("t", b) // breaks the run
	tr.Push("t", a)
	tr.Push("t", a)
	if _, _, ok := tr.Tripped(); ok {
		t.Fatalf("threshold-3 tracker must not trip on (A,A,B,A,A)")
	}
}

// TestSpinTracker_RespectsCustomThreshold pins the configurability
// contract: agent.spin_lock_threshold=5 means 5 in a row trips, 4 doesn't.
func TestSpinTracker_RespectsCustomThreshold(t *testing.T) {
	tr := newSpinTracker(5)
	d := argsDigest(`{"a":1}`)
	for i := 0; i < 4; i++ {
		tr.Push("tool", d)
	}
	if _, _, ok := tr.Tripped(); ok {
		t.Fatalf("4 pushes must not trip threshold-5 tracker")
	}
	tr.Push("tool", d)
	if _, count, ok := tr.Tripped(); !ok || count != 5 {
		t.Fatalf("5th push: want trip with count=5, got ok=%v count=%d", ok, count)
	}
}

// TestSpinTracker_DisabledNoOps proves threshold ≤ 0 makes the tracker a
// zero-cost no-op: no entries accumulate, Tripped never fires regardless
// of input. Required by the "set negative to disable" config knob.
func TestSpinTracker_DisabledNoOps(t *testing.T) {
	for _, threshold := range []int{0, -1} {
		tr := newSpinTracker(threshold)
		if tr.enabled() {
			t.Fatalf("threshold=%d should be disabled", threshold)
		}
		for i := 0; i < 10; i++ {
			tr.Push("t", "d")
		}
		if _, _, ok := tr.Tripped(); ok {
			t.Fatalf("disabled tracker (threshold=%d) must never trip", threshold)
		}
	}
}

// TestSpinTracker_SlidingWindow proves the buffer trims to the last
// `threshold` entries, so a long history of mixed tools followed by 3
// identical ones still trips — past variance shouldn't immunize the agent
// from a fresh stuck loop.
func TestSpinTracker_SlidingWindow(t *testing.T) {
	tr := newSpinTracker(3)
	for i := 0; i < 50; i++ {
		tr.Push("varying", argsDigest(`{"i":42}`)) // distinct from below
	}
	d := argsDigest(`{"path":"stuck.go"}`)
	tr.Push("read_file", d)
	tr.Push("read_file", d)
	tr.Push("read_file", d)
	entry, count, ok := tr.Tripped()
	if !ok {
		t.Fatalf("sliding window should trip on the most recent 3 identical pushes")
	}
	if entry.tool != "read_file" || count != 3 {
		t.Fatalf("unexpected trip: tool=%s count=%d", entry.tool, count)
	}
}
