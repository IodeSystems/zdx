package types

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/events"
)

func TestComment_Validate(t *testing.T) {
	c := &Comment{}
	cases := []struct {
		name    string
		detail  string
		wantErr bool
	}{
		{"missing body", `{}`, true},
		{"empty body", `{"body":""}`, true},
		{"whitespace body", `{"body":"   "}`, true},
		{"invalid json", `{not json`, true},
		{"empty detail", ``, true},
		{"unknown format", `{"body":"hi","format":"html"}`, true},
		{"valid markdown default", `{"body":"hi"}`, false},
		{"valid markdown explicit", `{"body":"hi","format":"markdown"}`, false},
		{"valid text", `{"body":"hi","format":"text"}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := c.Validate(json.RawMessage(tc.detail))
			if tc.wantErr && err == nil {
				t.Fatal("want error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestComment_Summary_Truncates(t *testing.T) {
	c := &Comment{}
	long := strings.Repeat("a", 500)
	got, err := c.Summary(json.RawMessage(`{"body":"` + long + `"}`))
	if err != nil {
		t.Fatal(err)
	}
	if r := []rune(got); len(r) != commentSummaryMaxLen {
		t.Fatalf("expected length %d, got %d", commentSummaryMaxLen, len(r))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

func TestComment_Summary_CollapsesWhitespace(t *testing.T) {
	c := &Comment{}
	got, err := c.Summary(json.RawMessage(`{"body":"line one\n\nline   two"}`))
	if err != nil {
		t.Fatal(err)
	}
	if got != "line one line two" {
		t.Fatalf("expected collapsed whitespace, got %q", got)
	}
}

func TestComment_Detail_FillsDefaultFormat(t *testing.T) {
	c := &Comment{}
	v, err := c.Detail(json.RawMessage(`{"body":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	d, ok := v.(CommentDetail)
	if !ok {
		t.Fatalf("expected CommentDetail, got %T", v)
	}
	if d.Format != "markdown" {
		t.Fatalf("expected default format markdown, got %q", d.Format)
	}
}

func TestComment_Audience(t *testing.T) {
	c := &Comment{}
	want := map[string]bool{events.AudienceUI: false, events.AudienceAgent: false}
	for _, a := range c.Audience() {
		want[a] = true
	}
	if !want[events.AudienceUI] || !want[events.AudienceAgent] {
		t.Fatalf("expected audience {ui,agent}, got %v", c.Audience())
	}
}

func TestComment_ReactComponentName(t *testing.T) {
	if (&Comment{}).ReactComponentName() != "EventComment" {
		t.Fatalf("unexpected react component name")
	}
}
