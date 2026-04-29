package work

import (
	"testing"
	"time"
)

// daysAgo returns a date string N days before today, using local time so the
// resulting (now - parsedUTC) duration brackets N. Margins in TestJournalOverdue
// allow ±1.5 day of TZ/time-of-day drift around the cadence boundary.
func daysAgo(n int) string {
	return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
}

func TestJournalOverdue(t *testing.T) {
	tests := []struct {
		name        string
		owner, tech string
		closed      int64
		wantOver    bool
		wantRole    string
	}{
		// (a) zero closed → cadence 30 days
		{"zero_closed_fresh", daysAgo(28), daysAgo(28), 0, false, ""},
		{"zero_closed_stale", daysAgo(32), daysAgo(32), 0, true, "owner"},

		// (b) 100 closed → 30/10=3, clamped to 7-day floor
		{"high_velocity_fresh", daysAgo(5), daysAgo(5), 100, false, ""},
		{"high_velocity_stale", daysAgo(9), daysAgo(9), 100, true, "owner"},

		// (c) 50 closed → 30/5=6, also clamped to 7 (same boundary as 100)
		{"mid_velocity_clamped_fresh", daysAgo(5), daysAgo(5), 50, false, ""},
		{"mid_velocity_clamped_stale", daysAgo(9), daysAgo(9), 50, true, "owner"},

		// (d) 10 closed → 30/1=30 (boundary with case a)
		{"ten_closed_fresh", daysAgo(28), daysAgo(28), 10, false, ""},
		{"ten_closed_stale", daysAgo(32), daysAgo(32), 10, true, "owner"},

		// (e) empty journal date → overdue regardless of velocity
		{"empty_owner_overdue", "", daysAgo(1), 0, true, "owner"},
		{"empty_tech_overdue", daysAgo(1), "", 0, true, "tech"},
		{"both_empty_owner_first", "", "", 0, true, "owner"},

		// (f) owner checked before tech
		{"owner_stale_tech_fresh", daysAgo(32), daysAgo(1), 0, true, "owner"},
		{"owner_fresh_tech_stale", daysAgo(1), daysAgo(32), 0, true, "tech"},
		{"both_stale_owner_wins", daysAgo(32), daysAgo(32), 0, true, "owner"},
		{"both_fresh", daysAgo(1), daysAgo(1), 0, false, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOver, gotRole := journalOverdue(tc.owner, tc.tech, tc.closed)
			if gotOver != tc.wantOver || gotRole != tc.wantRole {
				t.Fatalf("journalOverdue(%q,%q,%d) = (%v,%q); want (%v,%q)",
					tc.owner, tc.tech, tc.closed, gotOver, gotRole, tc.wantOver, tc.wantRole)
			}
		})
	}
}

func TestJournalOverdueMalformedDate(t *testing.T) {
	// Unparseable dates are treated as overdue (defensive — never silently fresh).
	over, role := journalOverdue("not-a-date", daysAgo(1), 0)
	if !over || role != "owner" {
		t.Fatalf("malformed owner: got (%v,%q); want (true,\"owner\")", over, role)
	}
	over, role = journalOverdue(daysAgo(1), "garbage", 0)
	if !over || role != "tech" {
		t.Fatalf("malformed tech: got (%v,%q); want (true,\"tech\")", over, role)
	}
}
