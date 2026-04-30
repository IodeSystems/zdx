package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/events"
)

func TestRevision_Validate(t *testing.T) {
	r := &Revision{}
	cases := []struct {
		name    string
		detail  string
		wantErr bool
	}{
		{"missing field", `{"from":"a","to":"b","from_version":1,"to_version":2,"actor":"u"}`, true},
		{"invalid field", `{"field":"summary","from":"a","to":"b","from_version":1,"to_version":2,"actor":"u"}`, true},
		{"missing from", `{"field":"body","to":"b","from_version":0,"to_version":1,"actor":"u"}`, true},
		{"missing to", `{"field":"body","from":"a","from_version":1,"to_version":2,"actor":"u"}`, true},
		{"empty to", `{"field":"body","from":"a","to":"","from_version":1,"to_version":2,"actor":"u"}`, true},
		{"identical from/to", `{"field":"body","from":"x","to":"x","from_version":1,"to_version":2,"actor":"u"}`, true},
		{"to_version <= from_version", `{"field":"body","from":"a","to":"b","from_version":2,"to_version":2,"actor":"u"}`, true},
		{"to_version < from_version", `{"field":"body","from":"a","to":"b","from_version":3,"to_version":2,"actor":"u"}`, true},
		{"negative from_version", `{"field":"body","from":"a","to":"b","from_version":-1,"to_version":1,"actor":"u"}`, true},
		{"missing actor", `{"field":"body","from":"a","to":"b","from_version":1,"to_version":2,"actor":""}`, true},
		{"empty detail", ``, true},
		{"valid initial revision", `{"field":"body","from":"","to":"hello","from_version":0,"to_version":1,"actor":"u"}`, false},
		{"valid title update", `{"field":"title","from":"old","to":"new","from_version":1,"to_version":2,"actor":"u"}`, false},
		{"valid description", `{"field":"description","from":"a","to":"b","from_version":4,"to_version":5,"actor":"u"}`, false},
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

func TestRevision_Summary(t *testing.T) {
	r := &Revision{}
	got, err := r.Summary(json.RawMessage(`{"field":"body","from":"a","to":"b","from_version":3,"to_version":4,"actor":"u"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "body") {
		t.Fatalf("summary missing field name: %q", got)
	}
	if !strings.Contains(got, "v3") || !strings.Contains(got, "v4") {
		t.Fatalf("summary missing version arrow: %q", got)
	}
	if !strings.Contains(got, "updated") {
		t.Fatalf("summary missing 'updated': %q", got)
	}
}

func TestRevision_Audience(t *testing.T) {
	r := &Revision{}
	want := map[string]bool{events.AudienceUI: false, events.AudienceAgent: false}
	for _, a := range r.Audience() {
		want[a] = true
	}
	if !want[events.AudienceUI] || !want[events.AudienceAgent] {
		t.Fatalf("expected audience {ui,agent}, got %v", r.Audience())
	}
}

func TestRevision_Detail_RoundTrip(t *testing.T) {
	r := &Revision{}
	v, err := r.Detail(json.RawMessage(`{"field":"body","from":"a","to":"b","from_version":1,"to_version":2,"actor":"u"}`))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := v.(RevisionDetail)
	if !ok {
		t.Fatalf("expected RevisionDetail, got %T", v)
	}
	if d.Field != "body" || d.From == nil || *d.From != "a" || d.To == nil || *d.To != "b" {
		t.Fatalf("unexpected parsed detail: %+v", d)
	}
}
