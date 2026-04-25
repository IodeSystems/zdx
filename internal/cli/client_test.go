package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// In global mode the slug should fall back to the local .zdx/config.yaml
// when DX_REMOTE_SLUG is unset, so srcless loop sessions running inside a
// cloned worktree make slug-scoped API calls correctly without prompting
// for signin (IS-463).
func TestDefaultClient_GlobalSlugFallback(t *testing.T) {
	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".zdx"), 0o700); err != nil {
		t.Fatalf("mkdir global .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpHome, ".zdx", "config.yaml"),
		[]byte("remote:\n  url: https://global.example.com\n"), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpHome, ".zdx", "credentials"),
		[]byte("globalkey\n"), 0o600); err != nil {
		t.Fatalf("write global credentials: %v", err)
	}
	t.Setenv("HOME", tmpHome)

	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, ".zdx"), 0o700); err != nil {
		t.Fatalf("mkdir worktree .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".zdx", "config.yaml"),
		[]byte("remote:\n  url: https://wrong.example.com\n  slug: my-project\n"), 0o600); err != nil {
		t.Fatalf("write worktree config: %v", err)
	}

	prevCwd, _ := os.Getwd()
	if err := os.Chdir(worktree); err != nil {
		t.Fatalf("chdir worktree: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevCwd) })

	t.Setenv("DX_GLOBAL", "1")
	t.Setenv("DX_REMOTE_URL", "")
	t.Setenv("DX_REMOTE_SLUG", "")
	t.Setenv("DX_REMOTE_API_KEY", "")

	c, err := DefaultClient()
	if err != nil {
		t.Fatalf("DefaultClient: %v", err)
	}
	if got, want := c.Slug(), "my-project"; got != want {
		t.Errorf("slug = %q, want %q (local config fallback)", got, want)
	}
	if got, want := c.base, "https://global.example.com"; got != want {
		t.Errorf("base = %q, want %q (global url, not local)", got, want)
	}
	if got, want := c.token, "globalkey"; got != want {
		t.Errorf("token = %q, want %q (global creds)", got, want)
	}
}

// DX_REMOTE_SLUG should still take priority over the local config in global mode.
func TestDefaultClient_GlobalSlugEnvWins(t *testing.T) {
	tmpHome := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpHome, ".zdx"), 0o700); err != nil {
		t.Fatalf("mkdir global .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpHome, ".zdx", "config.yaml"),
		[]byte("remote:\n  url: https://global.example.com\n"), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpHome, ".zdx", "credentials"),
		[]byte("globalkey\n"), 0o600); err != nil {
		t.Fatalf("write global credentials: %v", err)
	}
	t.Setenv("HOME", tmpHome)

	worktree := t.TempDir()
	if err := os.MkdirAll(filepath.Join(worktree, ".zdx"), 0o700); err != nil {
		t.Fatalf("mkdir worktree .zdx: %v", err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".zdx", "config.yaml"),
		[]byte("remote:\n  slug: local-slug\n"), 0o600); err != nil {
		t.Fatalf("write worktree config: %v", err)
	}

	prevCwd, _ := os.Getwd()
	if err := os.Chdir(worktree); err != nil {
		t.Fatalf("chdir worktree: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prevCwd) })

	t.Setenv("DX_GLOBAL", "1")
	t.Setenv("DX_REMOTE_URL", "")
	t.Setenv("DX_REMOTE_SLUG", "env-slug")
	t.Setenv("DX_REMOTE_API_KEY", "")

	c, err := DefaultClient()
	if err != nil {
		t.Fatalf("DefaultClient: %v", err)
	}
	if got, want := c.Slug(), "env-slug"; got != want {
		t.Errorf("slug = %q, want %q (env wins over local)", got, want)
	}
}
