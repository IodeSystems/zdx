package agentdaemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"nhooyr.io/websocket"
)

const (
	drainTimeout    = 10 * time.Minute
	backoffInitial  = 1 * time.Second
	backoffMax      = 30 * time.Second
	backoffResetAge = 10 * time.Second // connection survived this long → reset backoff
)

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

	// now is injectable for tests; defaults to time.Now.
	now func() time.Time
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

	// Send handshake.
	handshake, _ := json.Marshal(map[string]any{
		"agent_id":        d.AgentID,
		"hostname":        d.Hostname,
		"pid":             d.Pid,
		"capabilities":    d.Capabilities,
		"worktree_path":   d.WorktreePath,
		"worktree_branch": d.WorktreeBranch,
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

	// Pump reads using a background context so parent-ctx cancellation does not
	// cause nhooyr.io/websocket to tear down the connection before the close
	// handshake. conn.Close will signal the goroutine to stop by closing the
	// underlying connection.
	readErr := make(chan error, 1)
	go func() {
		for {
			_, _, err := conn.Read(context.Background())
			if err != nil {
				readErr <- err
				return
			}
		}
	}()

	select {
	case err := <-readErr:
		return err
	case <-ctx.Done():
	}

	// ctx cancelled — drain current task then close cleanly.
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
