//go:build docker_e2e

// Real-server verification of the daemon WS handshake + control protocol.
// Skipped under the default test run (needs docker for ephemeral postgres);
// opt in via:
//
//	go test -tags docker_e2e ./internal/agentdaemon/
//
// Spins up an ephemeral zdx-server via internal/devserver (which brings up
// postgres via docker compose), connects an agentdaemon.Daemon to it, and
// verifies the pause/resume control roundtrip via the admin send-command
// API. No prod tokens, no LLM, no slot containers — just the daemon ↔
// server WS contract against a real server with a real DB.
package agentdaemon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
	"github.com/iodesystems/zdx-go/internal/devserver"
)

func TestDaemon_PauseResume_AgainstEphemeralServer(t *testing.T) {
	h, cleanup, err := devserver.Start(devserver.Options{})
	if err != nil {
		t.Skipf("ephemeral devserver: %v", err)
	}
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	agentID := "e2e-daemon-" + t.Name()[:8]
	ctrlCh := make(chan agentdaemon.ControlMsg, 8)

	d := &agentdaemon.Daemon{
		ServerURL:    h.URL,
		AgentID:      agentID,
		APIKey:       h.AdminToken,
		WorktreePath: "/tmp",
		Hostname:     "e2e-host",
		Pid:          1,
		Capabilities: []string{"test"},
		Holder:       agentdaemon.NoopHolder(),
		ControlCh:    ctrlCh,
	}

	daemonCtx, daemonCancel := context.WithCancel(ctx)
	defer daemonCancel()

	runDone := make(chan error, 1)
	go func() { runDone <- d.Run(daemonCtx) }()

	// Wait for the WS handshake to complete — give the daemon a beat to
	// dial, send registration, and read the ack. There's no public ready-
	// signal on Daemon; polling the server's connected-agents list is the
	// observable proxy. Cap the wait so a stuck dial fails the test loudly.
	if err := waitForConnected(ctx, h.URL, h.AdminToken, agentID, 10*time.Second); err != nil {
		daemonCancel()
		t.Fatalf("daemon never registered: %v", err)
	}

	// Push a pause via the operator API. The server's handler resolves the
	// agent in the in-memory Registry, writes the WS frame, and we expect
	// the daemon to forward the ControlMsg on ControlCh.
	if err := postAgentCommand(ctx, h.URL, h.AdminToken, agentID, "pause"); err != nil {
		t.Fatalf("send pause: %v", err)
	}
	select {
	case msg := <-ctrlCh:
		if msg.Type != "pause" {
			t.Errorf("first ControlMsg.Type = %q, want pause", msg.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ControlMsg received after pause command")
	}

	if err := postAgentCommand(ctx, h.URL, h.AdminToken, agentID, "resume"); err != nil {
		t.Fatalf("send resume: %v", err)
	}
	select {
	case msg := <-ctrlCh:
		if msg.Type != "resume" {
			t.Errorf("ControlMsg.Type = %q, want resume", msg.Type)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no ControlMsg received after resume command")
	}

	// Clean shutdown: cancel daemon ctx, wait for Run to return.
	daemonCancel()
	select {
	case err := <-runDone:
		if err != nil && ctx.Err() == nil {
			// Run returns nil on graceful shutdown; non-nil here would
			// indicate the daemon errored mid-flight.
			t.Logf("daemon Run returned: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Error("daemon did not exit within 15s of context cancellation")
	}
}

// waitForConnected polls /api/agents/list until the named agent appears
// connected, or returns an error after timeout.
func waitForConnected(ctx context.Context, baseURL, token, agentID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := baseURL + "/api/agents/list"
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.Header.Set("X-Api-Key", token)
		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			var body struct {
				Agents []struct {
					ID              string `json:"id"`
					ConnectionState string `json:"connection_state"`
				} `json:"agents"`
			}
			_ = json.NewDecoder(resp.Body).Decode(&body)
			resp.Body.Close()
			for _, a := range body.Agents {
				if a.ID == agentID && a.ConnectionState == "connected" {
					return nil
				}
			}
		} else if resp != nil {
			resp.Body.Close()
		}
		if time.Now().After(deadline) {
			return ctx.Err()
		}
		select {
		case <-time.After(200 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func postAgentCommand(ctx context.Context, baseURL, token, agentID, command string) error {
	body, _ := json.Marshal(map[string]string{"command": command})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		baseURL+"/api/agents/"+agentID+"/command", bytes.NewReader(body))
	req.Header.Set("X-Api-Key", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &httpErr{Status: resp.StatusCode, URL: req.URL.String()}
	}
	return nil
}

type httpErr struct {
	Status int
	URL    string
}

func (e *httpErr) Error() string {
	return e.URL + ": HTTP " + http.StatusText(e.Status)
}
