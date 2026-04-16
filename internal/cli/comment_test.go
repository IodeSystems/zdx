package cli

import (
	"testing"
)

func TestResolveAuthorAlias(t *testing.T) {
	t.Run("flag wins over env", func(t *testing.T) {
		t.Setenv("DX_AUTHOR_ALIAS", "from-env")
		if got := resolveAuthorAlias("from-flag"); got != "from-flag" {
			t.Fatalf("want from-flag, got %q", got)
		}
	})

	t.Run("env fallback when flag empty", func(t *testing.T) {
		t.Setenv("DX_AUTHOR_ALIAS", "claude")
		if got := resolveAuthorAlias(""); got != "claude" {
			t.Fatalf("want claude, got %q", got)
		}
	})

	t.Run("empty when neither set", func(t *testing.T) {
		t.Setenv("DX_AUTHOR_ALIAS", "")
		if got := resolveAuthorAlias(""); got != "" {
			t.Fatalf("want empty, got %q", got)
		}
	})
}
