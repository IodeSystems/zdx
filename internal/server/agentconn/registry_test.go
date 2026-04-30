package agentconn_test

import (
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/server/agentconn"
)

func TestRegistry(t *testing.T) {
	r := agentconn.NewRegistry(nil)

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

func TestRegistryOnDisconnectCallback(t *testing.T) {
	var calls []string
	r := agentconn.NewRegistry(func(id string) { calls = append(calls, id) })

	c := &agentconn.Conn{AgentID: "agent-x", ConnectedAt: time.Now()}
	if err := r.Register(c); err != nil {
		t.Fatalf("register: %v", err)
	}

	// callback fires exactly once on unregister
	r.Unregister("agent-x")
	if len(calls) != 1 || calls[0] != "agent-x" {
		t.Fatalf("expected exactly one callback for agent-x, got %v", calls)
	}

	// no callback for a second unregister of the same (already removed) id
	r.Unregister("agent-x")
	if len(calls) != 1 {
		t.Fatalf("expected no second callback, got %v", calls)
	}

	// no callback for an id that was never registered
	r.Unregister("never-registered")
	if len(calls) != 1 {
		t.Fatalf("expected no callback for unknown id, got %v", calls)
	}
}
