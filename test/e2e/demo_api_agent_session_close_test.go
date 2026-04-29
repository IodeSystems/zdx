package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoAPI_AgentSessionClose is the demo for spec 122 on feature
// agent-harness: an agent signals completion or failure to the orchestrator
// via POST /api/dx/agent/sessions/{sid}/close; the session outcome is always
// recorded regardless of exit code.
//
// The demo exercises two paths:
//  1. Clean exit (exit_code=0): agent completed its work successfully.
//  2. Error exit (exit_code=1): agent encountered an unresolved error.
//
// In both cases the /close endpoint must return 200 with a "closed" status and
// the session must transition to lifecycle="done" (closed_at set in DB).
// The status field ("ok"/"churn"/"errored") is set asynchronously by LLM
// summarization if events are present; this demo verifies the synchronous
// close path only.
func TestDemoAPI_AgentSessionClose(t *testing.T) {
	rec := newApiRecorder(t, "agent-session-close")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_agent_session_close_test.go", Note: "agent session close demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_agent_sessions.go", Note: "POST /api/dx/agent/sessions/{sid}/close — CloseClaudeSession + async summarize"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/lifecycle.go", Note: "postAgentSessionClose — client-side close call"})
	t.Cleanup(rec.Save)

	const slug = "demo-agent-session-close"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Agent Session Close",
	}, nil))

	closeAndVerify := func(sessionID string, exitCode int, label string) {
		t.Helper()

		// Step 1: create the session (idempotent by session_id).
		var created struct {
			ID        int64  `json:"id"`
			SessionID string `json:"session_id"`
			Created   bool   `json:"created"`
		}
		mustOK(t, rec.Do(http.MethodPost,
			fmt.Sprintf("/api/dx/agent/sessions/%s/create?slug=%s", sessionID, slug),
			map[string]any{
				"provider":   "claude",
				"alias":      label,
				"trigger":    "manual",
				"title":      fmt.Sprintf("Demo session %s", label),
				"issue_id":   "",
				"agent_id":   "",
				"agent_type": "claude",
			}, &created))
		if created.ID == 0 {
			t.Fatalf("[%s] create: expected non-zero session pk", label)
		}
		if created.SessionID != sessionID {
			t.Fatalf("[%s] create: session_id mismatch: got %q want %q", label, created.SessionID, sessionID)
		}

		// Step 2: close the session with the given exit code.
		var closed struct {
			SessionID string `json:"session_id"`
			Status    string `json:"status"`
		}
		mustOK(t, rec.Do(http.MethodPost,
			fmt.Sprintf("/api/dx/agent/sessions/%s/close?slug=%s", sessionID, slug),
			map[string]any{
				"exit_code":   exitCode,
				"duration_ms": 1000,
				"event_count": 0,
				"tokens": map[string]int64{
					"input":       0,
					"output":      0,
					"cache_read":  0,
					"cache_write": 0,
				},
			}, &closed))
		if closed.SessionID != sessionID {
			t.Errorf("[%s] close response: session_id mismatch: got %q want %q", label, closed.SessionID, sessionID)
		}
		if closed.Status != "closed" {
			t.Errorf("[%s] close response: status=%q, want \"closed\"", label, closed.Status)
		}

		// Step 3: fetch the session and verify closed_at is recorded (lifecycle="done").
		var sess struct {
			ID        int64  `json:"id"`
			SessionID string `json:"session_id"`
			Lifecycle string `json:"lifecycle"`
			ClosedAt  string `json:"closed_at"`
		}
		mustOK(t, rec.Do(http.MethodGet,
			fmt.Sprintf("/api/dx/claude/sessions/%d?slug=%s", created.ID, slug),
			nil, &sess))
		if sess.Lifecycle != "done" {
			t.Errorf("[%s] GET session: lifecycle=%q, want \"done\" (closed_at must be set)", label, sess.Lifecycle)
		}
		if sess.ClosedAt == "" {
			t.Errorf("[%s] GET session: closed_at is empty; outcome was not recorded", label)
		}
	}

	closeAndVerify("demo-close-session-success", 0, "success-exit")
	closeAndVerify("demo-close-session-failure", 1, "error-exit")
}
