package escalation

import (
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

func TestMergedTechToUser_StandardOnly(t *testing.T) {
	got := MergedTechToUser(nil)
	if len(got) != len(TechToUserStandard) {
		t.Fatalf("nil cfg: want %d entries, got %d", len(TechToUserStandard), len(got))
	}
	for i, want := range TechToUserStandard {
		if got[i] != want {
			t.Errorf("entry %d: want %q, got %q", i, want, got[i])
		}
	}
}

func TestMergedTechToUser_AppendsUserOnly(t *testing.T) {
	cfg := &config.Config{
		Escalations: config.EscalationsConfig{
			UserOnly: []string{"custom regulatory trigger"},
		},
	}
	got := MergedTechToUser(cfg)
	if len(got) != len(TechToUserStandard)+1 {
		t.Fatalf("want %d entries, got %d", len(TechToUserStandard)+1, len(got))
	}
	if got[len(got)-1] != "custom regulatory trigger" {
		t.Errorf("last entry should be the custom trigger, got %q", got[len(got)-1])
	}
}

func TestMergedTechToUser_DedupesCaseInsensitively(t *testing.T) {
	cfg := &config.Config{
		Escalations: config.EscalationsConfig{
			// First entry duplicates a standard, second is genuinely new.
			UserOnly: []string{"DB Engine Swap", "tenant migration freeze"},
		},
	}
	got := MergedTechToUser(cfg)
	if len(got) != len(TechToUserStandard)+1 {
		t.Fatalf("want %d entries (one new), got %d", len(TechToUserStandard)+1, len(got))
	}
	if got[len(got)-1] != "tenant migration freeze" {
		t.Errorf("appended entry should be the non-dup, got %q", got[len(got)-1])
	}
}

func TestMergedTechToUser_IgnoresEmptyAndWhitespace(t *testing.T) {
	cfg := &config.Config{
		Escalations: config.EscalationsConfig{
			UserOnly: []string{"   ", ""},
		},
	}
	got := MergedTechToUser(cfg)
	if len(got) != len(TechToUserStandard) {
		t.Fatalf("blank entries should be dropped; got %d", len(got))
	}
}

func TestMatchesTechToUser(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		want   bool
	}{
		{"empty reason", "", false},
		{"unrelated prose", "renamed a helper function", false},
		{"exact match",
			"DB engine swap from postgres to mysql", true},
		{"case-insensitive",
			"this is an IRREVERSIBLE DATA MIGRATION that needs sign-off", true},
		{"substring inside larger phrase",
			"introducing api paradigm shift via streaming endpoints", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := MatchesTechToUser(c.reason, nil); got != c.want {
				t.Errorf("MatchesTechToUser(%q): want %v, got %v", c.reason, c.want, got)
			}
		})
	}
}

func TestMatchesTechToUser_HonorsUserOnly(t *testing.T) {
	cfg := &config.Config{
		Escalations: config.EscalationsConfig{
			UserOnly: []string{"deprecate legacy webhook"},
		},
	}
	if !MatchesTechToUser("we plan to deprecate legacy webhook in Q3", cfg) {
		t.Errorf("project-extension trigger should match")
	}
	if MatchesTechToUser("we plan to deprecate legacy webhook in Q3", nil) {
		t.Errorf("nil cfg should not see the project extension")
	}
}

func TestMatchesProductToUser(t *testing.T) {
	cases := []struct {
		reason string
		want   bool
	}{
		{"feature without candidate goal — needs PM input", true},
		{"two-goal unresolvable conflict between retention and growth", true},
		{"explicit strategic redirection from CEO", true},
		{"minor copy fix", false},
		{"", false},
	}
	for _, c := range cases {
		if got := MatchesProductToUser(c.reason); got != c.want {
			t.Errorf("MatchesProductToUser(%q): want %v, got %v", c.reason, c.want, got)
		}
	}
}
