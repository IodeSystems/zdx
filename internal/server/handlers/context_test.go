package handlers

import (
	"context"
	"testing"
)

func TestProjectScopeFromContext(t *testing.T) {
	ctx := context.Background()
	if got := ProjectScopeFromContext(ctx); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}

	scope := []string{"proj-a", "proj-b"}
	ctx = context.WithValue(ctx, CtxProjectScope, scope)
	got := ProjectScopeFromContext(ctx)
	if len(got) != 2 || got[0] != "proj-a" || got[1] != "proj-b" {
		t.Fatalf("unexpected scope: %v", got)
	}
}

func TestIsInProjectScope(t *testing.T) {
	base := context.Background()

	// nil scope → unrestricted
	if !IsInProjectScope(base, "anything") {
		t.Fatal("nil scope should be unrestricted")
	}

	ctx := context.WithValue(base, CtxProjectScope, []string{"proj-a", "proj-b"})
	if !IsInProjectScope(ctx, "proj-a") {
		t.Fatal("proj-a should be in scope")
	}
	if !IsInProjectScope(ctx, "proj-b") {
		t.Fatal("proj-b should be in scope")
	}
	if IsInProjectScope(ctx, "proj-c") {
		t.Fatal("proj-c should not be in scope")
	}
}
