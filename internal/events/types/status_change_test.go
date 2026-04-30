package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/events"
)

func TestStatusChange_Validate(t *testing.T) {
	s := &StatusChange{}
	cases := []struct {
		name    string
		detail  string
		wantErr bool
	}{
		{"missing target_kind", `{"from_status":"open","to_status":"closed","actor":"u"}`, true},
		{"empty target_kind", `{"target_kind":"","from_status":"open","to_status":"closed","actor":"u"}`, true},
		{"missing from", `{"target_kind":"issue","to_status":"closed","actor":"u"}`, true},
		{"empty from", `{"target_kind":"issue","from_status":"","to_status":"closed","actor":"u"}`, true},
		{"missing to", `{"target_kind":"issue","from_status":"open","actor":"u"}`, true},
		{"empty to", `{"target_kind":"issue","from_status":"open","to_status":"","actor":"u"}`, true},
		{"identical from/to", `{"target_kind":"issue","from_status":"open","to_status":"open","actor":"u"}`, true},
		{"missing actor", `{"target_kind":"issue","from_status":"open","to_status":"closed","actor":""}`, true},
		{"empty detail", ``, true},
		{"valid issue close", `{"target_kind":"issue","from_status":"open","to_status":"closed","actor":"u"}`, false},
		{"valid task wip", `{"target_kind":"task","from_status":"ready","to_status":"wip","actor":"agent"}`, false},
		{"valid feature kind", `{"target_kind":"feature","from_status":"draft","to_status":"shipped","actor":"u"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := s.Validate(json.RawMessage(tc.detail))
			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestStatusChange_Summary(t *testing.T) {
	s := &StatusChange{}
	got, err := s.Summary(json.RawMessage(`{"target_kind":"issue","from_status":"open","to_status":"closed","actor":"u"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "issue") {
		t.Fatalf("summary missing target_kind: %q", got)
	}
	if !strings.Contains(got, "open → closed") {
		t.Fatalf("summary missing transition arrow: %q", got)
	}
}

func TestStatusChange_Audience(t *testing.T) {
	s := &StatusChange{}
	want := map[string]bool{events.AudienceUI: false, events.AudienceAgent: false}
	for _, a := range s.Audience() {
		want[a] = true
	}
	if !want[events.AudienceUI] || !want[events.AudienceAgent] {
		t.Fatalf("expected audience {ui,agent}, got %v", s.Audience())
	}
}

func TestStatusChange_Detail_RoundTrip(t *testing.T) {
	s := &StatusChange{}
	v, err := s.Detail(json.RawMessage(`{"target_kind":"issue","from_status":"open","to_status":"closed","actor":"u"}`))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := v.(StatusChangeDetail)
	if !ok {
		t.Fatalf("expected StatusChangeDetail, got %T", v)
	}
	if d.TargetKind != "issue" || d.FromStatus != "open" || d.ToStatus != "closed" {
		t.Fatalf("unexpected parsed detail: %+v", d)
	}
}
