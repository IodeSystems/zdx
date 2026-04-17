package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestSignAssetURL_RoundTrip(t *testing.T) {
	const secret = "supersecret"
	signed := SignAssetURL(secret, "/api/files/42", time.Hour)
	if !strings.HasPrefix(signed, "/api/files/42?sig=") {
		t.Fatalf("signed URL missing sig: %s", signed)
	}

	r := httptest.NewRequest("GET", signed, nil)
	if !VerifyAssetURL(secret, r) {
		t.Fatalf("VerifyAssetURL rejected freshly-signed URL: %s", signed)
	}
}

func TestVerifyAssetURL_RejectsExpired(t *testing.T) {
	const secret = "supersecret"
	signed := SignAssetURL(secret, "/api/files/42", -1*time.Second)
	r := httptest.NewRequest("GET", signed, nil)
	if VerifyAssetURL(secret, r) {
		t.Fatal("VerifyAssetURL accepted expired URL")
	}
}

func TestVerifyAssetURL_RejectsTamperedPath(t *testing.T) {
	const secret = "supersecret"
	signed := SignAssetURL(secret, "/api/files/42", time.Hour)
	tampered := strings.Replace(signed, "/api/files/42", "/api/files/43", 1)
	r := httptest.NewRequest("GET", tampered, nil)
	if VerifyAssetURL(secret, r) {
		t.Fatal("VerifyAssetURL accepted URL with tampered path")
	}
}

func TestVerifyAssetURL_RejectsBadSecret(t *testing.T) {
	signed := SignAssetURL("secret-A", "/api/files/42", time.Hour)
	r := httptest.NewRequest("GET", signed, nil)
	if VerifyAssetURL("secret-B", r) {
		t.Fatal("VerifyAssetURL accepted URL signed by a different secret")
	}
}

func TestVerifyAssetURL_EmptySecret(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/files/42?sig=abc&exp=99999999999", nil)
	if VerifyAssetURL("", r) {
		t.Fatal("VerifyAssetURL accepted empty secret")
	}
}

func TestVerifyAssetURL_MissingParams(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/files/42", nil)
	if VerifyAssetURL("secret", r) {
		t.Fatal("VerifyAssetURL accepted URL without sig/exp")
	}
}
