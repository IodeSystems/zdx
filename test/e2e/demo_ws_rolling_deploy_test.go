package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/devserver"
)

// TestDemoWS_RollingDeployFanout verifies spec 107: during a rolling deploy with both
// slots running, WS events published via valkey are delivered to subscribers on either slot
// so clients connected during a slot transition do not miss events.
//
// Walk-through:
//  1. Boot two server slots (slot-a, slot-b) sharing one Valkey instance and one Postgres.
//  2. Subscribe a WS client to slot-a; publish an event via slot-b's HTTP API. Assert delivery on slot-a.
//  3. Flip: subscribe to slot-b, publish from slot-a's HTTP API. Assert delivery on slot-b.
func TestDemoWS_RollingDeployFanout(t *testing.T) {
	valkeyAddr := os.Getenv("ZDX_VALKEY_ADDR")
	if valkeyAddr == "" {
		t.Skip("ZDX_VALKEY_ADDR not set; skipping rolling-deploy demo")
	}
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable (external runner mode); skipping rolling-deploy demo")
	}

	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_ws_rolling_deploy_test.go", Note: "rolling-deploy WS fanout demo (spec 107)"},
		{FilePath: "internal/ws/valkey.go", Note: "ValkeyBroker: distributed pub/sub across server slots"},
		{FilePath: "internal/server/ws_handlers.go", Note: "WS subscribe/relay path"},
	})

	// Start slot-a (bootstrapped — owns the admin token).
	slotA, cleanA, err := devserver.Start(devserver.Options{
		DSN:        srv.DSN,
		ValkeyAddr: valkeyAddr,
	})
	if err != nil {
		t.Fatalf("slot-a start: %v", err)
	}
	t.Cleanup(cleanA)

	// Start slot-b sharing the same DB and Valkey. Skip bootstrap — the admin
	// token from slot-a is valid on both servers since they share the DB.
	slotB, cleanB, err := devserver.Start(devserver.Options{
		DSN:           srv.DSN,
		ValkeyAddr:    valkeyAddr,
		SkipBootstrap: true,
		AdminToken:    slotA.AdminToken,
	})
	if err != nil {
		t.Fatalf("slot-b start: %v", err)
	}
	t.Cleanup(cleanB)

	// Allow Valkey PSubscribe goroutines to establish before publishing.
	time.Sleep(100 * time.Millisecond)

	// Create a project shared across both slots.
	slug := "e2e-rolling-deploy"
	rollingDo(t, slotA, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Rolling Deploy Demo"}, nil)
	channel := "project:" + slug + ":issues"

	// ── Pass 1: subscribe on slot-a, publish from slot-b ────────────────────
	t.Run("subscribe-a-publish-b", func(t *testing.T) {
		conn := rollingSubscribeWS(t, slotA, channel)
		t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })

		rollingDo(t, slotB, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug": slug, "title": "event from slot-b",
				"context": "rolling deploy test", "auto_ready": true,
			}, nil)

		rollingAssertEvent(t, conn, "issue.created")
	})

	// ── Pass 2: subscribe on slot-b, publish from slot-a ────────────────────
	t.Run("subscribe-b-publish-a", func(t *testing.T) {
		conn := rollingSubscribeWS(t, slotB, channel)
		t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })

		rollingDo(t, slotA, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug": slug, "title": "event from slot-a",
				"context": "rolling deploy test", "auto_ready": true,
			}, nil)

		rollingAssertEvent(t, conn, "issue.created")
	})
}

// rollingDo makes an authenticated API call to a specific slot handle.
func rollingDo(t *testing.T, h *devserver.Handle, method, path string, body, out any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(method, h.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", h.AdminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if out != nil && resp.StatusCode < 400 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp
}

// rollingSubscribeWS signs a WS token on h, dials the WS endpoint, sends the
// subscribe message, and returns the conn after the "subscribed" ack.
func rollingSubscribeWS(t *testing.T, h *devserver.Handle, channel string) *websocket.Conn {
	t.Helper()

	var signResp struct {
		Token string `json:"token"`
	}
	r := rollingDo(t, h, http.MethodPost, "/api/ws/sign",
		map[string]string{"channel": channel}, &signResp)
	if r.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(r.Body)
		t.Fatalf("sign token on %s: HTTP %d: %s", h.URL, r.StatusCode, body)
	}

	wsURL := strings.Replace(h.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial %s: %v", h.URL, err)
	}

	subMsg, _ := json.Marshal(map[string]string{
		"type":  "subscribe",
		"token": signResp.Token,
	})
	if err := conn.Write(ctx, websocket.MessageText, subMsg); err != nil {
		conn.Close(websocket.StatusInternalError, "")
		t.Fatalf("ws subscribe write: %v", err)
	}

	_, ackData, err := conn.Read(ctx)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "")
		t.Fatalf("ws read ack: %v", err)
	}
	var ack struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	json.Unmarshal(ackData, &ack) //nolint:errcheck
	if ack.Type != "subscribed" {
		conn.Close(websocket.StatusInternalError, "")
		t.Fatalf("want subscribed ack, got type=%q message=%q", ack.Type, ack.Message)
	}

	return conn
}

// rollingAssertEvent reads the next WS message and asserts it carries wantEvent.
func rollingAssertEvent(t *testing.T, conn *websocket.Conn, wantEvent string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	var env struct {
		Type  string `json:"type"`
		Event string `json:"event"`
	}
	json.Unmarshal(data, &env) //nolint:errcheck
	if env.Type != "message" {
		t.Errorf("want envelope type=message, got %q", env.Type)
	}
	if env.Event != wantEvent {
		t.Errorf("want event %q, got %q", wantEvent, env.Event)
	}
}
