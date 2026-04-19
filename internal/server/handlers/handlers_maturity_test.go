package handlers

import (
	"strings"
	"testing"
)

func TestBuildMaturityStatusParams(t *testing.T) {
	cases := []struct {
		name          string
		status        string
		justification string
		snoozeUntil   string
		wantErrFrag   string // non-empty when we expect a validation error
	}{
		{name: "open ok", status: "open"},
		{name: "done ok", status: "done", justification: "shipped"},
		{name: "dismissed ok", status: "dismissed", justification: "n/a"},
		{name: "snoozed ok", status: "snoozed", snoozeUntil: "2026-05-01T00:00:00Z"},

		{name: "unknown status", status: "invalid", wantErrFrag: "one of"},
		{name: "done missing justification", status: "done", wantErrFrag: "justification required"},
		{name: "dismissed missing justification", status: "dismissed", wantErrFrag: "justification required"},
		{name: "snoozed missing snooze_until", status: "snoozed", wantErrFrag: "snooze_until required"},
		{name: "snoozed bad timestamp", status: "snoozed", snoozeUntil: "not-a-date", wantErrFrag: "RFC3339"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := buildMaturityStatusParams(42, tc.status, tc.justification, tc.snoozeUntil)
			if tc.wantErrFrag == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if params.ID != 42 {
					t.Errorf("ID = %d, want 42", params.ID)
				}
				if params.Status != tc.status {
					t.Errorf("Status = %q, want %q", params.Status, tc.status)
				}
				if tc.status == "snoozed" && !params.SnoozeUntil.Valid {
					t.Errorf("expected SnoozeUntil to be valid for snoozed status")
				}
				if tc.status != "snoozed" && params.SnoozeUntil.Valid {
					t.Errorf("expected SnoozeUntil to be cleared for non-snoozed status")
				}
				return
			}
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrFrag)
			}
			if !strings.Contains(err.Error(), tc.wantErrFrag) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantErrFrag)
			}
		})
	}
}
