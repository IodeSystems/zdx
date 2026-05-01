package ship

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSecretsFile_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deploy.secret.properties")
	body := "deploy.host=ubuntu@example.com\nhz.token=abc123\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSecretsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["deploy.host"] != "ubuntu@example.com" {
		t.Errorf("deploy.host = %q, want ubuntu@example.com", got["deploy.host"])
	}
	if got["hz.token"] != "abc123" {
		t.Errorf("hz.token = %q, want abc123", got["hz.token"])
	}
}

func TestLoadSecretsFile_SkipsCommentsAndBlankLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.properties")
	body := "# top comment\n\n  \nkey1=v1\n! bang comment\nkey2=v2\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSecretsFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d keys, want 2: %#v", len(got), got)
	}
	if got["key1"] != "v1" || got["key2"] != "v2" {
		t.Errorf("unexpected values: %#v", got)
	}
}

func TestLoadSecretsFile_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.properties")
	body := "  key  =  value with spaces  \n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSecretsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["key"] != "value with spaces" {
		t.Errorf("got %q, want %q", got["key"], "value with spaces")
	}
}

func TestLoadSecretsFile_ValueWithEqualsKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.properties")
	body := "url=postgres://user:pass@host/db?sslmode=disable\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSecretsFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got["url"] != "postgres://user:pass@host/db?sslmode=disable" {
		t.Errorf("got %q", got["url"])
	}
}

func TestLoadSecretsFile_MissingFileReturnsEmpty(t *testing.T) {
	got, err := LoadSecretsFile(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %#v", got)
	}
}

func TestLoadSecretsFile_EmptyPathReturnsEmpty(t *testing.T) {
	got, err := LoadSecretsFile("")
	if err != nil {
		t.Fatalf("expected nil error for empty path, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %#v", got)
	}
}

func TestLoadSecretsFile_MalformedLineErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.properties")
	if err := os.WriteFile(path, []byte("no_equals_sign\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadSecretsFile(path)
	if err == nil {
		t.Fatal("expected error for malformed line, got nil")
	}
}
