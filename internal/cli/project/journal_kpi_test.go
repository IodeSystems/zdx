package project

import (
	"strings"
	"testing"
)

func TestFormatKPIDeltaLine(t *testing.T) {
	tests := []struct {
		name     string
		delta    kpiDelta
		contains []string
		absent   []string
	}{
		{
			name:     "regression with time marker",
			delta:    kpiDelta{CheckName: "go_build", Unit: "s", Prev: 8, Curr: 18, Pct: 125},
			contains: []string{"go_build", "18", "+125.0%", "*"},
		},
		{
			name:     "improvement no marker",
			delta:    kpiDelta{CheckName: "test_count", Unit: "", Prev: 100, Curr: 110, Pct: 10},
			contains: []string{"test_count", "110", "+10.0%"},
			absent:   []string{"*"},
		},
		{
			name:     "negative pct shows minus sign",
			delta:    kpiDelta{CheckName: "lint_errors", Unit: "", Prev: 10, Curr: 5, Pct: -50},
			contains: []string{"lint_errors", "5", "-50.0%"},
		},
		{
			name:     "pct over 50 triggers marker",
			delta:    kpiDelta{CheckName: "some_check", Unit: "ms", Prev: 100, Curr: 160, Pct: 60},
			contains: []string{"some_check", "160", "+60.0%", "*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			line := formatKPIDeltaLine(tt.delta)
			for _, s := range tt.contains {
				if !strings.Contains(line, s) {
					t.Errorf("expected line to contain %q, got: %q", s, line)
				}
			}
			for _, s := range tt.absent {
				if strings.Contains(line, s) {
					t.Errorf("expected line to NOT contain %q, got: %q", s, line)
				}
			}
		})
	}
}

func TestPrintKPIDeltaSummaryEmpty(t *testing.T) {
	// Should not panic or print anything for empty/null payloads.
	printKPIDeltaSummary("")
	printKPIDeltaSummary("[]")
	printKPIDeltaSummary("invalid json")
}
