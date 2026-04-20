package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

func TestWSNotificationDelivery(t *testing.T) {
	requiresAPI(t)

	slug := "e2e-ws-notify"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E WS Notify"}, nil)

	channel := "project:" + slug + ":issues"

	// Sign a WS token for the issues channel.
	var signResp struct {
		Token string `json:"token"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/ws/sign",
		map[string]string{"channel": channel}, &signResp))
	if signResp.Token == "" {
		t.Fatal("sign returned empty token")
	}

	// Open the single multiplexed WebSocket (no query-string token — auth
	// happens via an in-protocol subscribe message).
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send a subscribe message with the signed token.
	subPayload, _ := json.Marshal(map[string]string{
		"type":  "subscribe",
		"token": signResp.Token,
	})
	if err := conn.Write(ctx, websocket.MessageText, subPayload); err != nil {
		t.Fatalf("ws write subscribe: %v", err)
	}

	// Expect a {"type":"subscribed","channel":...} ack first.
	_, ackData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read subscribed ack: %v", err)
	}
	var ack struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(ackData, &ack); err != nil {
		t.Fatalf("unmarshal ack: %v", err)
	}
	if ack.Type != "subscribed" {
		t.Fatalf("want subscribed ack, got type=%q message=%q", ack.Type, ack.Message)
	}
	if ack.Channel != channel {
		t.Errorf("ack channel: want %q got %q", channel, ack.Channel)
	}

	// Create an issue — this should publish an issue.created event.
	var issue struct {
		ID int `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "ws test issue", "context": "trigger notification", "auto_ready": true},
		&issue))

	// Read the relayed message envelope.
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("want text message, got %v", msgType)
	}

	var msg struct {
		Type    string          `json:"type"`
		Channel string          `json:"channel"`
		Event   string          `json:"event"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal ws message: %v", err)
	}

	if msg.Type != "message" {
		t.Errorf("envelope type: want %q got %q", "message", msg.Type)
	}
	if msg.Channel != channel {
		t.Errorf("channel: want %q got %q", channel, msg.Channel)
	}
	if msg.Event != "issue.created" {
		t.Errorf("event: want %q got %q", "issue.created", msg.Event)
	}

	var payload struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.Title != "ws test issue" {
		t.Errorf("payload title: want %q got %q", "ws test issue", payload.Title)
	}
}
