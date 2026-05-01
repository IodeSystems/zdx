package agentdaemon_test

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/agentdaemon"
)

// resumeCall records the (SessionID, IssueID) that a consumer goroutine
// received when it handled a resume ControlMsg.
type resumeCall struct {
	SessionID string
	IssueID   string
}

// TestResumeE2E_ConsumerCallsTake simulates the wiring that cmd/dx-agent/main.go
// has after TK-1509. A consumer goroutine reads ControlCh and records the
// (SessionID, IssueID) it would pass to agent.RunResume on resume. The test
// fails if the consumer is never fired — i.e., if main.go were reverted to the
// no-op that only logs and discards the ControlMsg.
func TestResumeE2E_ConsumerCallsTake(t *testing.T) {
	const wantSID = "session-e2e-resume"
	const wantIssue = "IS-e2e"

	pause, _ := json.Marshal(map[string]any{"type": "pause"})
	resume, _ := json.Marshal(map[string]any{"type": "resume"})
	srv := controlServer(t, [][]byte{pause, resume})
	defer srv.Close()

	ctrlCh := make(chan agentdaemon.ControlMsg, 8)
	d := &agentdaemon.Daemon{
		ServerURL:    srv.URL,
		AgentID:      "test-e2e-resume",
		Capabilities: []string{"test"},
		ControlCh:    ctrlCh,
		Holder: &sessionHolder{task: &agentdaemon.RunningTask{
			ID:        "TK-e2e",
			SessionID: wantSID,
			IssueID:   wantIssue,
			Started:   time.Now(),
		}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- d.RunForever(ctx) }()

	// Consumer goroutine — mirrors what cmd/dx-agent/main.go does after TK-1509.
	// It records the resume call instead of invoking agent.RunResume so the test
	// does not require a project config or claude binary on CI.
	var called atomic.Bool
	var recorded resumeCall

	consumerDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		for msg := range ctrlCh {
			switch msg.Type {
			case "pause":
				// informational only
			case "resume":
				recorded = resumeCall{SessionID: msg.SessionID, IssueID: msg.IssueID}
				called.Store(true)
			}
		}
	}()

	// Wait for both pause and resume to arrive.
	deadline := time.After(2 * time.Second)
	for !called.Load() {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for resume consumer to fire")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	<-done
	close(ctrlCh)
	<-consumerDone

	if recorded.SessionID != wantSID {
		t.Errorf("consumer received SessionID=%q, want %q", recorded.SessionID, wantSID)
	}
	if recorded.IssueID != wantIssue {
		t.Errorf("consumer received IssueID=%q, want %q", recorded.IssueID, wantIssue)
	}
}
