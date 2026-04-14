package server

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

// Timing is a single observation queued for persistence by the drainer.
// HTTP, SQL, and embedder timings all flow through one channel so callers
// never have to know about ctx wiring or fire-and-forget goroutines.
type Timing struct {
	Name       string
	DurationMs int32
	Source     string
}

// timingSink is the buffered channel the QueryTracer and middlewares write to.
// A nil sink silently drops — useful during early init or tests.
type timingSink chan Timing

// NewTimingSink returns a sink sized for bursty per-request traffic.
// main.go creates one and hands it to both the pgx tracer and the server.
func NewTimingSink() timingSink {
	return make(chan Timing, 4096)
}

// StartSinkToClient forwards every Timing on sink to c.RecordWithSource.
// zdx-server itself becomes a client of its own ingest endpoint — no direct
// zdx_timed writes happen anywhere except inside the ingest handler.
func StartSinkToClient(sink <-chan Timing, c *zdxclient.Client) {
	go func() {
		for t := range sink {
			c.RecordWithSource(t.Name, t.Source, time.Duration(t.DurationMs)*time.Millisecond, nil)
		}
	}()
}

// track records the duration since start under name, attributing it to
// whatever source ctx carries. Sub-1ms events are dropped as noise.
// Safe on a nil sink. Typical usage:
//
//	start := time.Now()
//	vec, err := c.Embed(ctx, text)
//	e.sink.track(ctx, "llm:embed", start)
func (s timingSink) track(ctx context.Context, name string, start time.Time) {
	if s == nil {
		return
	}
	ms := int32(time.Since(start).Milliseconds()) //nolint:gosec
	if ms < 1 {
		return
	}
	s.send(Timing{Name: name, DurationMs: ms, Source: sourceFromCtx(ctx)})
}

// send pushes a timing without ever blocking the caller. On overflow we drop
// rather than slow the request path.
func (s timingSink) send(t Timing) {
	if s == nil {
		return
	}
	select {
	case s <- t:
	default:
	}
}

// sourceFromCtx returns the label stamped by sourceMiddleware or WithSource,
// falling back to "background" for queries originating outside a labeled scope.
func sourceFromCtx(ctx context.Context) string {
	if s, ok := ctx.Value(ctxSource).(string); ok && s != "" {
		return s
	}
	return "background"
}

// WithSource labels ctx so queries running under it are attributed to source
// instead of "background". Use at the entry of cron jobs, CLI commands, or
// any long-lived goroutine you want visible in the timings dashboard.
func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, ctxSource, source)
}

// WithoutTiming returns a ctx that the QueryTracer will skip entirely.
// Reserved for the drainer itself and rare genuinely-uninteresting queries.
func WithoutTiming(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxSkipTiming, true)
}

// StartTimedEventsRetention deletes zdx_timed_events and zdx_counter_events
// rows older than 30 days. Runs once immediately, then every 24 hours.
func (s *Server) StartTimedEventsRetention(ctx context.Context) {
	cleanup := func() {
		cutoff := pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -30), Valid: true}
		noTiming := WithoutTiming(ctx)
		n, err := s.q.DeleteTimedEventsOlderThan(noTiming, cutoff)
		if err != nil {
			log.Printf("timed-events retention: %v", err)
		} else if n > 0 {
			log.Printf("timed-events retention: deleted %d rows", n)
		}
		cn, cerr := s.q.DeleteCounterEventsOlderThan(noTiming, cutoff)
		if cerr != nil {
			log.Printf("counter-events retention: %v", cerr)
		} else if cn > 0 {
			log.Printf("counter-events retention: deleted %d rows", cn)
		}
		en, eerr := s.q.DeleteErrorEventsOlderThan(noTiming, cutoff)
		if eerr != nil {
			log.Printf("error-events retention: %v", eerr)
		} else if en > 0 {
			log.Printf("error-events retention: deleted %d rows", en)
		}
		ln, lerr := s.q.DeleteLogEventsOlderThan(noTiming, cutoff)
		if lerr != nil {
			log.Printf("log-events retention: %v", lerr)
		} else if ln > 0 {
			log.Printf("log-events retention: deleted %d rows", ln)
		}
	}
	cleanup()
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanup()
			}
		}
	}()
}
