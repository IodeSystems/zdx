// Package zdxclient is the Go client for pushing timing metrics to zdx.
//
// It buffers events in memory and flushes them to zdx's ingest endpoint on
// a timer or when the batch fills. Enqueue is always non-blocking: on
// overflow, events are dropped and the dropped counter is incremented.
// The client owns a single goroutine that does HTTP I/O so callers can
// Record from any goroutine at any rate without backpressure.
//
// Typical lifecycle:
//
//	c, err := zdxclient.New(zdxclient.Config{
//	    Endpoint: "https://zdx.example.com",
//	    Token:    os.Getenv("ZDX_TOKEN"),
//	    Component: "my-service",
//	})
//	if err != nil { ... }
//	defer c.Close(context.Background())
//
//	c.Record("db:query", duration, nil)
package zdxclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// Tags is a free-form label map attached to a single event. Values are folded
// into the server's context_json column.
type Tags map[string]string

// AuthMode selects how the bearer-secret is presented to the server.
//   - AuthBearer (default): "Authorization: Bearer <token>" — for integration tokens.
//   - AuthApiKey: "X-Api-Key: <token>" — for user/admin/scoped api keys; only the
//     log-ingest endpoint accepts this mode today.
type AuthMode string

const (
	AuthBearer AuthMode = "bearer"
	AuthApiKey AuthMode = "x-api-key"
)

// Config controls a Client's endpoint, identity, and batching behavior.
// Only Endpoint and Token are required; other fields have sensible defaults.
type Config struct {
	// Endpoint is the zdx base URL, e.g. "https://zdx.example.com". No trailing slash required.
	Endpoint string

	// Token is the secret used to authenticate. Interpretation depends on AuthMode.
	Token string

	// AuthMode selects the authentication header. Defaults to AuthBearer.
	AuthMode AuthMode

	// ProjectSlug is stamped on log batches when the token isn't project-bound
	// (e.g. user/admin api key, or unbound integration token). Required for the
	// AuthApiKey mode against /api/ingest/logs. Optional otherwise.
	ProjectSlug string

	// Component overrides the token's default component for every event. Optional.
	Component string

	// Environment is stamped on every event — e.g. "prod", "staging", "dev". Optional.
	Environment string

	// Host is stamped on every event's context_json. Defaults to os.Hostname. Optional.
	Host string

	// FlushInterval is the max time an event waits before being sent. Defaults to 5s.
	FlushInterval time.Duration

	// MaxBatch is the number of events that triggers an immediate flush. Defaults to 500.
	MaxBatch int

	// BufferSize is the ring-buffer capacity. Overflow drops events. Defaults to 4096.
	BufferSize int

	// HTTPClient is the transport used for POSTs. Defaults to &http.Client{Timeout: 10s}.
	HTTPClient *http.Client

	// OnError is called for transport/HTTP errors with the number of events affected.
	// Defaults to no-op. Use this to wire into your logger.
	OnError func(err error, eventCount int)
}

// Client holds the in-memory buffer and the background flusher.
// A Client is safe for concurrent use.
type Client struct {
	cfg           Config
	events        chan event
	counterEvents chan counterEvent
	errorEvents   chan errorEvent
	logEvents     chan logEvent
	flushed       chan struct{}
	stop          chan struct{}
	wg            sync.WaitGroup
	closed        atomic.Bool
	dropped       atomic.Uint64
}

// event is the wire-format struct used directly in JSON payloads.
// Matches server-side IngestEvent one-to-one.
type event struct {
	Name       string            `json:"name"`
	DurationMs int32             `json:"duration_ms"`
	Source     string            `json:"source,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

// batch matches server-side IngestBatch.
type batch struct {
	Component   string  `json:"component,omitempty"`
	Environment string  `json:"environment,omitempty"`
	Host        string  `json:"host,omitempty"`
	Events      []event `json:"events"`
}

type counterEvent struct {
	Name   string            `json:"name"`
	Value  int32             `json:"value"`
	Source string            `json:"source,omitempty"`
	Tags   map[string]string `json:"tags,omitempty"`
}

type counterBatch struct {
	Component   string         `json:"component,omitempty"`
	Environment string         `json:"environment,omitempty"`
	Host        string         `json:"host,omitempty"`
	Events      []counterEvent `json:"events"`
}

type errorEvent struct {
	Name       string            `json:"name"`
	Message    string            `json:"message,omitempty"`
	StackTrace string            `json:"stack_trace,omitempty"`
	Source     string            `json:"source,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
}

type errorBatch struct {
	Component   string       `json:"component,omitempty"`
	Environment string       `json:"environment,omitempty"`
	Host        string       `json:"host,omitempty"`
	Events      []errorEvent `json:"events"`
}

type logEvent struct {
	Level   string            `json:"level,omitempty"`
	Message string            `json:"message"`
	Source  string            `json:"source,omitempty"`
	Tags    map[string]string `json:"tags,omitempty"`
}

type logBatch struct {
	ProjectSlug string     `json:"project_slug,omitempty"`
	Component   string     `json:"component,omitempty"`
	Environment string     `json:"environment,omitempty"`
	Host        string     `json:"host,omitempty"`
	Events      []logEvent `json:"events"`
}

// New validates cfg, fills in defaults, and starts the background flusher.
// Call Close when done to drain remaining events.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		return nil, errors.New("zdxclient: Endpoint is required")
	}
	if cfg.Token == "" {
		return nil, errors.New("zdxclient: Token is required")
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxBatch <= 0 {
		cfg.MaxBatch = 500
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 4096
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.OnError == nil {
		cfg.OnError = func(error, int) {}
	}
	if cfg.AuthMode == "" {
		cfg.AuthMode = AuthBearer
	}
	c := &Client{
		cfg:           cfg,
		events:        make(chan event, cfg.BufferSize),
		counterEvents: make(chan counterEvent, cfg.BufferSize),
		errorEvents:   make(chan errorEvent, cfg.BufferSize),
		logEvents:     make(chan logEvent, cfg.BufferSize),
		flushed:       make(chan struct{}),
		stop:          make(chan struct{}),
	}
	c.wg.Add(4)
	go c.run()
	go c.runCounters()
	go c.runErrors()
	go c.runLogs()
	return c, nil
}

// Record enqueues a timing event. Non-blocking: if the buffer is full the
// event is dropped and the Dropped() counter is incremented. Callers should
// Record freely from hot paths without worrying about latency.
func (c *Client) Record(name string, duration time.Duration, tags Tags) {
	if c.closed.Load() {
		return
	}
	ms := int32(duration.Milliseconds()) //nolint:gosec
	ev := event{Name: name, DurationMs: ms, Tags: tags}
	select {
	case c.events <- ev:
	default:
		c.dropped.Add(1)
	}
}

// RecordWithSource is like Record but also stamps a source label on the event
// (e.g. the URL path that originated the work). Useful for HTTP handlers.
func (c *Client) RecordWithSource(name, source string, duration time.Duration, tags Tags) {
	if c.closed.Load() {
		return
	}
	ms := int32(duration.Milliseconds()) //nolint:gosec
	ev := event{Name: name, DurationMs: ms, Source: source, Tags: tags}
	select {
	case c.events <- ev:
	default:
		c.dropped.Add(1)
	}
}

// RecordCounter enqueues a counter event. Non-blocking: if the buffer is full
// the event is dropped and the Dropped() counter is incremented.
func (c *Client) RecordCounter(name string, value int32, tags Tags) {
	if c.closed.Load() {
		return
	}
	ev := counterEvent{Name: name, Value: value, Tags: tags}
	select {
	case c.counterEvents <- ev:
	default:
		c.dropped.Add(1)
	}
}

// RecordCounterWithSource is like RecordCounter but also stamps a source label.
func (c *Client) RecordCounterWithSource(name, source string, value int32, tags Tags) {
	if c.closed.Load() {
		return
	}
	ev := counterEvent{Name: name, Value: value, Source: source, Tags: tags}
	select {
	case c.counterEvents <- ev:
	default:
		c.dropped.Add(1)
	}
}

// RecordError enqueues an error event. Non-blocking.
func (c *Client) RecordError(name, message string, tags Tags) {
	if c.closed.Load() {
		return
	}
	ev := errorEvent{Name: name, Message: message, Tags: tags}
	select {
	case c.errorEvents <- ev:
	default:
		c.dropped.Add(1)
	}
}

// RecordErrorWithStack is like RecordError but includes a stack trace and source.
func (c *Client) RecordErrorWithStack(name, message, stackTrace, source string, tags Tags) {
	if c.closed.Load() {
		return
	}
	ev := errorEvent{Name: name, Message: message, StackTrace: stackTrace, Source: source, Tags: tags}
	select {
	case c.errorEvents <- ev:
	default:
		c.dropped.Add(1)
	}
}

// applyAuth sets the request's auth header based on cfg.AuthMode.
func (c *Client) applyAuth(req *http.Request) {
	switch c.cfg.AuthMode {
	case AuthApiKey:
		req.Header.Set("X-Api-Key", c.cfg.Token)
	default:
		req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	}
}

// RecordLog enqueues a structured log event. Non-blocking.
func (c *Client) RecordLog(level, message string, tags Tags) {
	if c.closed.Load() {
		return
	}
	ev := logEvent{Level: level, Message: message, Tags: tags}
	select {
	case c.logEvents <- ev:
	default:
		c.dropped.Add(1)
	}
}

// RecordLogWithSource is like RecordLog but includes a source label.
func (c *Client) RecordLogWithSource(level, message, source string, tags Tags) {
	if c.closed.Load() {
		return
	}
	ev := logEvent{Level: level, Message: message, Source: source, Tags: tags}
	select {
	case c.logEvents <- ev:
	default:
		c.dropped.Add(1)
	}
}

// Dropped returns the cumulative number of events dropped due to buffer
// overflow or HTTP errors. Useful to surface as its own metric.
func (c *Client) Dropped() uint64 {
	return c.dropped.Load()
}

// Close stops the background flusher and drains any buffered events.
// Returns when the drain completes or ctx is canceled. Idempotent.
func (c *Client) Close(ctx context.Context) error {
	if c.closed.Swap(true) {
		return nil
	}
	close(c.stop)
	done := make(chan struct{})
	go func() {
		c.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run is the flusher goroutine. It collects events until MaxBatch or
// FlushInterval fires, then POSTs. On stop, it drains any remaining buffered
// events before returning.
func (c *Client) run() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()
	buf := make([]event, 0, c.cfg.MaxBatch)
	for {
		select {
		case ev := <-c.events:
			buf = append(buf, ev)
			if len(buf) >= c.cfg.MaxBatch {
				c.flush(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				c.flush(buf)
				buf = buf[:0]
			}
		case <-c.stop:
			// Drain anything still in the channel, then flush the leftover.
			for {
				select {
				case ev := <-c.events:
					buf = append(buf, ev)
				default:
					if len(buf) > 0 {
						c.flush(buf)
					}
					return
				}
			}
		}
	}
}

// runCounters is the counter flusher goroutine — mirrors run() for counter events.
func (c *Client) runCounters() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()
	buf := make([]counterEvent, 0, c.cfg.MaxBatch)
	for {
		select {
		case ev := <-c.counterEvents:
			buf = append(buf, ev)
			if len(buf) >= c.cfg.MaxBatch {
				c.flushCounters(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				c.flushCounters(buf)
				buf = buf[:0]
			}
		case <-c.stop:
			for {
				select {
				case ev := <-c.counterEvents:
					buf = append(buf, ev)
				default:
					if len(buf) > 0 {
						c.flushCounters(buf)
					}
					return
				}
			}
		}
	}
}

func (c *Client) flushCounters(events []counterEvent) {
	payload := counterBatch{
		Component:   c.cfg.Component,
		Environment: c.cfg.Environment,
		Host:        c.cfg.Host,
		Events:      events,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.dropped.Add(uint64(len(events)))
		c.cfg.OnError(fmt.Errorf("marshal: %w", err), len(events))
		return
	}
	url := c.cfg.Endpoint + "/api/ingest/counters"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
		}
		req, rErr := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if rErr != nil {
			lastErr = rErr
			break
		}
		req.Header.Set("Content-Type", "application/json")
		c.applyAuth(req)
		resp, doErr := c.cfg.HTTPClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		lastErr = fmt.Errorf("ingest returned %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			break
		}
	}
	c.dropped.Add(uint64(len(events)))
	c.cfg.OnError(lastErr, len(events))
}

func (c *Client) runErrors() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()
	buf := make([]errorEvent, 0, c.cfg.MaxBatch)
	for {
		select {
		case ev := <-c.errorEvents:
			buf = append(buf, ev)
			if len(buf) >= c.cfg.MaxBatch {
				c.flushErrors(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				c.flushErrors(buf)
				buf = buf[:0]
			}
		case <-c.stop:
			for {
				select {
				case ev := <-c.errorEvents:
					buf = append(buf, ev)
				default:
					if len(buf) > 0 {
						c.flushErrors(buf)
					}
					return
				}
			}
		}
	}
}

func (c *Client) flushErrors(events []errorEvent) {
	payload := errorBatch{
		Component:   c.cfg.Component,
		Environment: c.cfg.Environment,
		Host:        c.cfg.Host,
		Events:      events,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.dropped.Add(uint64(len(events)))
		c.cfg.OnError(fmt.Errorf("marshal: %w", err), len(events))
		return
	}
	url := c.cfg.Endpoint + "/api/ingest/errors"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
		}
		req, rErr := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if rErr != nil {
			lastErr = rErr
			break
		}
		req.Header.Set("Content-Type", "application/json")
		c.applyAuth(req)
		resp, doErr := c.cfg.HTTPClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		lastErr = fmt.Errorf("ingest returned %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			break
		}
	}
	c.dropped.Add(uint64(len(events)))
	c.cfg.OnError(lastErr, len(events))
}

func (c *Client) runLogs() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.cfg.FlushInterval)
	defer ticker.Stop()
	buf := make([]logEvent, 0, c.cfg.MaxBatch)
	for {
		select {
		case ev := <-c.logEvents:
			buf = append(buf, ev)
			if len(buf) >= c.cfg.MaxBatch {
				c.flushLogs(buf)
				buf = buf[:0]
			}
		case <-ticker.C:
			if len(buf) > 0 {
				c.flushLogs(buf)
				buf = buf[:0]
			}
		case <-c.stop:
			for {
				select {
				case ev := <-c.logEvents:
					buf = append(buf, ev)
				default:
					if len(buf) > 0 {
						c.flushLogs(buf)
					}
					return
				}
			}
		}
	}
}

func (c *Client) flushLogs(events []logEvent) {
	payload := logBatch{
		ProjectSlug: c.cfg.ProjectSlug,
		Component:   c.cfg.Component,
		Environment: c.cfg.Environment,
		Host:        c.cfg.Host,
		Events:      events,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.dropped.Add(uint64(len(events)))
		c.cfg.OnError(fmt.Errorf("marshal: %w", err), len(events))
		return
	}
	url := c.cfg.Endpoint + "/api/ingest/logs"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
		}
		req, rErr := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if rErr != nil {
			lastErr = rErr
			break
		}
		req.Header.Set("Content-Type", "application/json")
		c.applyAuth(req)
		resp, doErr := c.cfg.HTTPClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		lastErr = fmt.Errorf("ingest returned %d", resp.StatusCode)
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			break
		}
	}
	c.dropped.Add(uint64(len(events)))
	c.cfg.OnError(lastErr, len(events))
}

// flush POSTs one batch with exponential backoff. On terminal failure the
// batch is discarded and OnError is called with the event count.
func (c *Client) flush(events []event) {
	payload := batch{
		Component:   c.cfg.Component,
		Environment: c.cfg.Environment,
		Host:        c.cfg.Host,
		Events:      events,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		c.dropped.Add(uint64(len(events)))
		c.cfg.OnError(fmt.Errorf("marshal: %w", err), len(events))
		return
	}
	url := c.cfg.Endpoint + "/api/ingest/timings"
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<attempt) * 200 * time.Millisecond)
		}
		req, rErr := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if rErr != nil {
			lastErr = rErr
			break
		}
		req.Header.Set("Content-Type", "application/json")
		c.applyAuth(req)
		resp, doErr := c.cfg.HTTPClient.Do(req)
		if doErr != nil {
			lastErr = doErr
			continue
		}
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		lastErr = fmt.Errorf("ingest returned %d", resp.StatusCode)
		// Don't retry 4xx — the server is telling us something we can't fix.
		if resp.StatusCode >= 400 && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			break
		}
	}
	c.dropped.Add(uint64(len(events)))
	c.cfg.OnError(lastErr, len(events))
}
