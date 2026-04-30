package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// AgentAdapter is the provider-agnostic surface a coding agent (Claude,
// local-LLM, future providers) implements so RunLifecycle can drive its full
// session lifecycle: create on the server, tail its transcript + subagent
// transcripts, render each event, stream events to the server, and close on
// process exit.
//
// Implementations must be cheap to construct and safe to use from a single
// goroutine. RunLifecycle owns the lifecycle goroutines.
type AgentAdapter interface {
	// Provider returns a short identifier ("claude", "local", ...) recorded
	// on the server-side session.
	Provider() string

	// Start launches the underlying agent process and returns the path of
	// the main JSONL transcript the runner should tail. The caller is
	// expected to start streaming after this returns.
	Start(ctx context.Context, sid, issueID, alias string) (transcriptPath string, err error)

	// Wait blocks until the agent process exits.
	Wait() (exitCode int, err error)

	// SubagentDir returns the directory under which the runner should look
	// for `agent-*.jsonl` subagent transcripts to spawn child tailers.
	// Return "" for adapters that don't have subagents.
	SubagentDir(sid string) string

	// ParseLine converts one raw JSONL line from a transcript into a
	// generic event ready to be persisted/streamed. agentID is empty for
	// main-session events; non-empty for subagent events.
	ParseLine(line []byte, agentID string) (AgentEvent, error)

	// RenderEvent renders one event JSON into a single human-readable line
	// for stdout + .zdx/logs/agent.log. Return "" to skip rendering.
	RenderEvent(eventJSON []byte) string
}

// AuditExtractor is an optional interface adapters may implement to emit
// synthetic audit events (tool_call/file_edit/shell_cmd) derived from raw
// JSONL lines. If an adapter implements this, handleTranscriptLine calls it
// and enqueues the returned events alongside the original.
type AuditExtractor interface {
	ExtractAuditEvents(line []byte, agentID string) []AgentEvent
}

// AgentEvent is the wire-level event RunLifecycle persists and streams.
// EventJSON is the opaque adapter-specific payload the server stores as-is;
// it must be valid JSON.
type AgentEvent struct {
	EventType        string
	EventJSON        json.RawMessage
	AgentID          string
	AgentType        string
	AgentDescription string
}

// agentLifecycleState is the on-disk state for a single session. Persisted
// to .zdx/state/agent/<sid>.json after every batch flush so a killed runner
// can resume without re-sending events the server already has.
type agentLifecycleState struct {
	Sid       string                             `json:"sid"`
	IssueID   string                             `json:"issue_id"`
	Alias     string                             `json:"alias"`
	Provider  string                             `json:"provider"`
	GlobalSeq int32                              `json:"global_seq"`
	Sources   map[string]*agentLifecycleSourceSt `json:"sources"`
}

type agentLifecycleSourceSt struct {
	Path             string `json:"path"`
	Offset           int64  `json:"offset"`
	AgentID          string `json:"agent_id"`
	AgentType        string `json:"agent_type"`
	AgentDescription string `json:"agent_description"`
}

// agentEventsBatch is one ndjson line for the /events endpoint.
type agentEventsBatch struct {
	Seq              int32           `json:"seq"`
	EventType        string          `json:"event_type"`
	EventJSON        json.RawMessage `json:"event_json"`
	AgentID          string          `json:"agent_id,omitempty"`
	AgentType        string          `json:"agent_type,omitempty"`
	AgentDescription string          `json:"agent_description,omitempty"`
}

// RunLifecycle drives one full agent session from start to close. It:
//  1. POSTs /api/dx/agent/sessions/{sid}/create (idempotent server-side).
//     On success, prints the live UI session URL to stdout so the operator
//     can open the server-side event stream page.
//  2. Spawns goroutines to tail the main transcript and each new
//     agent-*.jsonl in adapter.SubagentDir(sid). Each line is parsed,
//     emitted to stdout as a single heartbeat-dot, appended (fully
//     rendered) to .zdx/logs/agent.log, and queued to the event-flusher
//     goroutine. Full event detail is viewable in the UI session page.
//  3. The flusher batches queued events and POSTs them to
//     /api/dx/agent/sessions/{sid}/events as ndjson; the server dedupes
//     by seq so retries are safe.
//  4. On adapter.Wait() returning, drains the queue, prints a trailing
//     newline to terminate the dots line, then POSTs /close with
//     exit_code, duration_ms, event_count, and tokens (zeros for now —
//     the server derives real token counts from events).
//
// The remote may be invalid (e.g. dev with no server configured); in that
// case rendering still runs but no HTTP traffic is sent and no URL is
// printed.
// defaultStallTimeout is how long a session can go without producing any
// transcript event before the watchdog SIGTERMs the agent process. Common
// cause: a background task (Monitor, run_in_background) completes but its
// tail -f reader hangs, leaving the session silently blocked. 15 minutes
// is well above any legitimate operation (bin/ship ~2m, test suites ~3m)
// while catching stuck sessions before they burn hours of lease time.
// Previously 4 hours — reduced after observing agents stuck for 30+ min
// on completed background tasks.
// catches sessions stuck on a hanging tool call (e.g. a command that blocks
// forever). Set to 0 to disable.
const defaultStallTimeout = 15 * time.Minute

// ErrSessionStalled is returned by RunLifecycle when the stall watchdog
// kills the agent process. Callers can check this to decide whether to
// retry with a resume or summary.
var ErrSessionStalled = errors.New("session stalled: no transcript activity")

func RunLifecycle(
	ctx context.Context,
	adapter AgentAdapter,
	rc remoteConfig,
	sid, issueID, alias, trigger string,
	todoID int32,
) (exitCode int, err error) {
	if adapter == nil {
		return 1, errors.New("RunLifecycle: nil adapter")
	}
	provider := adapter.Provider()
	if provider == "" {
		provider = "unknown"
	}

	stateDir := filepath.Join(".zdx", "state", "agent")
	_ = os.MkdirAll(stateDir, 0o755)
	stateFile := filepath.Join(stateDir, sid+".json")

	logDir := filepath.Join(".zdx", "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logFile := filepath.Join(logDir, "agent.log")
	logF, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if logF != nil {
		defer logF.Close()
	}

	state := loadAgentLifecycleState(stateFile)
	if state == nil {
		state = &agentLifecycleState{
			Sid:      sid,
			IssueID:  issueID,
			Alias:    alias,
			Provider: provider,
			Sources:  map[string]*agentLifecycleSourceSt{},
		}
	}

	if rc.valid() {
		sessPK, cErr := postAgentSessionCreate(rc, sid, issueID, alias, provider, trigger, todoID)
		if cErr != nil {
			// Visible on stdout (not just stderr) so the operator sees the
			// session isn't being recorded — otherwise the only clue is the
			// missing "Session:" URL line, which is too easy to overlook.
			fmt.Printf("%s session create failed — events will not be recorded: %v\n",
				colorize(ansiBold+ansiRed, "WARNING:"), cErr)
			fmt.Fprintf(os.Stderr, "[lifecycle] create: %v\n", cErr)
		} else {
			// The UI route /project/<slug>/agents/<id> expects the numeric
			// session pk (it coerces the path param with Number()); the UUID
			// sid would yield NaN and all downstream fetches would fail.
			sessionURL := fmt.Sprintf("%s/project/%s/agents/%d",
				strings.TrimRight(rc.url, "/"), rc.slug, sessPK)
			fmt.Printf("%s %s\n", colorize(ansiBold, "Session:"), colorize(ansiBold+ansiCyan, sessionURL))
		}
	}

	// Derive a child context the stall watchdog can cancel to SIGTERM the
	// agent without cancelling the parent loop context.
	sessionCtx, sessionCancel := context.WithCancel(ctx)
	defer sessionCancel()

	transcriptPath, err := adapter.Start(sessionCtx, sid, issueID, alias)
	if err != nil {
		return 1, fmt.Errorf("adapter.Start: %w", err)
	}

	flusher := newAgentEventFlusher(rc, sid, state, stateFile)
	// Build compact label for dot-line prefixes so the operator always knows
	// which project/issue/todo is running without scrolling up.
	{
		var labelParts []string
		if rc.slug != "" {
			labelParts = append(labelParts, rc.slug)
		}
		if issueID != "" {
			labelParts = append(labelParts, issueID)
		}
		if todoID > 0 {
			labelParts = append(labelParts, fmt.Sprintf("todo-%d", todoID))
		}
		if len(labelParts) > 0 {
			flusher.dotLabel = "[" + strings.Join(labelParts, "/") + "] "
		}
	}
	flusher.start(sessionCtx)
	// Seed last-event time so the watchdog doesn't fire before any events arrive.
	flusher.lastEventUnix.Store(time.Now().UnixNano())

	mainCancel := tailTranscript(sessionCtx, transcriptPath, "", "", "", adapter, logF, flusher, state, stateFile)
	subCancel := watchSubagents(sessionCtx, adapter, sid, logF, flusher, state, stateFile)

	// Stall watchdog: if no transcript events arrive for defaultStallTimeout,
	// cancel sessionCtx so the adapter's signal handler SIGTERMs the agent.
	var stalled atomic.Bool
	if defaultStallTimeout > 0 {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-sessionCtx.Done():
					return
				case <-ticker.C:
					last := flusher.lastEventUnix.Load()
					if last > 0 && time.Since(time.Unix(0, last)) > defaultStallTimeout {
						fmt.Fprintf(os.Stderr, "[lifecycle] stall watchdog: no transcript events for %s, terminating session\n", defaultStallTimeout)
						stalled.Store(true)
						sessionCancel()
						return
					}
				}
			}
		}()
	}

	startedAt := time.Now()
	eventCountAtStart := state.GlobalSeq
	exitCode, waitErr := adapter.Wait()

	mainCancel()
	subCancel()
	flusher.stopAndDrain()

	// Terminate the heartbeat-dots line before the token/close summaries
	// print on their own lines.
	if flusher.sawEvent.Load() {
		fmt.Println()
	}

	durationMs := time.Since(startedAt).Milliseconds()
	emitted := state.GlobalSeq - eventCountAtStart

	if rc.valid() {
		if cErr := postAgentSessionClose(rc, sid, exitCode, durationMs, emitted); cErr != nil {
			fmt.Fprintf(os.Stderr, "[lifecycle] close: %v\n", cErr)
		}
	}

	// Successful close — clear the resume state so the next run starts fresh.
	_ = os.Remove(stateFile)

	if stalled.Load() {
		return exitCode, ErrSessionStalled
	}
	if waitErr != nil {
		return exitCode, waitErr
	}
	return exitCode, nil
}

// ── State persistence ─────────────────────────────────────────────────────

func loadAgentLifecycleState(path string) *agentLifecycleState {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var s agentLifecycleState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	if s.Sources == nil {
		s.Sources = map[string]*agentLifecycleSourceSt{}
	}
	return &s
}

func saveAgentLifecycleState(path string, s *agentLifecycleState) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, path)
}

// ── Tailing ───────────────────────────────────────────────────────────────

// tailTranscript polls one transcript file and feeds each line into the
// flusher. sourceKey identifies the source for state-tracking; pass "" to
// use the path. Returns a cancel func.
func tailTranscript(
	ctx context.Context,
	path string,
	sourceKey, agentID, agentType string,
	adapter AgentAdapter,
	logF *os.File,
	flusher *agentEventFlusher,
	state *agentLifecycleState,
	stateFile string,
) func() {
	if sourceKey == "" {
		sourceKey = path
	}
	done := make(chan struct{})
	go func() {
		// Wait for file to exist (polled, no fsnotify dependency).
		for i := 0; i < 240; i++ {
			if _, err := os.Stat(path); err == nil {
				break
			}
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-time.After(500 * time.Millisecond):
			}
		}

		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		// Resume from saved offset if present.
		var src *agentLifecycleSourceSt
		flusher.mu.Lock()
		src = state.Sources[sourceKey]
		if src == nil {
			src = &agentLifecycleSourceSt{Path: path, AgentID: agentID, AgentType: agentType}
			state.Sources[sourceKey] = src
		}
		startOffset := src.Offset
		flusher.mu.Unlock()

		if startOffset > 0 {
			if _, sErr := f.Seek(startOffset, io.SeekStart); sErr != nil {
				startOffset = 0
				_, _ = f.Seek(0, io.SeekStart)
			}
		}

		reader := bufio.NewReader(f)
		offset := startOffset
		var partial bytes.Buffer

		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			default:
			}
			chunk, rErr := reader.ReadBytes('\n')
			if len(chunk) > 0 {
				partial.Write(chunk)
				if chunk[len(chunk)-1] == '\n' {
					line := partial.Bytes()
					offset += int64(len(line))
					handleTranscriptLine(line, agentID, agentType, "", adapter, logF, flusher, sourceKey, offset, state, stateFile)
					partial.Reset()
				}
			}
			if rErr != nil {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-time.After(200 * time.Millisecond):
				}
			}
		}
	}()
	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
	}
}

func handleTranscriptLine(
	line []byte,
	agentID, agentType, agentDesc string,
	adapter AgentAdapter,
	logF *os.File,
	flusher *agentEventFlusher,
	sourceKey string,
	offset int64,
	state *agentLifecycleState,
	stateFile string,
) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return
	}
	ev, pErr := adapter.ParseLine(line, agentID)
	if pErr != nil {
		return
	}
	if ev.AgentID == "" && agentID != "" {
		ev.AgentID = agentID
	}
	if ev.AgentType == "" && agentType != "" {
		ev.AgentType = agentType
	}
	if ev.AgentDescription == "" && agentDesc != "" {
		ev.AgentDescription = agentDesc
	}
	if rendered := adapter.RenderEvent(ev.EventJSON); rendered != "" {
		// stdout gets a compact heartbeat-dot per event; the full rendered
		// text is preserved in .zdx/logs/agent.log. The clickable session
		// URL (printed by RunLifecycle on session create) is the path to
		// the live-streamed UI session page for full detail.
		const dotsPerLine = 80
		n := flusher.dotCount.Add(1)
		if (n-1)%dotsPerLine == 0 {
			// Start of a new dot line: emit a preceding newline (except for
			// the very first line, which follows the Session: URL's own \n)
			// then the context label so the operator always sees which
			// project/issue/todo is active.
			if n > 1 {
				_, _ = os.Stdout.Write([]byte("\n"))
			}
			if flusher.dotLabel != "" {
				_, _ = os.Stdout.Write([]byte(flusher.dotLabel))
			}
		}
		_, _ = os.Stdout.Write([]byte("."))
		flusher.sawEvent.Store(true)
		if logF != nil {
			fmt.Fprintln(logF, rendered)
		}
	}
	flusher.enqueue(ev, sourceKey, offset)

	if ae, ok := adapter.(AuditExtractor); ok {
		for _, auditEv := range ae.ExtractAuditEvents(line, agentID) {
			if auditEv.AgentID == "" && agentID != "" {
				auditEv.AgentID = agentID
			}
			if auditEv.AgentType == "" && agentType != "" {
				auditEv.AgentType = agentType
			}
			flusher.enqueue(auditEv, sourceKey, offset)
		}
	}
}

// watchSubagents polls the adapter's subagent dir and spawns a tailer for
// each new agent-*.jsonl file. Returns a cancel func.
func watchSubagents(
	ctx context.Context,
	adapter AgentAdapter,
	sid string,
	logF *os.File,
	flusher *agentEventFlusher,
	state *agentLifecycleState,
	stateFile string,
) func() {
	dir := adapter.SubagentDir(sid)
	if dir == "" {
		return func() {}
	}
	done := make(chan struct{})
	var mu sync.Mutex
	known := map[string]bool{}
	var cancels []func()

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			matches, _ := filepath.Glob(filepath.Join(dir, "agent-*.jsonl"))
			mu.Lock()
			for _, m := range matches {
				if known[m] {
					continue
				}
				known[m] = true
				agentID, agentType, _ := parseSubagentMeta(m)
				cancel := tailTranscript(ctx, m, m, agentID, agentType, adapter, logF, flusher, state, stateFile)
				cancels = append(cancels, cancel)
			}
			mu.Unlock()
		}
	}()
	return func() {
		select {
		case <-done:
		default:
			close(done)
		}
		mu.Lock()
		for _, c := range cancels {
			c()
		}
		mu.Unlock()
	}
}

// ── Event flusher ─────────────────────────────────────────────────────────

// agentEventFlusher batches events from all tailers and POSTs them as
// ndjson to /api/dx/agent/sessions/{sid}/events. Persists state after every
// successful flush so a killed runner resumes without losing position.
type agentEventFlusher struct {
	rc        remoteConfig
	sid       string
	state     *agentLifecycleState
	stateFile string

	mu        sync.Mutex
	queue     []agentEventsBatch
	queueOff  []flushOffsetUpdate
	flushTick *time.Ticker
	doneCh    chan struct{}
	flushedCh chan struct{}

	// sawEvent is set true the first time a rendered event is written as a
	// stdout heartbeat-dot; RunLifecycle reads it post-Wait to decide
	// whether to emit a trailing newline that terminates the dots line.
	sawEvent atomic.Bool

	// dotLabel is a compact context prefix (e.g. "[slug/IS-N/todo-N] ") printed
	// at the start of each wrapped dot line so the operator always sees which
	// project/issue/todo is active without needing to scroll up.
	dotLabel string
	// dotCount tracks total dots emitted; used to wrap at dotsPerLine.
	dotCount atomic.Int64

	// lastEventUnix is updated (UnixNano) every time an event is enqueued.
	// The stall watchdog reads it to detect stuck sessions.
	lastEventUnix atomic.Int64
}

type flushOffsetUpdate struct {
	sourceKey string
	offset    int64
}

func newAgentEventFlusher(rc remoteConfig, sid string, state *agentLifecycleState, stateFile string) *agentEventFlusher {
	return &agentEventFlusher{
		rc:        rc,
		sid:       sid,
		state:     state,
		stateFile: stateFile,
		doneCh:    make(chan struct{}),
		flushedCh: make(chan struct{}, 1),
	}
}

func (f *agentEventFlusher) start(ctx context.Context) {
	f.flushTick = time.NewTicker(500 * time.Millisecond)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-f.doneCh:
				return
			case <-f.flushTick.C:
				f.flush()
			}
		}
	}()
}

func (f *agentEventFlusher) enqueue(ev AgentEvent, sourceKey string, offset int64) {
	f.lastEventUnix.Store(time.Now().UnixNano())
	f.mu.Lock()
	f.state.GlobalSeq++
	seq := f.state.GlobalSeq
	f.queue = append(f.queue, agentEventsBatch{
		Seq:              seq,
		EventType:        ev.EventType,
		EventJSON:        ev.EventJSON,
		AgentID:          ev.AgentID,
		AgentType:        ev.AgentType,
		AgentDescription: ev.AgentDescription,
	})
	f.queueOff = append(f.queueOff, flushOffsetUpdate{sourceKey: sourceKey, offset: offset})
	f.mu.Unlock()
}

func (f *agentEventFlusher) flush() {
	f.mu.Lock()
	if len(f.queue) == 0 {
		f.mu.Unlock()
		return
	}
	batch := f.queue
	offs := f.queueOff
	f.queue = nil
	f.queueOff = nil
	f.mu.Unlock()

	if !f.rc.valid() {
		// No server configured — still apply offsets so the state file
		// reflects what we processed.
		f.applyOffsetsAndSave(offs)
		return
	}

	var body bytes.Buffer
	enc := json.NewEncoder(&body)
	for _, ev := range batch {
		_ = enc.Encode(ev)
	}

	u := fmt.Sprintf("%s/api/dx/agent/sessions/%s/events?slug=%s",
		f.rc.url, url.PathEscape(f.sid), url.QueryEscape(f.rc.slug))
	req, _ := http.NewRequest("POST", u, &body)
	req.Header.Set("X-Api-Key", f.rc.key)
	req.Header.Set("Content-Type", "application/x-ndjson")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Re-queue at head so we retry on the next tick.
		f.mu.Lock()
		f.queue = append(batch, f.queue...)
		f.queueOff = append(offs, f.queueOff...)
		f.mu.Unlock()
		return
	}
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		f.mu.Lock()
		f.queue = append(batch, f.queue...)
		f.queueOff = append(offs, f.queueOff...)
		f.mu.Unlock()
		return
	}

	f.applyOffsetsAndSave(offs)
}

func (f *agentEventFlusher) applyOffsetsAndSave(offs []flushOffsetUpdate) {
	f.mu.Lock()
	for _, o := range offs {
		if src, ok := f.state.Sources[o.sourceKey]; ok && o.offset > src.Offset {
			src.Offset = o.offset
		}
	}
	saveAgentLifecycleState(f.stateFile, f.state)
	f.mu.Unlock()
}

func (f *agentEventFlusher) stopAndDrain() {
	if f.flushTick != nil {
		f.flushTick.Stop()
	}
	close(f.doneCh)
	// Final flush to drain anything left in the queue.
	f.flush()
}

// ── Server HTTP helpers ───────────────────────────────────────────────────

// postAgentSessionCreate POSTs the session-create body and returns the server's
// numeric session pk from the response. The pk — not the UUID sid — is what the
// UI route expects in its path; see the Session: URL build in RunLifecycle.
func postAgentSessionCreate(rc remoteConfig, sid, issueID, alias, provider, trigger string, todoID int32) (int64, error) {
	// Server's Huma schema currently marks every body field required. Send
	// explicit empty strings for the optional identity fields so the request
	// validates rather than 422s. IS-261 also loosens the server side; this
	// is belt-and-suspenders so a future tag reset doesn't silently break
	// session recording again.
	body := dxclient.CreateAgentSessionRequest{
		Provider: provider,
		IssueId:  issueID,
		Alias:    alias,
		Trigger:  trigger,
	}
	if todoID > 0 {
		body.TodoId = &todoID
	}
	path := fmt.Sprintf("/api/dx/agent/sessions/%s/create?slug=%s",
		url.PathEscape(sid), url.QueryEscape(rc.slug))
	var resp dxclient.CreateAgentSessionResponse
	if err := postLifecycleJSON(rc, path, body, &resp, "create"); err != nil {
		return 0, err
	}
	return resp.Id, nil
}

func postAgentSessionClose(rc remoteConfig, sid string, exitCode int, durationMs int64, eventCount int32) error {
	body := dxclient.CloseAgentSessionRequest{
		ExitCode:   int32(exitCode),
		DurationMs: durationMs,
		EventCount: eventCount,
		// Tokens are derived server-side from persisted events; we send
		// zeros and let the server's GetClaudeSessionTokenUsage be the
		// source of truth.
		Tokens: dxclient.TokensStruct{},
	}
	path := fmt.Sprintf("/api/dx/agent/sessions/%s/close?slug=%s",
		url.PathEscape(sid), url.QueryEscape(rc.slug))
	return postLifecycleJSON(rc, path, body, nil, "close")
}

// postLifecycleJSON marshals body and POSTs it to rc.url+path with the
// standard session-lifecycle headers. If respOut is non-nil, decodes the 2xx
// response body into it. op is a short label used in error messages so the
// operator can tell create from close from events.
func postLifecycleJSON(rc remoteConfig, path string, body, respOut any, op string) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("%s: marshal: %w", op, err)
	}
	req, _ := http.NewRequest("POST", rc.url+path, bytes.NewReader(buf))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s HTTP %d: %s", op, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if respOut != nil {
		if err := json.NewDecoder(resp.Body).Decode(respOut); err != nil {
			return fmt.Errorf("%s: decode response: %w", op, err)
		}
	}
	return nil
}
