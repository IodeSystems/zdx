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

// sessionHolder reports a fixed running task with a known session + issue.
type sessionHolder struct {
	task *agentdaemon.RunningTask
}

func (h *sessionHolder) CurrentTask() *agentdaemon.RunningTask         { return h.task }
func (h *sessionHolder) WaitForCompletion(_ context.Context) error     { return nil }

// controlServer upgrades to WS, reads the handshake, acks it, then sends the
// provided frames (in order) after a short delay and holds until the client
// closes.
func controlServer(t *testing.T, frames [][]byte) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			t.Logf("accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "done")
		bctx := context.Background()

		// Read handshake
		if _, _, err = conn.Read(bctx); err != nil {
			t.Logf("read handshake: %v", err)
			return
		}

		// Send ack
		ack, _ := json.Marshal(map[string]any{"type": "registered", "server_time": time.Now()})
		if err := conn.Write(bctx, websocket.MessageText, ack); err != nil {
			t.Logf("write ack: %v", err)
			return
		}

		// Small delay so the daemon's read goroutine is ready.
		time.Sleep(20 * time.Millisecond)

		for _, frame := range frames {
			if err := conn.Write(bctx, websocket.MessageText, frame); err != nil {
				t.Logf("write frame: %v", err)
				return
			}
			time.Sleep(20 * time.Millisecond)
		}

		// Hold until client disconnects.
		for {
			if _, _, err := conn.Read(bctx); err != nil {
				return
			}
		}
	}))
}

// TestPauseResume: server sends pause then resume; daemon forwards both
// ControlMsgs on ControlCh with the session/issue from the holder preserved.
// The ControlMsg.SessionID becomes TakeConfig.ResumeSID (= prevSID in
// runSession) and ControlMsg.IssueID becomes TakeConfig.ResumeIssueID,
// replicating the crash-recovery resume path in take.go.
func TestPauseResume(t *testing.T) {
	const wantSID = "session-abc123"
	const wantIssue = "IS-99"

	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	resume, _ := json.Marshal(map[string]any{"type": "resume"})
	srv := controlServer(t, [][]byte{pause, resume})
	defer srv.Close()

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:    srv.URL,
		AgentID:      "test-resume",
		Capabilities: []string{"test"},
		ControlCh:    ctrlCh,
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-1",
			SessionID: wantSID,
			IssueID:   wantIssue,
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	// Collect two ControlMsgs within a generous deadline.
	var msgs []agentdaemon.ControlMsg
	deadline := time.After(2 * time.Second)
collect:
	for len(msgs) < 2 {
		select {
		case m := <-ctrlCh:
			msgs = append(msgs, m)
		case <-deadline:
			t.Fatalf("timed out waiting for control messages; got %d of 2", len(msgs))
			break collect
		}
	}
	cancel()
	<-done

	// First msg: pause — session ID captured from the holder.
	if msgs[0].Type != "pause" {
		t.Errorf("msgs[0].Type = %q, want %q", msgs[0].Type, "pause")
	}
	if msgs[0].SessionID != wantSID {
		t.Errorf("msgs[0].SessionID = %q, want %q", msgs[0].SessionID, wantSID)
	}
	if msgs[0].IssueID != wantIssue {
		t.Errorf("msgs[0].IssueID = %q, want %q", msgs[0].IssueID, wantIssue)
	}

	// Second msg: resume — carries the same session/issue captured at pause time.
	// These map directly to TakeConfig.ResumeSID (= prevSID) and
	// TakeConfig.ResumeIssueID, triggering the resumed=true path in Take().
	if msgs[1].Type != "resume" {
		t.Errorf("msgs[1].Type = %q, want %q", msgs[1].Type, "resume")
	}
	if msgs[1].SessionID != wantSID {
		t.Errorf("msgs[1].SessionID = %q, want %q (original prevSID not preserved)", msgs[1].SessionID, wantSID)
	}
	if msgs[1].IssueID != wantIssue {
		t.Errorf("msgs[1].IssueID = %q, want %q", msgs[1].IssueID, wantIssue)
	}
}

// TestResumeWithoutPause: a spurious resume (no prior pause) must be silently
// ignored and must not send to ControlCh.
func TestResumeWithoutPause(t *testing.T) {
	resume, _ := json.Marshal(map[string]any{"type": "resume"})
	srv := controlServer(t, [][]byte{resume})
	defer srv.Close()

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:    srv.URL,
		AgentID:      "test-spurious-resume",
		Capabilities: []string{"test"},
		ControlCh:    ctrlCh,
		Holder:       agentdaemon.NoopHolder(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	// Wait long enough for the frame to be processed.
	time.Sleep(150 * time.Millisecond)
	cancel()
	<-done

	if len(ctrlCh) != 0 {
		t.Errorf("expected no ControlMsg for spurious resume, got %d", len(ctrlCh))
	}
}
