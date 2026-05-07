package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/server/agentconn"
)

// e2eHolder reports a fixed running task so pause carries session+issue ids.
type e2eHolder struct {
	task *agentdaemon.RunningTask
}

func (h *e2eHolder) CurrentTask() *agentdaemon.RunningTask     { return h.task }
func (h *e2eHolder) WaitForCompletion(_ context.Context) error { return nil }

// buildE2EServer creates a real httptest.Server with both huma routes and the
// WS connect handler wired to the same chi mux — identical to prod wiring,
// minus the apiKeyMiddleware. The test stamps role=admin on ctx for the WS
// connect path so authorizeAgentRegister allows the global handshake; in
// prod this happens via apiKeyMiddleware against a real admin token.
func buildE2EServer(t *testing.T, registry *agentconn.Registry) (*httptest.Server, *Handler) {
	t.Helper()
	mux := chi.NewMux()
	api := humachi.New(mux, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{
		AgentCommander: registry,
	}}
	h.registerAgentRoutes(api)
	connectHandler := h.HandleAgentConnect(registry)
	mux.Get("/api/agents/connect", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), CtxUserRole, "admin")
		connectHandler(w, r.WithContext(ctx))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, h
}

// pollUntil retries fn every 10ms until it returns true or deadline elapses.
func pollUntil(deadline time.Duration, fn func() bool) bool {
	end := time.Now().Add(deadline)
	for time.Now().Before(end) {
		if fn() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// TestE2EPauseControlPlane exercises the full pause control plane:
// real Registry → real WS daemon → send-command HTTP endpoint.
func TestE2EPauseControlPlane(t *testing.T) {
	const agentID = "e2e-agent-pause"
	const wantSID = "sid-e2e"
	const wantIssue = "IS-e2e"

	registry := agentconn.NewRegistry(nil)
	srv, _ := buildE2EServer(t, registry)

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:          srv.URL,
		AgentID:            agentID,
		Capabilities:       []string{"test"},
		ControlCh:          ctrlCh,
		PauseRenewInterval: time.Hour, // suppress renewer ticks
		Holder: &e2eHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-X",
			SessionID: wantSID,
			IssueID:   wantIssue,
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	daemonDone := make(chan error, 1)
	go func() { daemonDone <- d.RunForever(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-daemonDone
	})

	// Wait for daemon to register.
	if !pollUntil(time.Second, func() bool { return registry.IsConnected(agentID) }) {
		t.Fatal("daemon never registered with server")
	}

	// POST pause command.
	body, _ := json.Marshal(map[string]string{"command": "pause"})
	resp, err := http.Post(srv.URL+"/api/agents/"+agentID+"/command", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST command: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	// Drain ControlCh — pause message must arrive within 2s.
	var got agentdaemon.ControlMsg
	select {
	case got = <-ctrlCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for pause ControlMsg")
	}

	if got.Type != "pause" {
		t.Errorf("ControlMsg.Type = %q, want %q", got.Type, "pause")
	}
	if got.SessionID != wantSID {
		t.Errorf("ControlMsg.SessionID = %q, want %q", got.SessionID, wantSID)
	}
	if got.IssueID != wantIssue {
		t.Errorf("ControlMsg.IssueID = %q, want %q", got.IssueID, wantIssue)
	}

	// PauseSnapshot must reflect the paused state.
	if !pollUntil(500*time.Millisecond, func() bool {
		paused, _, _ := d.PauseSnapshot()
		return paused
	}) {
		t.Error("PauseSnapshot: paused = false, want true after pause command")
	}
	paused, snapSID, snapIssue := d.PauseSnapshot()
	if !paused {
		t.Errorf("paused = false, want true")
	}
	if snapSID != wantSID {
		t.Errorf("PauseSnapshot sessionID = %q, want %q", snapSID, wantSID)
	}
	if snapIssue != wantIssue {
		t.Errorf("PauseSnapshot issueID = %q, want %q", snapIssue, wantIssue)
	}
}

// TestE2EPauseUnknownAgent verifies the "agent not connected" branch with a
// real registry (no mocked Commander) returns 404.
func TestE2EPauseUnknownAgent(t *testing.T) {
	registry := agentconn.NewRegistry(nil)
	srv, _ := buildE2EServer(t, registry)

	body, _ := json.Marshal(map[string]string{"command": "pause"})
	resp, err := http.Post(srv.URL+"/api/agents/no-such-agent/command", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST command: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}
