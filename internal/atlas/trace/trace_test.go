package trace_test

import (
	"context"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/atlas/trace"
)

func TestNoteWithoutStart(t *testing.T) {
	ctx := context.Background()
	trace.Note(ctx, "foo", "bar") // must not panic
	if got := trace.From(ctx); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestStartNoteFrom(t *testing.T) {
	ctx := trace.Start(context.Background())
	trace.Note(ctx, "step", 1)
	trace.Note(ctx, "step", 2)
	entries := trace.From(ctx)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Label != "step" || entries[0].Value != 1 {
		t.Errorf("entry 0 wrong: %+v", entries[0])
	}
	if entries[1].Value != 2 {
		t.Errorf("entry 1 wrong: %+v", entries[1])
	}
}

func TestCoderefCallsite(t *testing.T) {
	ctx := trace.Start(context.Background())
	trace.Note(ctx, "here", "x") // this line sets the coderef
	entries := trace.From(ctx)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if !strings.Contains(entries[0].Coderef, "trace_test.go") {
		t.Errorf("expected coderef to contain trace_test.go, got %q", entries[0].Coderef)
	}
}

func TestCap(t *testing.T) {
	ctx := trace.Start(context.Background())
	for i := range 60 {
		trace.Note(ctx, "n", i)
	}
	entries := trace.From(ctx)
	if len(entries) != 50 {
		t.Fatalf("expected 50 entries (cap), got %d", len(entries))
	}
}

func TestFromNilContext(t *testing.T) {
	ctx := context.Background()
	if got := trace.From(ctx); got != nil {
		t.Fatalf("expected nil from unstarted context, got %v", got)
	}
}

func TestStartIsolation(t *testing.T) {
	base := context.Background()
	ctx1 := trace.Start(base)
	ctx2 := trace.Start(base)
	trace.Note(ctx1, "a", 1)
	trace.Note(ctx2, "b", 2)
	if len(trace.From(ctx1)) != 1 {
		t.Error("ctx1 should have 1 entry")
	}
	if len(trace.From(ctx2)) != 1 {
		t.Error("ctx2 should have 1 entry")
	}
	if trace.From(ctx1)[0].Label != "a" {
		t.Error("ctx1 entry should be 'a'")
	}
	if trace.From(ctx2)[0].Label != "b" {
		t.Error("ctx2 entry should be 'b'")
	}
}
