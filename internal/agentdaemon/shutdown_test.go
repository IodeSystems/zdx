package agentdaemon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
)

// echoServer upgrades to WebSocket, reads the handshake, acks it, then holds.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		bctx := context.Background()
		_, _, err = conn.Read(bctx)
		if err != nil {
			t.Logf("read handshake: %v", err)
			return
		}
		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now()})
		if err := conn.Write(bctx, websocket.MessageText, ack); err != nil {
			t.Logf("write ack: %v", err)
			return
		}
		// Hold until client closes.
		for {
			if _, _, err := conn.Read(bctx); err != nil {
				return
			}
		}
	}))
}

func newDaemon(serverURL string) *agentdaemon.Daemon {
	return &agentdaemon.Daemon{
		ServerURL:    serverURL,
		AgentID:      "test-agent",
		Capabilities: []string{"test"},
		Holder:       agentdaemon.NoopHolder(),
	}
}

// TestRunForeverNoop: no-op holder, cancelling ctx returns nil within ~100ms.
func TestRunForeverNoop(t *testing.T) {
	srv := echoServer(t)
	defer srv.Close()

	d := newDaemon(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("RunForever did not return within 500ms after ctx cancel")
	}
}

// blockingHolder blocks WaitForCompletion for the given duration.
type blockingHolder struct {
	block time.Duration
}

func (h *blockingHolder) CurrentTask() *agentdaemon.RunningTask {
	return &agentdaemon.RunningTask{ID: "TK-test", Started: time.Now()}
}
func (h *blockingHolder) WaitForCompletion(ctx context.Context) error {
	select {
	case <-time.After(h.block):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TestRunForeverDrain: holder with 200ms task, cancel waits before returning,
// close frame uses StatusNormalClosure.
func TestRunForeverDrain(t *testing.T) {
	closeStatuses := make(chan websocket.StatusCode, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		// Use background ctx so request ctx cancellation doesn't mask the close frame.
		bctx := context.Background()
		_, _, err = conn.Read(bctx)
		if err != nil {
			return
		}
		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now()})
		_ = conn.Write(bctx, websocket.MessageText, ack)
		for {
			_, _, err := conn.Read(bctx)
			if err != nil {
				closeStatuses <- websocket.CloseStatus(err)
				return
			}
		}
	}))
	defer srv.Close()

	d := &agentdaemon.Daemon{
		ServerURL:    srv.URL,
		AgentID:      "test-drain",
		Capabilities: []string{"test"},
		Holder:       &blockingHolder{block: 200 * time.Millisecond},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if elapsed < 150*time.Millisecond {
			t.Fatalf("expected drain wait ≥150ms, got %s", elapsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("RunForever did not return within 2s after ctx cancel with drain")
	}

	select {
	case status := <-closeStatuses:
		if status != websocket.StatusNormalClosure {
			t.Fatalf("expected StatusNormalClosure, got %v", status)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("server did not receive close frame")
	}
}
