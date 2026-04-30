package agentdaemon

// ControlMsg is a server→agent command delivered over the WS control channel.
// The daemon forwards these to Daemon.ControlCh so the task loop can react
// without polling.
type ControlMsg struct {
	// Type is "pause" or "resume".
	Type string

	// SessionID is the Claude session ID that was active when the pause was
	// issued.  On resume, the task loop passes this as TakeConfig.ResumeSID so
	// runSession receives prevSID=SessionID and resumed=true — exactly the same
	// path taken by crash-recovery in take.go.
	SessionID string

	// IssueID is the issue being worked at pause time; maps to
	// TakeConfig.ResumeIssueID on resume so Take skips re-claiming.
	IssueID string
}
