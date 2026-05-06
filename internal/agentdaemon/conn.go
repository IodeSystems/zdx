package agentdaemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

const (
	drainTimeout         = 10 * time.Minute
	backoffInitial       = 1 * time.Second
	backoffMax           = 30 * time.Second
	backoffResetAge      = 10 * time.Second // connection survived this long → reset backoff
	defaultPauseRenewInt = 5 * time.Minute  // mirrors take.go (lease/2 with min 1m), conservative default

	// pingInterval keeps the WS connection from being killed by idle timeouts
	// in intermediate proxies (nginx default 60s; observed ~50s with the
	// production deployment). Pinging at half-the-cutoff gives us a margin
	// against scheduling jitter and one missed pong.
	pingInterval = 25 * time.Second
	// pingTimeout bounds a single Ping round-trip. nhooyr.io/websocket's Ping
	// blocks until pong returns, ctx expires, or the conn is closed.
	pingTimeout = 10 * time.Second
)

// LeaseRenewer is invoked by the pause hold loop to keep a paused agent's task
// lease alive while no new LLM turns are running. Wiring is left to the caller
// — the daemon does not perform HTTP itself.
type LeaseRenewer interface {
	Renew(ctx context.Context, sessionID, issueID string)
}

// LeaseRenewerFunc adapts a plain func to LeaseRenewer.
type LeaseRenewerFunc func(ctx context.Context, sessionID, issueID string)

// Renew implements LeaseRenewer.
func (f LeaseRenewerFunc) Renew(ctx context.Context, sessionID, issueID string) {
	f(ctx, sessionID, issueID)
}

// Daemon holds the configuration for a persistent agent connection.
type Daemon struct {
	ServerURL      string
	AgentID        string
	APIKey         string
	WorktreePath   string
	WorktreeBranch string
	Hostname       string
	Pid            int32
	Capabilities   []string
	Holder         TaskHolder
	// ProjectSlug is the project the daemon registers under; empty
	// string means the agent is registering into the server-wide
	// global pool (not bound to any one project).
	ProjectSlug string
	// Idle reports the agent's initial work-loop state. False = the
	// caller will start claiming work after the WS comes up. True =
	// the agent stays passive, controllable from the UI.
	Idle bool

	// ControlCh receives pause/resume commands forwarded from the WS channel.
	// If nil, control messages are logged but not forwarded. Callers should
	// create a buffered channel (cap ≥ 4) before calling Run/RunForever.
	ControlCh chan ControlMsg

	// PauseRenewer, if non-nil, is invoked on PauseRenewInterval while the
	// agent is paused, so the active task lease stays alive without spawning
	// new LLM turns. Stops on "resume" or daemon shutdown.
	PauseRenewer LeaseRenewer

	// PauseRenewInterval overrides the default hold-loop tick (5m). Tests
	// inject a short interval to observe the renewer being called.
	PauseRenewInterval time.Duration

	// StateFile, if non-empty, gets a one-line "paused\n<sid>\n<issue>\n"
	// breadcrumb written on pause and removed on resume. A crashed/restarted
	// daemon can read this file to know it was paused.
	StateFile string

	// EventLog, when non-nil, receives structured lifecycle events the
	// daemon would otherwise log via log.Printf — connected, disconnected,
	// control received, pause/resume/drain, dial retry. Wiring this lets
	// the host's tracelog see the daemon's chain without agentdaemon
	// importing tracelog (no back-coupling). nil disables emission;
	// log.Printf still fires for human-readable stderr output.
	EventLog func(name string, kv ...any)

	// PingInterval overrides the heartbeat ping cadence (default 25s). Tests
	// inject a sub-second value to observe pings without slowing the suite.
	// Production should leave this zero.
	PingInterval time.Duration

	// PingTimeout overrides how long a single ping waits for pong (default
	// 10s). Tests pair this with PingInterval to fail fast.
	PingTimeout time.Duration

	// now is injectable for tests; defaults to time.Now.
	now func() time.Time

	// connMu guards wsConn. Separate from mu so WS sends don't contend with
	// pause/resume state updates.
	connMu sync.Mutex
	wsConn *websocket.Conn

	// paused state, guarded by mu. Set on "pause", cleared on "resume".
	// pauseHoldStop signals the hold goroutine to exit; pauseHoldDone is
	// closed when the hold goroutine returns (used by tests for synchronization).
	mu            sync.Mutex
	paused        bool
	pausedSID     string
	pausedIssueID string
	pauseHoldStop chan struct{}
	pauseHoldDone chan struct{}
}

// emit calls EventLog when set; no-op otherwise. Lets callers wire a
// structured logger (tracelog) without making agentdaemon import it.
func (d *Daemon) emit(name string, kv ...any) {
	if d.EventLog != nil {
		d.EventLog(name, kv...)
	}
}

// Run dials the server, sends the registration handshake, and blocks until the
// connection closes or ctx is cancelled. On ctx cancellation it drains the
// current task (if any), sends a clean close frame, and returns nil. Any
// connection error is returned so RunForever can decide whether to retry.
func (d *Daemon) Run(ctx context.Context) error {
	wsURL := toWSURL(d.ServerURL) + "/api/agents/connect"
	opts := &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			"X-Api-Key": {d.APIKey},
		},
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	// Send handshake. ProjectSlug + Idle are optional fields the server
	// reads (and ignores when absent for older daemons that didn't send
	// them).
	handshake, _ := json.Marshal(map[string]any{
		"agent_id":        d.AgentID,
		"hostname":        d.Hostname,
		"pid":             d.Pid,
		"capabilities":    d.Capabilities,
		"worktree_path":   d.WorktreePath,
		"worktree_branch": d.WorktreeBranch,
		"project_slug":    d.ProjectSlug,
		"idle":            d.Idle,
	})
	if err := conn.Write(ctx, websocket.MessageText, handshake); err != nil {
		return fmt.Errorf("write handshake: %w", err)
	}

	// Expect "registered" acknowledgement.
	_, _, err = conn.Read(ctx)
	if err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	log.Printf("registered with server as %s", d.AgentID)
	d.emit("daemon.connected", "agent_id", d.AgentID, "server", d.ServerURL)

	d.connMu.Lock()
	d.wsConn = conn
	d.connMu.Unlock()
	defer func() {
		d.connMu.Lock()
		d.wsConn = nil
		d.connMu.Unlock()
	}()

	// Pump reads using a background context so parent-ctx cancellation does not
	// cause nhooyr.io/websocket to tear down the connection before the close
	// handshake. conn.Close will signal the goroutine to stop by closing the
	// underlying connection.
	readErr := make(chan error, 1)
	go func() {
		for {
			_, data, err := conn.Read(context.Background())
			if err != nil {
				readErr <- err
				return
			}
			d.handleIncoming(data)
		}
	}()

	// Heartbeat: periodically ping the server so idle proxies don't drop the
	// connection. On ping failure we close the conn — the read goroutine will
	// observe the close and propagate the error to RunForever for retry.
	pingCtx, stopPings := context.WithCancel(context.Background())
	defer stopPings()
	go d.pingLoop(pingCtx, conn)

	select {
	case err := <-readErr:
		return err
	case <-ctx.Done():
	}

	// ctx cancelled — stop any pause hold loop, drain current task, close cleanly.
	d.stopPauseHold()

	holder := d.Holder
	if holder == nil {
		holder = NoopHolder()
	}
	if task := holder.CurrentTask(); task != nil {
		log.Printf("draining: waiting for task %s", task.ID)
		drainCtx, cancel := context.WithTimeout(context.Background(), drainTimeout)
		defer cancel()
		if err := holder.WaitForCompletion(drainCtx); err != nil {
			log.Printf("drain timeout for task %s: %v", task.ID, err)
		}
	}

	// Send a clean close frame; nhooyr.io/websocket's Close acquires the read
	// mutex internally so the read goroutine will exit once the server acks.
	conn.Close(websocket.StatusNormalClosure, "shutdown") //nolint:errcheck
	return nil
}

// RunForever wraps Run in a retry loop with exponential backoff. Returns nil
// only when ctx is cancelled (i.e. intentional shutdown).
func (d *Daemon) RunForever(ctx context.Context) error {
	now := d.now
	if now == nil {
		now = time.Now
	}

	backoff := backoffInitial
	attempt := 0

	for {
		if ctx.Err() != nil {
			return nil
		}

		start := now()
		err := d.Run(ctx)

		if ctx.Err() != nil {
			return nil
		}

		// Connection that lived long enough resets the backoff counter.
		if now().Sub(start) >= backoffResetAge {
			backoff = backoffInitial
			attempt = 0
		} else {
			attempt++
		}

		if err != nil {
			log.Printf("connection lost (attempt %d): %v — retrying in %s", attempt, err, backoff)
			d.emit("daemon.disconnected", "agent_id", d.AgentID, "attempt", attempt, "err", err.Error(), "retry_in_seconds", int(backoff.Seconds()))
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > backoffMax {
			backoff = backoffMax
		}
	}
}

// pingLoop sends periodic application-level WS pings until ctx is cancelled or
// a ping fails. On failure it closes the conn so the read pump errors out and
// RunForever can reconnect. Library handles inbound pings (auto-pong) on its
// own — this loop only generates outbound traffic.
func (d *Daemon) pingLoop(ctx context.Context, conn *websocket.Conn) {
	interval := d.PingInterval
	if interval <= 0 {
		interval = pingInterval
	}
	timeout := d.PingTimeout
	if timeout <= 0 {
		timeout = pingTimeout
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pctx, cancel := context.WithTimeout(ctx, timeout)
			err := conn.Ping(pctx)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return // shutting down — not a real failure
				}
				log.Printf("ping failed: %v — closing connection", err)
				d.emit("daemon.ping_failed", "agent_id", d.AgentID, "err", err.Error())
				// CloseNow skips the close handshake — pointless when the peer
				// stopped responding. This unblocks the read goroutine fast so
				// RunForever can reconnect.
				conn.CloseNow() //nolint:errcheck
				return
			}
		}
	}
}

// handleIncoming processes one server→agent control message. Unknown types are
// silently ignored so future message types don't break older daemons.
func (d *Daemon) handleIncoming(data []byte) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "pause":
		holder := d.Holder
		if holder == nil {
			holder = NoopHolder()
		}
		sid, issueID := "", ""
		if task := holder.CurrentTask(); task != nil {
			sid = task.SessionID
			issueID = task.IssueID
		}
		d.mu.Lock()
		if d.paused {
			// Already paused — ignore duplicate pause to avoid stacking hold loops.
			d.mu.Unlock()
			return
		}
		d.paused = true
		d.pausedSID = sid
		d.pausedIssueID = issueID
		stop := make(chan struct{})
		done := make(chan struct{})
		d.pauseHoldStop = stop
		d.pauseHoldDone = done
		d.mu.Unlock()

		log.Printf("agent paused — holding lease (session=%s issue=%s)", sid, issueID)
		d.emit("daemon.control", "type", "pause", "session_id", sid, "issue_id", issueID)
		d.writePauseState(sid, issueID)
		go d.runPauseHold(stop, done, sid, issueID)

		if d.ControlCh != nil {
			select {
			case d.ControlCh <- ControlMsg{Type: "pause", SessionID: sid, IssueID: issueID}:
			default:
			}
		}

	case "resume":
		d.mu.Lock()
		if !d.paused {
			d.mu.Unlock()
			return
		}
		sid := d.pausedSID
		issueID := d.pausedIssueID
		stop := d.pauseHoldStop
		d.paused = false
		d.pausedSID = ""
		d.pausedIssueID = ""
		d.pauseHoldStop = nil
		d.pauseHoldDone = nil
		d.mu.Unlock()

		if stop != nil {
			close(stop)
		}
		d.removePauseState()
		log.Printf("agent resumed (session=%s issue=%s)", sid, issueID)
		d.emit("daemon.control", "type", "resume", "session_id", sid, "issue_id", issueID)
		if d.ControlCh != nil {
			select {
			case d.ControlCh <- ControlMsg{Type: "resume", SessionID: sid, IssueID: issueID}:
			default:
			}
		}

	case "drain":
		// Drain is informational at the daemon level — the consumer (loop)
		// reads it from ControlCh and stops claiming new work, then the
		// in-flight session releases on its own. The daemon itself stays
		// connected so the operator can still see the drain in progress;
		// the loop's exit cancels the daemon's parent ctx for clean close.
		holder := d.Holder
		if holder == nil {
			holder = NoopHolder()
		}
		sid, issueID := "", ""
		if task := holder.CurrentTask(); task != nil {
			sid = task.SessionID
			issueID = task.IssueID
		}
		log.Printf("agent draining (session=%s issue=%s)", sid, issueID)
		d.emit("daemon.control", "type", "drain", "session_id", sid, "issue_id", issueID)
		if d.ControlCh != nil {
			select {
			case d.ControlCh <- ControlMsg{Type: "drain", SessionID: sid, IssueID: issueID}:
			default:
			}
		}
	}
}

// runPauseHold renews the task lease at PauseRenewInterval until stop is closed
// (resume or shutdown). Closes done on exit so callers can wait for teardown.
// If PauseRenewer is nil, the goroutine still runs (so resume can join it via
// done) but performs no work each tick.
func (d *Daemon) runPauseHold(stop <-chan struct{}, done chan<- struct{}, sid, issueID string) {
	defer close(done)
	interval := d.PauseRenewInterval
	if interval <= 0 {
		interval = defaultPauseRenewInt
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			if d.PauseRenewer == nil {
				continue
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			d.PauseRenewer.Renew(ctx, sid, issueID)
			cancel()
		}
	}
}

// stopPauseHold signals any active pause hold loop to exit. Idempotent.
func (d *Daemon) stopPauseHold() {
	d.mu.Lock()
	stop := d.pauseHoldStop
	d.pauseHoldStop = nil
	d.pauseHoldDone = nil
	d.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (d *Daemon) writePauseState(sid, issueID string) {
	if d.StateFile == "" {
		return
	}
	body := fmt.Sprintf("paused\n%s\n%s\n", sid, issueID)
	if err := os.WriteFile(d.StateFile, []byte(body), 0o644); err != nil {
		log.Printf("pause: write state file %s: %v", d.StateFile, err)
	}
}

func (d *Daemon) removePauseState() {
	if d.StateFile == "" {
		return
	}
	if err := os.Remove(d.StateFile); err != nil && !os.IsNotExist(err) {
		log.Printf("pause: remove state file %s: %v", d.StateFile, err)
	}
}

// PauseSnapshot reports whether the daemon is currently paused, plus the
// session id and issue id captured at pause time. Safe to call from any
// goroutine. A task loop polls this between LLM turns to gate spawning new
// ones without exclusively consuming ControlCh (spec 180).
func (d *Daemon) PauseSnapshot() (paused bool, sessionID, issueID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.paused, d.pausedSID, d.pausedIssueID
}

// SendMsg sends a raw JSON message over the active WS connection. Returns an
// error if the daemon is not connected. Implements AgentConn.
func (d *Daemon) SendMsg(ctx context.Context, data []byte) error {
	d.connMu.Lock()
	c := d.wsConn
	d.connMu.Unlock()
	if c == nil {
		return fmt.Errorf("not connected")
	}
	return c.Write(ctx, websocket.MessageText, data)
}

// toWSURL converts an http/https base URL to ws/wss.
func toWSURL(base string) string {
	base = strings.TrimRight(base, "/")
	switch {
	case strings.HasPrefix(base, "https://"):
		return "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		return "ws://" + strings.TrimPrefix(base, "http://")
	default:
		return base
	}
}
