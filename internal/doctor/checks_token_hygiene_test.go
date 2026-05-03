package doctor

import (
	"strings"
	"testing"
	"time"
)

// TestPartitionAdminScopedTokens covers the three buckets of admin-scoped
// keys: agent-* (always legacy), recently-used non-agent (info), and silent
// (older or never-used non-agent).
func TestPartitionAdminScopedTokens(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	tokens := []AdminScopedToken{
		{ID: 1, Name: "agent-legacy", LastUsedAt: now.Add(-30 * 24 * time.Hour)},
		{ID: 2, Name: "agent-stale", LastUsedAt: time.Time{}}, // never used, still legacy
		{ID: 3, Name: "admin-cli", LastUsedAt: now.Add(-1 * time.Hour)},
		{ID: 4, Name: "admin-cli-old", LastUsedAt: now.Add(-30 * 24 * time.Hour)}, // older than 7d
		{ID: 5, Name: "admin-unused", LastUsedAt: time.Time{}},                    // silent: non-agent, never used
	}

	legacy, recent := PartitionAdminScopedTokens(tokens, now)

	if got, want := len(legacy), 2; got != want {
		t.Fatalf("legacy: got %d, want %d (%v)", got, want, legacy)
	}
	for _, tok := range legacy {
		if !strings.HasPrefix(tok.Name, "agent-") {
			t.Errorf("non-agent token %q in legacy bucket", tok.Name)
		}
	}

	if got, want := len(recent), 1; got != want {
		t.Fatalf("recent: got %d, want %d (%v)", got, want, recent)
	}
	if recent[0].Name != "admin-cli" {
		t.Errorf("recent[0] = %q, want %q", recent[0].Name, "admin-cli")
	}
}

// TestPartitionAdminScopedTokens_BoundaryAt7Days verifies the inclusive 7-day
// edge — a token used exactly 7 days ago still counts as recent.
func TestPartitionAdminScopedTokens_BoundaryAt7Days(t *testing.T) {
	now := time.Date(2026, 5, 2, 12, 0, 0, 0, time.UTC)
	tokens := []AdminScopedToken{
		{ID: 1, Name: "admin-edge", LastUsedAt: now.Add(-7 * 24 * time.Hour)},
		{ID: 2, Name: "admin-just-past", LastUsedAt: now.Add(-7*24*time.Hour - time.Second)},
	}
	_, recent := PartitionAdminScopedTokens(tokens, now)
	if len(recent) != 1 || recent[0].Name != "admin-edge" {
		t.Errorf("recent = %v, want [admin-edge]", recent)
	}
}

// TestTokenHygieneRung verifies the no_legacy_agent_tokens and
// admin_tokens_quiet checks evaluate from ProjectState.
func TestTokenHygieneRung(t *testing.T) {
	t.Run("rung exists in every classification", func(t *testing.T) {
		for _, c := range AllClassifications {
			vine := Vine(c)
			var found bool
			for _, r := range vine {
				if r.Name != "token_hygiene" {
					continue
				}
				found = true
				if len(r.Checks) != 2 {
					t.Errorf("Vine(%s) token_hygiene has %d checks, want 2", c, len(r.Checks))
				}
			}
			if !found {
				t.Errorf("Vine(%s) missing token_hygiene rung", c)
			}
		}
	})

	t.Run("no_legacy_agent_tokens/pass when slice empty", func(t *testing.T) {
		findings := Evaluate(&ProjectState{})
		f, ok := findFinding(findings, "no_legacy_agent_tokens")
		if !ok {
			t.Fatal("no_legacy_agent_tokens check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("no_legacy_agent_tokens/fail with proposal naming the token", func(t *testing.T) {
		findings := Evaluate(&ProjectState{LegacyAgentTokens: []string{"agent-foo"}})
		f, ok := findFinding(findings, "no_legacy_agent_tokens")
		if !ok {
			t.Fatal("no_legacy_agent_tokens check not found")
		}
		if f.Status != StatusFail {
			t.Fatalf("want fail, got %s", f.Status)
		}
		if !strings.Contains(f.Message, "agent-foo") {
			t.Errorf("message missing token name: %q", f.Message)
		}
		if f.Check.Action != ActionPropose {
			t.Errorf("want ActionPropose, got %v", f.Check.Action)
		}
		if f.Proposal == "" {
			t.Error("expected non-empty Proposal")
		}
		if !strings.Contains(f.Proposal, "dx token mint") {
			t.Errorf("proposal missing rotation command: %q", f.Proposal)
		}
		if f.FixFunc != nil {
			t.Error("rotation requires human action — no FixFunc")
		}
	})

	t.Run("no_legacy_agent_tokens/truncates sample at 3 with remaining count", func(t *testing.T) {
		findings := Evaluate(&ProjectState{LegacyAgentTokens: []string{
			"agent-a", "agent-b", "agent-c", "agent-d", "agent-e",
		}})
		f, _ := findFinding(findings, "no_legacy_agent_tokens")
		if !strings.Contains(f.Message, "+2 more") {
			t.Errorf("expected '+2 more' suffix, got: %q", f.Message)
		}
		if strings.Contains(f.Message, "agent-d") {
			t.Errorf("expected agent-d to be truncated: %q", f.Message)
		}
	})

	t.Run("admin_tokens_quiet/pass when slice empty", func(t *testing.T) {
		findings := Evaluate(&ProjectState{})
		f, ok := findFinding(findings, "admin_tokens_quiet")
		if !ok {
			t.Fatal("admin_tokens_quiet check not found")
		}
		if f.Status != StatusPass {
			t.Errorf("want pass, got %s: %s", f.Status, f.Message)
		}
	})

	t.Run("admin_tokens_quiet/fail informational with token names", func(t *testing.T) {
		findings := Evaluate(&ProjectState{RecentAdminTokens: []string{"admin-cli"}})
		f, ok := findFinding(findings, "admin_tokens_quiet")
		if !ok {
			t.Fatal("admin_tokens_quiet check not found")
		}
		if f.Status != StatusFail {
			t.Fatalf("want fail, got %s", f.Status)
		}
		if !strings.Contains(f.Message, "admin-cli") {
			t.Errorf("message missing token name: %q", f.Message)
		}
		if f.Check.Action != ActionInfo {
			t.Errorf("want ActionInfo, got %v", f.Check.Action)
		}
		if f.Proposal != "" {
			t.Errorf("info-only check must not propose, got %q", f.Proposal)
		}
		if f.FixFunc != nil {
			t.Error("info-only check must not have FixFunc")
		}
	})
}
