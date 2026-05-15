package agent

import (
	"testing"
	"time"
)

func TestTightLoopDetector(t *testing.T) {
	t.Run("window not full → no trigger", func(t *testing.T) {
		d := newTightLoopDetector(5, 60*time.Second)
		base := time.Now()
		for i := range 4 {
			if got := d.record(base.Add(time.Duration(i) * time.Second)); got != 0 {
				t.Fatalf("iter %d: unexpected backoff %v", i, got)
			}
		}
	})

	t.Run("5 quick iterations span <60s → 1m backoff", func(t *testing.T) {
		d := newTightLoopDetector(5, 60*time.Second)
		base := time.Now()
		var got time.Duration
		for i := range 5 {
			got = d.record(base.Add(time.Duration(i) * time.Second))
		}
		if got != time.Minute {
			t.Fatalf("first trip: want 1m, got %v", got)
		}
	})

	t.Run("5 iterations spanning ≥60s → no trigger", func(t *testing.T) {
		d := newTightLoopDetector(5, 60*time.Second)
		base := time.Now()
		// 0s, 20s, 40s, 60s, 80s — spans 80s, exceeds threshold.
		for i := range 5 {
			if got := d.record(base.Add(time.Duration(i*20) * time.Second)); got != 0 {
				t.Fatalf("iter %d: unexpected backoff %v on legitimate idle pacing", i, got)
			}
		}
	})

	t.Run("exponential backoff up to 16m cap", func(t *testing.T) {
		d := newTightLoopDetector(5, 60*time.Second)
		wantSeq := []time.Duration{
			1 * time.Minute,
			2 * time.Minute,
			4 * time.Minute,
			8 * time.Minute,
			16 * time.Minute,
			16 * time.Minute, // cap
			16 * time.Minute, // cap holds
		}
		for trip, want := range wantSeq {
			base := time.Now().Add(time.Duration(trip) * time.Hour)
			var got time.Duration
			for i := range 5 {
				got = d.record(base.Add(time.Duration(i) * time.Second))
			}
			if got != want {
				t.Fatalf("trip %d: want %v, got %v", trip, want, got)
			}
		}
	})

	t.Run("reset clears window and trip counter", func(t *testing.T) {
		d := newTightLoopDetector(5, 60*time.Second)
		base := time.Now()
		for i := range 5 {
			d.record(base.Add(time.Duration(i) * time.Second))
		}
		// One trip recorded; next would be 2m without reset.
		d.reset()
		base = base.Add(time.Hour)
		var got time.Duration
		for i := range 5 {
			got = d.record(base.Add(time.Duration(i) * time.Second))
		}
		if got != time.Minute {
			t.Fatalf("after reset: want 1m (trip counter cleared), got %v", got)
		}
	})

	t.Run("after trip, window cleared so next 4 records do not re-trigger", func(t *testing.T) {
		d := newTightLoopDetector(5, 60*time.Second)
		base := time.Now()
		// First trip.
		for i := range 5 {
			d.record(base.Add(time.Duration(i) * time.Second))
		}
		// Window cleared — next 4 records, even quick ones, must not trip.
		for i := range 4 {
			if got := d.record(base.Add(time.Duration(10+i) * time.Second)); got != 0 {
				t.Fatalf("iter %d post-trip: unexpected backoff %v on partial window", i, got)
			}
		}
	})
}
