// Package envagentdaemon is the env-side daemon counterpart to
// internal/agentdaemon. It holds a persistent WS to dx-server, identifies
// itself as the agent for a particular deploy environment (production,
// staging, …), and heartbeats so the server knows the env is reachable.
//
// Unlike internal/agentdaemon there is no in-flight LLM work to drain on
// shutdown, no pause/resume control surface, and no TaskHolder. Future
// issues (IS-1229 deploy package, IS-1230 routing, IS-1232 schema-dump)
// will attach server→envd command handlers; the read-loop has a default
// no-op switch to make those additions backward-compatible with older
// daemons.
package envagentdaemon

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
	defaultHeartbeatInterval = 15 * time.Second

	backoffInitial  = 1 * time.Second
	backoffMax      = 30 * time.Second
	backoffResetAge = 10 * time.Second

	// pingInterval / pingTimeout: same idle-proxy concern as
	// internal/agentdaemon — nginx default 60s, observed ~50s in prod;
	// ping at half the cutoff so jitter + one missed pong still survives.
	defaultPingInterval = 25 * time.Second
	defaultPingTimeout  = 10 * time.Second
)

// Daemon holds the configuration for a persistent env-agent connection.
type Daemon struct {
	ServerURL string
	EnvName   string
	AgentID   string
	APIKey    string

	// Version is the daemon's build version, surfaced to the server on
	// handshake + every heartbeat so operators can see which deploy host
	// is on which build.
	Version string
	// OS is runtime.GOOS at startup; reported alongside Version.
	OS string

	// ManifestPath, if non-empty, is read on every heartbeat to surface
	// the env's currently-deployed commit. Manifest format is owned by
	// IS-1229; for now we treat the file as opaque text (commit sha or
	// short manifest blob — trimmed of whitespace before sending).
	ManifestPath string

	// HeartbeatInterval overrides the heartbeat send cadence (default 15s).
	HeartbeatInterval time.Duration
	// PingInterval / PingTimeout override the application-level WS ping
	// cadence (defaults 25s / 10s). Tests inject sub-second values.
	PingInterval time.Duration
	PingTimeout  time.Duration

	// EventLog, when non-nil, receives structured lifecycle events the
	// daemon would otherwise log via log.Printf. Same pattern as
	// internal/agentdaemon — lets tracelog wire in without back-coupling.
	EventLog func(name string, kv ...any)

	// now is injectable for tests; defaults to time.Now.
	now func() time.Time

	startedAt time.Time

	connMu sync.Mutex
	wsConn *websocket.Conn
}

func (d *Daemon) emit(name string, kv ...any) {
	if d.EventLog != nil {
		d.EventLog(name, kv...)
	}
}

// Run dials the server, sends the registration handshake, and blocks until the
// connection closes or ctx is cancelled. On ctx cancellation it sends a clean
// close frame and returns nil. Any connection error is returned so RunForever
// can decide whether to retry.
func (d *Daemon) Run(ctx context.Context) error {
	if d.startedAt.IsZero() {
		d.startedAt = time.Now()
	}

	wsURL := toWSURL(d.ServerURL) + "/api/dx/env-agents/connect"
	opts := &websocket.DialOptions{
		HTTPHeader: map[string][]string{
			"X-Api-Key": {d.APIKey},
		},
	}

	conn, _, err := websocket.Dial(ctx, wsURL, opts)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	hostname, _ := os.Hostname()
	handshake, _ := json.Marshal(map[string]any{
		"env_name":        d.EnvName,
		"agent_id":        d.AgentID,
		"hostname":        hostname,
		"version":         d.Version,
		"os":              d.OS,
		"deployed_commit": ReadDeployedCommit(d.ManifestPath),
		"uptime_secs":     int64(0),
	})
	if err := conn.Write(ctx, websocket.MessageText, handshake); err != nil {
		return fmt.Errorf("write handshake: %w", err)
	}

	if _, _, err := conn.Read(ctx); err != nil {
		return fmt.Errorf("read ack: %w", err)
	}
	log.Printf("registered with server as env=%s agent=%s", d.EnvName, d.AgentID)
	d.emit("envd.connected", "env", d.EnvName, "agent_id", d.AgentID, "server", d.ServerURL)

	d.connMu.Lock()
	d.wsConn = conn
	d.connMu.Unlock()
	defer func() {
		d.connMu.Lock()
		d.wsConn = nil
		d.connMu.Unlock()
	}()

	// Read pump using a background context so parent-ctx cancellation
	// doesn't tear down the conn before the close handshake. conn.Close
	// signals the goroutine to stop by closing the underlying connection.
	// No message types are handled yet — future issues hook in here.
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

	bgCtx, stopLoops := context.WithCancel(context.Background())
	defer stopLoops()
	go d.heartbeatLoop(bgCtx, conn)
	go d.pingLoop(bgCtx, conn)

	select {
	case err := <-readErr:
		return err
	case <-ctx.Done():
	}

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

		if now().Sub(start) >= backoffResetAge {
			backoff = backoffInitial
			attempt = 0
		} else {
			attempt++
		}

		if err != nil {
			log.Printf("connection lost (attempt %d): %v — retrying in %s", attempt, err, backoff)
			d.emit("envd.disconnected", "env", d.EnvName, "agent_id", d.AgentID, "attempt", attempt, "err", err.Error(), "retry_in_seconds", int(backoff.Seconds()))
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

func (d *Daemon) heartbeatLoop(ctx context.Context, conn *websocket.Conn) {
	interval := d.HeartbeatInterval
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			payload, _ := json.Marshal(map[string]any{
				"type":            "heartbeat",
				"version":         d.Version,
				"os":              d.OS,
				"deployed_commit": ReadDeployedCommit(d.ManifestPath),
				"uptime_secs":     int64(time.Since(d.startedAt).Seconds()),
			})
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("heartbeat write failed: %v — closing connection", err)
				d.emit("envd.heartbeat_failed", "env", d.EnvName, "agent_id", d.AgentID, "err", err.Error())
				conn.CloseNow() //nolint:errcheck
				return
			}
		}
	}
}

func (d *Daemon) pingLoop(ctx context.Context, conn *websocket.Conn) {
	interval := d.PingInterval
	if interval <= 0 {
		interval = defaultPingInterval
	}
	timeout := d.PingTimeout
	if timeout <= 0 {
		timeout = defaultPingTimeout
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
					return
				}
				log.Printf("ping failed: %v — closing connection", err)
				d.emit("envd.ping_failed", "env", d.EnvName, "agent_id", d.AgentID, "err", err.Error())
				conn.CloseNow() //nolint:errcheck
				return
			}
		}
	}
}

// handleIncoming is the forward-compat switch for server→envd command
// messages. IS-1229 / IS-1230 / IS-1232 will add cases here. Today: silently
// ignore so older daemons don't die on a new message type.
func (d *Daemon) handleIncoming(data []byte) {
	var msg struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	switch msg.Type {
	default:
		// Unknown / not yet implemented; ignore.
	}
}

// ReadDeployedCommit reads manifestPath and returns its trimmed contents.
// Empty path → empty string with no error. Read errors are swallowed and
// return empty string: a missing/unreadable manifest just means "unknown
// deployed commit" — not a daemon-fatal condition. Manifest format is
// owned by IS-1229; this helper only requires that the file's contents
// (after TrimSpace) identify the deploy in some opaque way.
func ReadDeployedCommit(manifestPath string) string {
	if manifestPath == "" {
		return ""
	}
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
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
