package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// TestAgentHeartbeatLoopAdvancesLastHeartbeat covers spec 70 on feature
// dx-todo-agent: an agent running solo with --agent-id periodically calls the
// heartbeat endpoint, keeping its last_heartbeat fresh for staleness detection.
//
// HeartbeatLoop (internal/cli/agent_shared.go) ticks at the supplied interval
// and POSTs /api/agents/{id}/heartbeat, which UPDATEs zdx_agents.last_heartbeat
// to NOW() (queries/agents.sql:24). Prod uses 60s; the loop's interval is
// injected, so the test runs with 50ms ticks for fast verification.
func TestAgentHeartbeatLoopAdvancesLastHeartbeat(t *testing.T) {
	const slug = "agent-heartbeat-loop"
	NewApiDriver(t, slug, "Agent Heartbeat Loop")
	agentID := registerAgent(t, slug, "heartbeat-loop-agent", "heartbeat-session", "")

	initial := getAgentHeartbeat(t, agentID)

	c := cli.NewClientWithSlug(srv.URL, srv.AdminToken, slug)
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		cli.HeartbeatLoop(c, agentID, 50*time.Millisecond, stop)
		close(done)
	}()

	// Allow several ticks (~4–5) so we are robust to scheduler jitter and
	// the postgres timestamp resolution (microseconds).
	time.Sleep(250 * time.Millisecond)
	close(stop)
	<-done

	after := getAgentHeartbeat(t, agentID)
	if !after.After(initial) {
		t.Fatalf("last_heartbeat did not advance: initial=%s after=%s",
			initial.Format(time.RFC3339Nano), after.Format(time.RFC3339Nano))
	}
}

func getAgentHeartbeat(t *testing.T, agentID string) time.Time {
	t.Helper()
	var resp struct {
		LastHeartbeat string `json:"last_heartbeat"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/agents/%s", agentID), nil, &resp))
	ts, err := time.Parse(time.RFC3339Nano, resp.LastHeartbeat)
	if err != nil {
		t.Fatalf("parse last_heartbeat %q: %v", resp.LastHeartbeat, err)
	}
	return ts
}
