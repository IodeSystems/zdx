package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/events"
)

func TestReaction_Validate(t *testing.T) {
	r := &Reaction{}
	cases := []struct {
		name    string
		detail  string
		wantErr bool
	}{
		{"missing emoji", `{"target_event_id":1}`, true},
		{"empty emoji", `{"emoji":"","target_event_id":1}`, true},
		{"oversized emoji", `{"emoji":"abcdefghi","target_event_id":1}`, true},
		{"missing target", `{"emoji":"👍"}`, true},
		{"zero target", `{"emoji":"👍","target_event_id":0}`, true},
		{"negative target", `{"emoji":"👍","target_event_id":-1}`, true},
		{"empty detail", ``, true},
		{"valid simple", `{"emoji":"👍","target_event_id":42}`, false},
		{"valid zwj sequence", `{"emoji":"👨‍👩‍👧","target_event_id":42}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := r.Validate(json.RawMessage(tc.detail))
			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestReaction_Summary(t *testing.T) {
	r := &Reaction{}
	got, err := r.Summary(json.RawMessage(`{"emoji":"👍","target_event_id":42}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "👍") || !strings.Contains(got, "42") {
		t.Fatalf("summary missing emoji or target id: %q", got)
	}
}

func TestReaction_Audience_AgentOnly(t *testing.T) {
	r := &Reaction{}
	got := r.Audience()
	if len(got) != 1 || got[0] != events.AudienceAgent {
		t.Fatalf("expected [agent], got %v", got)
	}
}

func TestReaction_ReactComponentName(t *testing.T) {
	if (&Reaction{}).ReactComponentName() != "EventReaction" {
		t.Fatalf("unexpected react component name")
	}
}
