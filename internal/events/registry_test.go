package events_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/events"
	_ "github.com/iodesystems/zdx-go/internal/events/types"
)

type fakeRenderer struct {
	eventType string
}

func (f *fakeRenderer) EventType() string                         { return f.eventType }
func (f *fakeRenderer) Audience() []string                        { return []string{events.AudienceAgent} }
func (f *fakeRenderer) Validate(_ json.RawMessage) error          { return nil }
func (f *fakeRenderer) Summary(_ json.RawMessage) (string, error) { return "", nil }
func (f *fakeRenderer) Detail(_ json.RawMessage) (any, error)     { return nil, nil }
func (f *fakeRenderer) ReactComponentName() string                { return "Fake" }

func TestRegister_DuplicatePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on duplicate registration, got none")
		}
	}()
	// "comment" is registered by side-effect import above.
	events.Register(&fakeRenderer{eventType: "comment"})
}

func TestRegister_EmptyTypePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on empty event type, got none")
		}
	}()
	events.Register(&fakeRenderer{eventType: ""})
}

func TestLookup_Unknown(t *testing.T) {
	_, err := events.Lookup("does-not-exist")
	if err == nil {
		t.Fatal("expected error for unknown event_type")
	}
	if !errors.Is(err, events.ErrUnknownEventType) {
		t.Fatalf("expected ErrUnknownEventType, got %v", err)
	}
}

func TestAll_RegistersInitialFour(t *testing.T) {
	want := map[string]bool{
		"comment":       false,
		"reaction":      false,
		"revision":      false,
		"status_change": false,
	}
	for _, r := range events.All() {
		if _, ok := want[r.EventType()]; ok {
			want[r.EventType()] = true
		}
	}
	var missing []string
	for k, v := range want {
		if !v {
			missing = append(missing, k)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing renderers in registry: %s", strings.Join(missing, ", "))
	}
}

func TestLookup_RoundTrip(t *testing.T) {
	for _, et := range []string{"comment", "reaction", "revision", "status_change"} {
		r, err := events.Lookup(et)
		if err != nil {
			t.Fatalf("Lookup(%q): %v", et, err)
		}
		if r.EventType() != et {
			t.Fatalf("Lookup(%q): renderer reports EventType()=%q", et, r.EventType())
		}
		if r.ReactComponentName() == "" {
			t.Fatalf("%s: empty ReactComponentName", et)
		}
		if len(r.Audience()) == 0 {
			t.Fatalf("%s: empty Audience", et)
		}
	}
}
