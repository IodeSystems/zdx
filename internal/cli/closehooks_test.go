package cli

import "testing"

func TestIsWorkerBranch(t *testing.T) {
	cases := []struct {
		branch string
		want   bool
	}{
		{"", false},
		{"HEAD", false},
		{"dev", false},
		{"main", false},
		{"feature/foo", true},
		{"agent/TK-1787", true},
		{"agent-claude-2", true},
	}
	for _, tc := range cases {
		if got := isWorkerBranch(tc.branch); got != tc.want {
			t.Errorf("isWorkerBranch(%q) = %v, want %v", tc.branch, got, tc.want)
		}
	}
}
