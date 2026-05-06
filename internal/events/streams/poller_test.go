package streams_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/events/streams"
)

type recordingHandler struct {
	tt          string
	body        string
	fetchCalls  int32
	applyCalls  int32
	lastEventID int64
	lastBody    string
	lastTarget  string
}

func (r *recordingHandler) TargetType() string { return r.tt }

func (r *recordingHandler) FetchBody(_ context.Context, _ *cli.Client, _, targetID string) (string, error) {
	atomic.AddInt32(&r.fetchCalls, 1)
	r.lastTarget = targetID
	return r.body, nil
}

func (r *recordingHandler) ApplyAddressed(_ context.Context, _ *cli.Client, _, targetID string, addressingEventID int64, revisedBody string) error {
	atomic.AddInt32(&r.applyCalls, 1)
	r.lastEventID = addressingEventID
	r.lastBody = revisedBody
	_ = targetID
	return nil
}

// TestRunOnce_DispatchesToHandler wires a fake StreamHandler through the
// generic poller against a real cli.Client backed by httptest.Server. It
// asserts the poller (a) lists stale streams filtered to the requested
// target_type, (b) lists events for the stream, (c) calls FetchBody for
// the body, (d) hands the body+pending events to the injected classifier,
// (e) calls ApplyAddressed on the addressed verdict, and (f) posts the
// verdict back.
func TestRunOnce_DispatchesToHandler(t *testing.T) {
	const targetType = "fake-runonce-target"
	const targetID = "FK-1"
	const eventID = int64(42)

	streams.Register(&recordingHandler{tt: targetType, body: "canonical body"})

	var listStaleHits, listEventsHits, verdictHits int32
	var verdictPayload map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/api/streams/stale", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listStaleHits, 1)
		if got := r.URL.Query().Get("target_type"); got != targetType {
			t.Errorf("/api/streams/stale target_type=%q, want %q", got, targetType)
		}
		writeJSON(w, dxclient.ListStaleStreamsResponse{
			Streams: &[]dxclient.StreamItem{{
				Id:         1,
				TargetType: targetType,
				TargetId:   targetID,
			}},
		})
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&listEventsHits, 1)
		if got := r.URL.Query().Get("target_type"); got != targetType {
			t.Errorf("/api/events target_type=%q, want %q", got, targetType)
		}
		if got := r.URL.Query().Get("target_id"); got != targetID {
			t.Errorf("/api/events target_id=%q, want %q", got, targetID)
		}
		writeJSON(w, dxclient.ListEventsResponse{
			Events: &[]dxclient.EventItem{{
				Id:         eventID,
				Author:     "alice",
				AuthorKind: "user",
				EventType:  "comment",
				TargetType: targetType,
				TargetId:   targetID,
				DetailJson: map[string]any{"body": "please tweak X"},
				// AgentProcessResult left nil — pending.
			}, {
				Id:                 eventID + 1,
				Author:             "bot",
				AuthorKind:         "agent",
				EventType:          "comment",
				TargetType:         targetType,
				TargetId:           targetID,
				DetailJson:         map[string]any{"body": "agent message — must be filtered out"},
				AgentProcessResult: nil,
			}, {
				Id:                 eventID + 2,
				Author:             "carol",
				AuthorKind:         "user",
				EventType:          "comment",
				TargetType:         targetType,
				TargetId:           targetID,
				DetailJson:         map[string]any{"body": "already-processed user comment"},
				AgentProcessResult: map[string]any{"verdict": "addressed"},
			}},
		})
	})
	mux.HandleFunc("/api/events/42/verdict", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&verdictHits, 1)
		if r.Method != http.MethodPost {
			t.Errorf("verdict method=%q, want POST", r.Method)
		}
		var body struct {
			Slug               string         `json:"slug"`
			AgentProcessResult map[string]any `json:"agent_process_result"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode verdict body: %v", err)
		}
		verdictPayload = body.AgentProcessResult
		writeJSON(w, dxclient.EventItem{Id: eventID})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := cli.NewClientWithSlug(srv.URL, "test-token", "test-slug")

	classifier := func(_ context.Context, body string, pending []dxclient.EventItem) ([]streams.CommentVerdict, error) {
		if body != "canonical body" {
			t.Errorf("classifier body=%q, want %q", body, "canonical body")
		}
		if len(pending) != 1 {
			t.Fatalf("classifier got %d pending events, want 1 (user-authored, no AgentProcessResult)", len(pending))
		}
		if pending[0].Id != eventID {
			t.Errorf("classifier pending[0].Id=%d, want %d", pending[0].Id, eventID)
		}
		return []streams.CommentVerdict{{
			EventID:     eventID,
			Verdict:     "addressed",
			RevisedBody: "rewritten body",
		}}, nil
	}

	sum, err := streams.RunOnce(context.Background(), c, "test-slug", []string{targetType}, classifier)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if sum.Streams != 1 || sum.Processed != 1 || sum.Revised != 1 || sum.Skipped != 0 {
		t.Errorf("Summary = %+v, want {Streams:1 Processed:1 Revised:1 Skipped:0}", sum)
	}
	if listStaleHits != 1 {
		t.Errorf("/api/streams/stale hits=%d, want 1", listStaleHits)
	}
	if listEventsHits != 1 {
		t.Errorf("/api/events hits=%d, want 1", listEventsHits)
	}
	if verdictHits != 1 {
		t.Errorf("/api/events/42/verdict hits=%d, want 1", verdictHits)
	}
	if got, _ := verdictPayload["verdict"].(string); got != "addressed" {
		t.Errorf("verdict payload verdict=%q, want addressed", got)
	}
	if got, _ := verdictPayload["revised_body"].(string); got != "rewritten body" {
		t.Errorf("verdict payload revised_body=%q, want %q", got, "rewritten body")
	}
}

// TestRunOnce_UnknownTargetTypeErrors covers the explicit-list path: if a
// caller asks for a target_type that has no registered handler, RunOnce
// surfaces ErrUnknownTargetType rather than silently no-op'ing.
func TestRunOnce_UnknownTargetTypeErrors(t *testing.T) {
	c := cli.NewClientWithSlug("http://127.0.0.1:0", "", "test-slug")
	_, err := streams.RunOnce(context.Background(), c, "test-slug", []string{"definitely-not-registered"}, nil)
	if err == nil {
		t.Fatal("expected error for unknown target_type, got nil")
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
