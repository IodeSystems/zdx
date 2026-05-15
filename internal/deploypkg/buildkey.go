// Package deploypkg implements the deploy-package format used by dx-server
// to ship signed binaries+migrations to dx-envd. This file owns the build-key
// lifecycle: generate, save, load, fingerprint.
package deploypkg

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

const (
	// pemTypeBuildKey is the PEM block type used for the Ed25519 build-key
	// private key. PKCS#8 wraps the Ed25519 seed in a standard envelope so
	// other tools (openssl pkey -in ...) can read it without custom parsing.
	pemTypeBuildKey = "PRIVATE KEY"

	// fingerprintHexLen is 16 bytes of the sha256 digest, hex-encoded (32
	// chars). Long enough to be globally unique in practice, short enough to
	// fit in manifest.signed_by and CLI output without wrapping.
	fingerprintHexLen = 32
)

// GenerateBuildKey returns a fresh Ed25519 keypair suitable for signing
// deploy packages. The private key includes the public key bytes per the
// crypto/ed25519 layout.
func GenerateBuildKey() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("deploypkg: generate ed25519 key: %w", err)
	}
	return priv, pub, nil
}

// SaveBuildKey writes priv to path as a PKCS#8-wrapped PEM block with 0600
// permissions. Refuses to overwrite an existing file (callers must delete
// first if they really mean to rotate).
func SaveBuildKey(path string, priv ed25519.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("deploypkg: marshal pkcs8: %w", err)
	}
	block := &pem.Block{Type: pemTypeBuildKey, Bytes: der}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("deploypkg: create %s: %w", path, err)
	}
	defer f.Close()
	if err := pem.Encode(f, block); err != nil {
		return fmt.Errorf("deploypkg: encode pem to %s: %w", path, err)
	}
	return nil
}

// LoadBuildKey reads a PEM-encoded Ed25519 private key from path. Accepts
// PKCS#8 PRIVATE KEY blocks (what SaveBuildKey writes). Errors if the file
// is missing, the PEM is malformed, or the contained key is not Ed25519.
func LoadBuildKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("deploypkg: read %s: %w", path, err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("deploypkg: %s: no PEM block found", path)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("deploypkg: parse pkcs8 in %s: %w", path, err)
	}
	priv, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("deploypkg: %s: key is %T, want ed25519.PrivateKey", path, key)
	}
	return priv, nil
}

// Fingerprint returns the canonical short identifier for pub: hex-encoded
// sha256(pub) truncated to 16 bytes (32 hex chars). Stable across load
// cycles and across hosts. Used as manifest.signed_by and as the lookup key
// in dx-envd's pinned-pubkey trust list.
func Fingerprint(pub ed25519.PublicKey) string {
	if len(pub) == 0 {
		return ""
	}
	sum := sha256.Sum256(pub)
	return hex.EncodeToString(sum[:])[:fingerprintHexLen]
}

// FingerprintPriv is a convenience for callers that hold only the private
// half — extracts the embedded public key and fingerprints it.
func FingerprintPriv(priv ed25519.PrivateKey) (string, error) {
	if len(priv) == 0 {
		return "", errors.New("deploypkg: empty private key")
	}
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return "", fmt.Errorf("deploypkg: priv.Public() is %T, want ed25519.PublicKey", priv.Public())
	}
	return Fingerprint(pub), nil
}
