package agentdaemon_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
)

// TestPauseSnapshotInitiallyFalse: a fresh Daemon reports unpaused with no
// session/issue ids — the zero value is meaningful for poll-style task loops.
func TestPauseSnapshotInitiallyFalse(t *testing.T) {
	d := &agentdaemon.Daemon{}
	paused, sid, issueID := d.PauseSnapshot()
	if paused {
		t.Errorf("paused = true, want false on fresh Daemon")
	}
	if sid != "" {
		t.Errorf("sessionID = %q, want %q", sid, "")
	}
	if issueID != "" {
		t.Errorf("issueID = %q, want %q", issueID, "")
	}
}

// TestPauseSnapshotAfterPause: after a pause WS frame the snapshot returns
// (true, sid, issue) using the values captured from Holder.CurrentTask().
func TestPauseSnapshotAfterPause(t *testing.T) {
	const wantSID = "session-snapshot-pause"
	const wantIssue = "IS-820"

	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	srv := controlServer(t, [][]byte{pause})
	defer srv.Close()

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:          srv.URL,
		AgentID:            "test-snapshot-pause",
		Capabilities:       []string{"test"},
		ControlCh:          ctrlCh,
		PauseRenewInterval: time.Hour,
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-snapshot",
			SessionID: wantSID,
			IssueID:   wantIssue,
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	select {
	case m := <-ctrlCh:
		if m.Type != "pause" {
			t.Fatalf("expected pause msg, got %q", m.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("never received pause ControlMsg")
	}

	paused, sid, issueID := d.PauseSnapshot()
	if !paused {
		t.Errorf("paused = false, want true after pause")
	}
	if sid != wantSID {
		t.Errorf("sessionID = %q, want %q", sid, wantSID)
	}
	if issueID != wantIssue {
		t.Errorf("issueID = %q, want %q", issueID, wantIssue)
	}

	cancel()
	<-done
}

// TestPauseSnapshotAfterResume: after a pause→resume cycle the snapshot
// returns to the zero state so the task loop knows it's free to spawn turns.
func TestPauseSnapshotAfterResume(t *testing.T) {
	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	resume, _ := json.Marshal(map[string]any{"type": "resume"})
	srv := controlServer(t, [][]byte{pause, resume})
	defer srv.Close()

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:          srv.URL,
		AgentID:            "test-snapshot-resume",
		Capabilities:       []string{"test"},
		ControlCh:          ctrlCh,
		PauseRenewInterval: time.Hour,
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-snapshot-resume",
			SessionID: "sid-snapshot-resume",
			IssueID:   "IS-snapshot-resume",
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	var msgs []agentdaemon.ControlMsg
	deadline := time.After(2 * time.Second)
	for len(msgs) < 2 {
		select {
		case m := <-ctrlCh:
			msgs = append(msgs, m)
		case <-deadline:
			t.Fatalf("timed out: got %d of 2 ControlMsgs", len(msgs))
		}
	}
	if !strings.EqualFold(msgs[0].Type, "pause") || !strings.EqualFold(msgs[1].Type, "resume") {
		t.Fatalf("unexpected message order: %v %v", msgs[0].Type, msgs[1].Type)
	}

	paused, sid, issueID := d.PauseSnapshot()
	if paused {
		t.Errorf("paused = true, want false after resume")
	}
	if sid != "" {
		t.Errorf("sessionID = %q, want %q after resume", sid, "")
	}
	if issueID != "" {
		t.Errorf("issueID = %q, want %q after resume", issueID, "")
	}

	cancel()
	<-done
}
