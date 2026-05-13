package agent

import (
	"strings"
	"testing"
)

func TestNormalizePersona(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "dev", false},
		{"  ", "dev", false},
		{"DEV", "dev", false},
		{"dev", "dev", false},
		{"tech", "tech", false},
		{"reviewer", "reviewer", false},
		{"product", "product", false},
		{"PM", "", true},
		{"owner", "", true},
		{"engineer", "", true},
	}
	for _, tc := range cases {
		got, err := NormalizePersona(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("NormalizePersona(%q): expected error, got %q", tc.in, got)
				continue
			}
			msg := err.Error()
			for _, p := range []string{"dev", "tech", "reviewer", "product"} {
				if !strings.Contains(msg, p) {
					t.Errorf("NormalizePersona(%q) error %q must list allowed persona %q", tc.in, msg, p)
				}
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizePersona(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NormalizePersona(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPersonaPromptEmbedsAllFour(t *testing.T) {
	wantSubstr := map[string]string{
		"dev":      "Persona: dev (engineer)",
		"tech":     "Persona: tech (tech lead)",
		"reviewer": "Persona: reviewer (QA",
		"product":  "Persona: product (PM)",
	}
	for persona, substr := range wantSubstr {
		got, err := PersonaPrompt(persona)
		if err != nil {
			t.Errorf("PersonaPrompt(%q): %v", persona, err)
			continue
		}
		if !strings.Contains(got, substr) {
			t.Errorf("PersonaPrompt(%q) missing %q\n--- got ---\n%s", persona, substr, got)
		}
	}
}

func TestPersonaPromptReviewerProductForbidEditWrite(t *testing.T) {
	for _, persona := range []string{"reviewer", "product"} {
		got, err := PersonaPrompt(persona)
		if err != nil {
			t.Fatalf("PersonaPrompt(%q): %v", persona, err)
		}
		// Each of these must emphasize the no-edit/write-file rule per IS-1096.
		for _, want := range []string{"cannot `edit_file` or `write_file`"} {
			if !strings.Contains(got, want) {
				t.Errorf("PersonaPrompt(%q) missing %q", persona, want)
			}
		}
	}
}

func TestPersonaPromptUnknownErrors(t *testing.T) {
	if _, err := PersonaPrompt("nope"); err == nil {
		t.Fatal("expected error for unknown persona")
	}
}
