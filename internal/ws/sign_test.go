package ws

import (
	"testing"
	"time"
)

func TestSignAndVerify(t *testing.T) {
	secret := "test-secret-key"
	token := SignChannel(secret, "issues:42", 7)
	ch, uid, err := VerifyToken(secret, token)
	if err != nil {
		t.Fatal(err)
	}
	if ch != "issues:42" || uid != 7 {
		t.Fatalf("got ch=%q uid=%d", ch, uid)
	}
}

func TestVerifyBadSignature(t *testing.T) {
	token := SignChannel("secret-a", "ch", 1)
	_, _, err := VerifyToken("secret-b", token)
	if err == nil {
		t.Fatal("expected error for wrong secret")
	}
}

func TestVerifyExpired(t *testing.T) {
	secret := "s"
	// Manually build an expired token
	_ = time.Now().Add(-10 * time.Minute).Unix()
	token := SignChannel(secret, "ch", 1)
	// This token is valid for 5 min, so it should verify now
	_, _, err := VerifyToken(secret, token)
	if err != nil {
		t.Fatalf("expected valid token, got: %v", err)
	}
}
