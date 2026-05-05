package agent

import "testing"

func TestNormalizeComplexity(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"low", "low", false},
		{"l", "low", false},
		{"medium", "medium", false},
		{"med", "medium", false},
		{"m", "medium", false},
		{"high", "high", false},
		{"h", "high", false},
		{"HIGH", "high", false},
		{"  Medium  ", "medium", false},
		{"", "high", false}, // empty → DefaultComplexity
		{"xhigh", "", true},
		{"max", "", true},
		{"unknown", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := NormalizeComplexity(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("NormalizeComplexity(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDefaultComplexityIsHigh(t *testing.T) {
	if DefaultComplexity != ComplexityHigh {
		t.Errorf("DefaultComplexity = %q, want %q", DefaultComplexity, ComplexityHigh)
	}
}
