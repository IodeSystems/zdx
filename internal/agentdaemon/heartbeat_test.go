package agentdaemon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
)

// silentServer accepts the WS upgrade, completes the handshake, then stops
// reading. Because nhooyr.io/websocket auto-pongs from inside conn.Read, a
// server that never reads also never pongs — making this a faithful proxy for
// the production failure mode (intermediate proxy drops idle ws frames).
func silentServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		bctx := context.Background()
		if _, _, err := conn.Read(bctx); err != nil {
			return
		}
		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now()})
		if err := conn.Write(bctx, websocket.MessageText, ack); err != nil {
			return
		}
		// Block forever without reading. The handler returning would close the
		// conn, so we hold the goroutine until the test tears the server down.
		<-r.Context().Done()
	}))
}

// TestHeartbeatPingFailureClosesConn: with a server that never reads (so never
// pongs), the daemon's ping must time out and force a disconnect, surfacing
// daemon.ping_failed. Validates the heartbeat actually triggers reconnect
// instead of letting an idle/half-dead conn linger.
func TestHeartbeatPingFailureClosesConn(t *testing.T) {
	srv := silentServer(t)
	defer srv.Close()

	var (
		mu       sync.Mutex
		gotPing  bool
		gotClose bool
	)
	d := &agentdaemon.Daemon{
		ServerURL:    srv.URL,
		AgentID:      "agent-heartbeat",
		PingInterval: 50 * time.Millisecond,
		PingTimeout:  200 * time.Millisecond,
		EventLog: func(name string, _ ...any) {
			mu.Lock()
			defer mu.Unlock()
			switch name {
			case "daemon.ping_failed":
				gotPing = true
			case "daemon.disconnected":
				gotClose = true
			}
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run (single attempt): expect it to return with an error once the ping
	// loop closes the conn.
	err := d.Run(ctx)
	if err == nil {
		t.Fatalf("expected Run to return non-nil error after ping failure")
	}

	mu.Lock()
	defer mu.Unlock()
	if !gotPing {
		t.Errorf("expected daemon.ping_failed event")
	}
	_ = gotClose // disconnect emission lives in RunForever, not Run; assert on ping_failed only
}
