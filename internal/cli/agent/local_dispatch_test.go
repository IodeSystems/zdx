package agent

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewRemoteDispatcher_EmptyCommandErrors(t *testing.T) {
	_, err := newRemoteDispatcher(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for empty command, got nil")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Errorf("error message should explain the issue: %v", err)
	}
}

func TestNewRemoteDispatcher_BogusCommandErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := newRemoteDispatcher(ctx, []string{"/nonexistent/binary-that-cannot-exist", "--mcp-stdio"})
	if err == nil {
		t.Fatal("expected error spawning a missing binary, got nil")
	}
}
