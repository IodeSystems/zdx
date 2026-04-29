package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
)

// TestDevReviewEmitsTaskReviewedEvent covers spec 78 on feature dx-todo-tasks:
// given two done tasks, when POST /api/dx/todo/dev/review is called with
// verdict=approve and verdict=reject respectively, then a task.reviewed event
// is published on the project's task channel for each, with the verdict and a
// non-zero review_id in the payload.
func TestDevReviewEmitsTaskReviewedEvent(t *testing.T) {
	requiresAPI(t)

	d := NewApiDriver(t, "e2e-dev-review-event", "Dev Review Event")
	sc := Given(d).
		TriagedIssue("Review WS event coverage", "spec 78", 2).
		Task(0, "approve target").
		Task(0, "reject target").
		Build()
	approveID := sc.Tasks[0]
	rejectID := sc.Tasks[1]

	d.MarkTaskDone(approveID)
	d.MarkTaskDone(rejectID)

	channel := "project:" + d.Slug + ":tasks"

	// Sign + open the multiplexed WS, subscribe before posting reviews to
	// avoid races where the publish lands before the subscriber attaches.
	var signResp struct {
		Token string `json:"token"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/ws/sign",
		map[string]string{"channel": channel}, &signResp))
	if signResp.Token == "" {
		t.Fatal("sign returned empty token")
	}

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/ws"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{})
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	subPayload, _ := json.Marshal(map[string]string{
		"type":  "subscribe",
		"token": signResp.Token,
	})
	if err := conn.Write(ctx, websocket.MessageText, subPayload); err != nil {
		t.Fatalf("ws write subscribe: %v", err)
	}

	// {"type":"subscribed","channel":...} ack.
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

	// Review approve, then reject.
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/review",
		map[string]any{"slug": d.Slug, "id": approveID, "verdict": "approve"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/review",
		map[string]any{"slug": d.Slug, "id": rejectID, "verdict": "reject"}, nil))

	// Collect task.reviewed events keyed by task id.
	wantApproveID := fmt.Sprintf("TK-%d", approveID)
	wantRejectID := fmt.Sprintf("TK-%d", rejectID)
	got := map[string]string{} // task id -> verdict

	for len(got) < 2 {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("ws read (got %d/2 events): %v", len(got), err)
		}
		var msg struct {
			Type    string          `json:"type"`
			Channel string          `json:"channel"`
			Event   string          `json:"event"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}
		if msg.Type != "message" || msg.Channel != channel {
			continue
		}
		if msg.Event != "task.reviewed" {
			continue
		}
		var payload struct {
			ID       string `json:"id"`
			Verdict  string `json:"verdict"`
			ReviewID int32  `json:"review_id"`
		}
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		if payload.ReviewID == 0 {
			t.Errorf("task %s: review_id is zero in payload", payload.ID)
		}
		got[payload.ID] = payload.Verdict
	}

	if v := got[wantApproveID]; v != "approve" {
		t.Errorf("approve task: want verdict=%q, got %q", "approve", v)
	}
	if v := got[wantRejectID]; v != "reject" {
		t.Errorf("reject task: want verdict=%q, got %q", "reject", v)
	}
}
