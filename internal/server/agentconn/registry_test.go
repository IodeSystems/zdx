package agentconn_test

import (
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/server/agentconn"
)

func TestRegistry(t *testing.T) {
	r := agentconn.NewRegistry()

	a := &agentconn.Conn{AgentID: "agent-1", ConnectedAt: time.Now()}
	b := &agentconn.Conn{AgentID: "agent-2", ConnectedAt: time.Now()}

	if err := r.Register(a); err != nil {
		t.Fatalf("register a: %v", err)
	}
	if err := r.Register(b); err != nil {
		t.Fatalf("register b: %v", err)
	}

	if got, ok := r.Get("agent-1"); !ok || got != a {
		t.Fatal("expected agent-1 in registry")
	}
	list := r.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(list))
	}

	// duplicate registration must fail
	if err := r.Register(a); err == nil {
		t.Fatal("expected error for duplicate agent-1")
	}

	r.Unregister("agent-1")
	if _, ok := r.Get("agent-1"); ok {
		t.Fatal("expected agent-1 to be unregistered")
	}
	if len(r.List()) != 1 {
		t.Fatal("expected 1 agent after unregister")
	}
}
