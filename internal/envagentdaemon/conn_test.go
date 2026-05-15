package envagentdaemon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/envagentdaemon"
)

// TestHandshakeAndHeartbeat exercises the daemon's connect path against a
// fake WS server: handshake JSON shape, the registered ack roundtrip, and
// at least one heartbeat frame at sub-second cadence.
func TestHandshakeAndHeartbeat(t *testing.T) {
	type frame struct {
		raw []byte
		err error
	}
	gotHandshake := make(chan frame, 1)
	gotHeartbeat := make(chan frame, 4)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dx/env-agents/connect" {
			http.Error(w, "wrong path: "+r.URL.Path, http.StatusNotFound)
			return
		}
		if got := r.Header.Get("X-Api-Key"); got != "secret-key" {
			http.Error(w, "bad api key", http.StatusUnauthorized)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")

		bctx := context.Background()
		// Handshake.
		_, data, err := conn.Read(bctx)
		gotHandshake <- frame{raw: data, err: err}
		if err != nil {
			return
		}
		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now()})
		if err := conn.Write(bctx, websocket.MessageText, ack); err != nil {
			return
		}
		// Subsequent frames are heartbeats; forward until the client closes.
		for {
			_, data, err := conn.Read(bctx)
			if err != nil {
				return
			}
			select {
			case gotHeartbeat <- frame{raw: data}:
			default:
			}
		}
	}))
	defer srv.Close()

	d := &envagentdaemon.Daemon{
		ServerURL:         srv.URL,
		EnvName:           "production",
		AgentID:           "envd-test-1",
		APIKey:            "secret-key",
		Version:           "test-build",
		OS:                "linux",
		HeartbeatInterval: 50 * time.Millisecond,
		// Keep the ping loop quiet during the test — it's exercised by its
		// own dedicated test in agentdaemon and reuses identical code here.
		PingInterval: 10 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runDone := make(chan error, 1)
	go func() { runDone <- d.Run(ctx) }()

	// Assert handshake shape.
	select {
	case f := <-gotHandshake:
		if f.err != nil {
			t.Fatalf("server read handshake: %v", f.err)
		}
		var msg map[string]any
		if err := json.Unmarshal(f.raw, &msg); err != nil {
			t.Fatalf("handshake not json: %v (raw=%q)", err, string(f.raw))
		}
		for _, k := range []string{"env_name", "agent_id", "version", "os", "deployed_commit", "uptime_secs", "hostname"} {
			if _, ok := msg[k]; !ok {
				t.Errorf("handshake missing %q (got %v)", k, msg)
			}
		}
		if msg["env_name"] != "production" {
			t.Errorf("env_name = %v, want production", msg["env_name"])
		}
		if msg["version"] != "test-build" {
			t.Errorf("version = %v, want test-build", msg["version"])
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server never received handshake")
	}

	// Assert at least one heartbeat with expected fields.
	select {
	case f := <-gotHeartbeat:
		var msg map[string]any
		if err := json.Unmarshal(f.raw, &msg); err != nil {
			t.Fatalf("heartbeat not json: %v (raw=%q)", err, string(f.raw))
		}
		if msg["type"] != "heartbeat" {
			t.Errorf("heartbeat type = %v, want heartbeat", msg["type"])
		}
		for _, k := range []string{"version", "os", "deployed_commit", "uptime_secs"} {
			if _, ok := msg[k]; !ok {
				t.Errorf("heartbeat missing %q (got %v)", k, msg)
			}
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no heartbeat received")
	}

	cancel()
	select {
	case <-runDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after ctx cancel")
	}
}

func TestReadDeployedCommit(t *testing.T) {
	if got := envagentdaemon.ReadDeployedCommit(""); got != "" {
		t.Errorf("empty path → %q, want \"\"", got)
	}
	if got := envagentdaemon.ReadDeployedCommit("/nonexistent/path/should/never/exist"); got != "" {
		t.Errorf("missing file → %q, want \"\" (errors swallowed)", got)
	}

	tmp := t.TempDir() + "/manifest"
	if err := os.WriteFile(tmp, []byte("  abc123\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := envagentdaemon.ReadDeployedCommit(tmp); got != "abc123" {
		t.Errorf("manifest read = %q, want abc123 (trimmed)", got)
	}
}
