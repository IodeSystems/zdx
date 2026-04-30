package agentdaemon

import (
	"context"
	"time"
)

// RunningTask is a snapshot of the task currently executing inside an agent session.
type RunningTask struct {
	ID        string
	SessionID string // current Claude session ID; used as prevSID on resume
	IssueID   string // issue being worked; used as ResumeIssueID on resume
	Started   time.Time
}

// TaskHolder is implemented by the component that owns the live agent session.
// Phase 1: no session runs, so the no-op holder is used.
// Phase 2 (IS-602+): the real session manager wires in here.
type TaskHolder interface {
	CurrentTask() *RunningTask
	WaitForCompletion(ctx context.Context) error
}

// noopHolder is the Phase-1 placeholder: never has a running task.
type noopHolder struct{}

func (noopHolder) CurrentTask() *RunningTask                 { return nil }
func (noopHolder) WaitForCompletion(_ context.Context) error { return nil }

// NoopHolder returns a TaskHolder that reports no running task.
func NoopHolder() TaskHolder { return noopHolder{} }
