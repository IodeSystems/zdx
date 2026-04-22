package config

import "testing"

func TestIsGlobalMode(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"no":    false,
		"false": false,
		"1":     true,
		"true":  true,
		"TRUE":  true,
		" 1 ":   true,
	}
	for val, want := range cases {
		t.Setenv("DX_GLOBAL", val)
		if got := IsGlobalMode(); got != want {
			t.Errorf("DX_GLOBAL=%q: got %v, want %v", val, got, want)
		}
	}
}
