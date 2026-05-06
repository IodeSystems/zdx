package streams_test

import (
	"context"
	"errors"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/events/streams"
)

type fakeStreamHandler struct {
	targetType string
}

func (f *fakeStreamHandler) TargetType() string { return f.targetType }

func (f *fakeStreamHandler) FetchBody(_ context.Context, _ *cli.Client, _, _ string) (string, error) {
	return "", nil
}

func (f *fakeStreamHandler) ApplyAddressed(_ context.Context, _ *cli.Client, _, _ string, _ int64, _ string) error {
	return nil
}

func TestRegister_AndLookup(t *testing.T) {
	h := &fakeStreamHandler{targetType: "test-roundtrip"}
	streams.Register(h)

	got, err := streams.Lookup("test-roundtrip")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if got.TargetType() != "test-roundtrip" {
		t.Fatalf("Lookup returned handler with TargetType()=%q", got.TargetType())
	}
}

func TestRegister_DuplicatePanics(t *testing.T) {
	streams.Register(&fakeStreamHandler{targetType: "test-duplicate"})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration, got none")
		}
	}()
	streams.Register(&fakeStreamHandler{targetType: "test-duplicate"})
}

func TestRegister_EmptyTargetTypePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty target_type, got none")
		}
	}()
	streams.Register(&fakeStreamHandler{targetType: ""})
}

func TestLookup_Unknown(t *testing.T) {
	_, err := streams.Lookup("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown target_type")
	}
	if !errors.Is(err, streams.ErrUnknownTargetType) {
		t.Fatalf("expected ErrUnknownTargetType, got %v", err)
	}
}

func TestAll_SortedByTargetType(t *testing.T) {
	streams.Register(&fakeStreamHandler{targetType: "test-sort-c"})
	streams.Register(&fakeStreamHandler{targetType: "test-sort-a"})
	streams.Register(&fakeStreamHandler{targetType: "test-sort-b"})

	all := streams.All()

	var seen []string
	for _, h := range all {
		switch h.TargetType() {
		case "test-sort-a", "test-sort-b", "test-sort-c":
			seen = append(seen, h.TargetType())
		}
	}
	want := []string{"test-sort-a", "test-sort-b", "test-sort-c"}
	if len(seen) != len(want) {
		t.Fatalf("expected %v in All(), got %v", want, seen)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Fatalf("All() not sorted: got %v, want %v", seen, want)
		}
	}
}
