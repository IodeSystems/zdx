package agentdaemon_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
)

// TestPauseHoldRenewsLease covers TK-1361: while paused, the daemon enters a
// hold loop that calls PauseRenewer at PauseRenewInterval — this is the
// "holds task lease without spawning new work" behaviour required by spec 180.
//
// We deliberately do not start any RunningTask: the test is about the daemon
// not spawning new turns and only renewing on the hold loop's cadence. (The
// "no new runSession is called" half of the unit test is structural — the
// daemon does not invoke runSession itself; the cmd/dx-agent task loop only
// reacts to ControlCh, which we observe via the captured pause message.)
func TestPauseHoldRenewsLease(t *testing.T) {
	const wantSID = "session-pause-hold"
	const wantIssue = "IS-602"

	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	srv := controlServer(t, [][]byte{pause})
	defer srv.Close()

	var renewCount int64
	var renewSID, renewIssue atomic.Value
	renewSID.Store("")
	renewIssue.Store("")

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:          srv.URL,
		AgentID:            "test-pause-hold",
		Capabilities:       []string{"test"},
		ControlCh:          ctrlCh,
		PauseRenewInterval: 15 * time.Millisecond,
		PauseRenewer: agentdaemon.LeaseRenewerFunc(func(_ context.Context, sid, issueID string) {
			atomic.AddInt64(&renewCount, 1)
			renewSID.Store(sid)
			renewIssue.Store(issueID)
		}),
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-paused",
			SessionID: wantSID,
			IssueID:   wantIssue,
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	// Wait for the pause ControlMsg so we know the hold loop has been spawned.
	select {
	case m := <-ctrlCh:
		if m.Type != "pause" {
			t.Fatalf("expected pause msg, got %q", m.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("never received pause ControlMsg")
	}

	// Allow several hold-loop ticks.
	time.Sleep(120 * time.Millisecond)

	got := atomic.LoadInt64(&renewCount)
	if got < 1 {
		t.Fatalf("PauseRenewer.Renew called %d times during hold, want ≥ 1", got)
	}
	if s := renewSID.Load().(string); s != wantSID {
		t.Errorf("renewer received sessionID %q, want %q", s, wantSID)
	}
	if s := renewIssue.Load().(string); s != wantIssue {
		t.Errorf("renewer received issueID %q, want %q", s, wantIssue)
	}

	// Cancel the daemon — hold loop must stop.
	cancel()
	<-done

	// After shutdown, count must not keep climbing.
	stopped := atomic.LoadInt64(&renewCount)
	time.Sleep(60 * time.Millisecond)
	if after := atomic.LoadInt64(&renewCount); after != stopped {
		t.Errorf("hold loop kept ticking after shutdown: was %d, now %d", stopped, after)
	}
}

// TestPauseStateFile: pause writes a "paused\n<sid>\n<issue>\n" breadcrumb to
// StateFile; resume removes it. A crashed/restarted daemon can read this file
// to know it was paused.
func TestPauseStateFile(t *testing.T) {
	const wantSID = "session-state-file"
	const wantIssue = "IS-99"

	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	srv := controlServer(t, [][]byte{pause})
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "agent.state")

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:          srv.URL,
		AgentID:            "test-state-file",
		Capabilities:       []string{"test"},
		ControlCh:          ctrlCh,
		StateFile:          statePath,
		PauseRenewInterval: time.Hour, // suppress renewer ticks; we're only checking the file
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-state",
			SessionID: wantSID,
			IssueID:   wantIssue,
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	// Wait for pause to be processed.
	select {
	case m := <-ctrlCh:
		if m.Type != "pause" {
			t.Fatalf("expected pause msg, got %q", m.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("never received pause ControlMsg")
	}

	body, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("state file not written after pause: %v", err)
	}
	want := "paused\n" + wantSID + "\n" + wantIssue + "\n"
	if string(body) != want {
		t.Errorf("state file = %q, want %q", string(body), want)
	}

	// Cancel: shutdown stops the hold loop. The state file is intentionally
	// NOT removed on shutdown — only on resume — so a crashed/restarted
	// daemon still finds the breadcrumb.
	cancel()
	<-done

	if _, err := os.Stat(statePath); err != nil {
		t.Errorf("state file should persist across daemon shutdown (crash-recovery breadcrumb), but stat failed: %v", err)
	}
}

// TestPauseStateFileResumeRemoves: full pause→resume cycle; state file is
// written on pause and removed on resume.
func TestPauseStateFileResumeRemoves(t *testing.T) {
	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	resume, _ := json.Marshal(map[string]any{"type": "resume"})
	srv := controlServer(t, [][]byte{pause, resume})
	defer srv.Close()

	dir := t.TempDir()
	statePath := filepath.Join(dir, "agent.state")

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:          srv.URL,
		AgentID:            "test-state-file-resume",
		Capabilities:       []string{"test"},
		ControlCh:          ctrlCh,
		StateFile:          statePath,
		PauseRenewInterval: time.Hour,
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-resume-state",
			SessionID: "sid-resume-state",
			IssueID:   "IS-resume-state",
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	// Wait for both pause and resume control messages.
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

	// Give the resume handler a moment to remove the file.
	deadline = time.After(500 * time.Millisecond)
	for {
		_, err := os.Stat(statePath)
		if os.IsNotExist(err) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("state file still present after resume")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
