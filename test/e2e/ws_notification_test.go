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

	// Open WebSocket subscription.
	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/api/ws/subscribe?token=" + signResp.Token
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"X-Api-Key": {srv.AdminToken}},
	})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Create an issue — this should publish an issue.created event.
	var issue struct {
		ID int `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "ws test issue", "context": "trigger notification", "auto_ready": true},
		&issue))

	// Read the WebSocket message.
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if msgType != websocket.MessageText {
		t.Fatalf("want text message, got %v", msgType)
	}

	var msg struct {
		Channel string          `json:"channel"`
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal ws message: %v", err)
	}

	if msg.Channel != channel {
		t.Errorf("channel: want %q got %q", channel, msg.Channel)
	}
	if msg.Type != "issue.created" {
		t.Errorf("type: want %q got %q", "issue.created", msg.Type)
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
