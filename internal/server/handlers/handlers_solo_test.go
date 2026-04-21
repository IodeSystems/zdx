package handlers

import "testing"

func TestFoldIssuePriority(t *testing.T) {
	cases := []struct {
		name     string
		base     int32
		priority string
		want     int32
	}{
		{"dev P1", 40, "1", 20},
		{"dev P2", 40, "2", 25},
		{"dev P3", 40, "3", 30},
		{"dev P4", 40, "4", 35},
		{"dev untriaged", 40, "", 35},
		{"closable P1", 35, "1", 15},
		{"add P2", 38, "2", 23},
		{"close:tracker P3", 36, "3", 26},
		{"unknown priority treated as P4", 40, "garbage", 35},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := foldIssuePriority(tc.base, tc.priority)
			if got != tc.want {
				t.Errorf("foldIssuePriority(%d, %q) = %d, want %d", tc.base, tc.priority, got, tc.want)
			}
		})
	}
}
