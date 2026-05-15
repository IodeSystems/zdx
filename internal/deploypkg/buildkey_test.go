package deploypkg

import (
	"crypto/ed25519"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildKeyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build-key.pem")

	priv, pub, err := GenerateBuildKey()
	if err != nil {
		t.Fatalf("GenerateBuildKey: %v", err)
	}
	if len(priv) != ed25519.PrivateKeySize {
		t.Fatalf("priv size = %d, want %d", len(priv), ed25519.PrivateKeySize)
	}
	if len(pub) != ed25519.PublicKeySize {
		t.Fatalf("pub size = %d, want %d", len(pub), ed25519.PublicKeySize)
	}

	if err := SaveBuildKey(path, priv); err != nil {
		t.Fatalf("SaveBuildKey: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("perms = %o, want 0600", perm)
	}

	loaded, err := LoadBuildKey(path)
	if err != nil {
		t.Fatalf("LoadBuildKey: %v", err)
	}
	if !loaded.Equal(priv) {
		t.Fatal("loaded key does not equal generated key")
	}

	fp1 := Fingerprint(pub)
	fp2 := Fingerprint(loaded.Public().(ed25519.PublicKey))
	if fp1 != fp2 {
		t.Fatalf("fingerprint mismatch: gen=%s loaded=%s", fp1, fp2)
	}
	if len(fp1) != fingerprintHexLen {
		t.Fatalf("fingerprint length = %d, want %d", len(fp1), fingerprintHexLen)
	}

	fpPriv, err := FingerprintPriv(loaded)
	if err != nil {
		t.Fatalf("FingerprintPriv: %v", err)
	}
	if fpPriv != fp1 {
		t.Fatalf("FingerprintPriv mismatch: %s vs %s", fpPriv, fp1)
	}
}

func TestSaveBuildKeyRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "build-key.pem")
	priv, _, err := GenerateBuildKey()
	if err != nil {
		t.Fatalf("GenerateBuildKey: %v", err)
	}
	if err := SaveBuildKey(path, priv); err != nil {
		t.Fatalf("SaveBuildKey first: %v", err)
	}
	if err := SaveBuildKey(path, priv); err == nil {
		t.Fatal("SaveBuildKey second call: want error, got nil")
	}
}

func TestLoadBuildKeyRejectsMissing(t *testing.T) {
	if _, err := LoadBuildKey(filepath.Join(t.TempDir(), "nope.pem")); err == nil {
		t.Fatal("LoadBuildKey on missing file: want error, got nil")
	}
}

func TestLoadBuildKeyRejectsNonPEM(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "garbage.pem")
	if err := os.WriteFile(path, []byte("not a pem block"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := LoadBuildKey(path); err == nil {
		t.Fatal("LoadBuildKey on non-PEM: want error, got nil")
	}
}

func TestFingerprintEmpty(t *testing.T) {
	if got := Fingerprint(nil); got != "" {
		t.Errorf("Fingerprint(nil) = %q, want empty", got)
	}
}

func TestFingerprintStability(t *testing.T) {
	// Known input → known output (locks the spec: hex(sha256(pub))[:32]).
	pub := ed25519.PublicKey(make([]byte, ed25519.PublicKeySize)) // all zero
	got := Fingerprint(pub)
	want := "66687aadf862bd776c8fc18b8e9f8e20" // hex(sha256(32 zero bytes))[:32]
	if got != want {
		t.Errorf("Fingerprint(zero pub) = %s, want %s", got, want)
	}
}
