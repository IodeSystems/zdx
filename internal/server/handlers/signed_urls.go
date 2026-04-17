package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// SignAssetURL returns "path?sig=<hex>&exp=<unix>" using HMAC-SHA256(secret, path+"\n"+exp).
// The browser can hit the resulting URL with no auth header until exp passes.
// Used for asset endpoints (file downloads, demo artifacts) where <video src> and bare
// fetch can't supply X-Api-Key.
func SignAssetURL(secret, path string, ttl time.Duration) string {
	if secret == "" {
		return path
	}
	exp := time.Now().Add(ttl).Unix()
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%d", path, exp)
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s?sig=%s&exp=%d", path, sig, exp)
}

// VerifyAssetURL re-signs r.URL.Path with the exp query param and constant-time
// compares against the sig query param. Returns true only when the signature matches
// and exp is in the future.
func VerifyAssetURL(secret string, r *http.Request) bool {
	if secret == "" {
		return false
	}
	q := r.URL.Query()
	sig := q.Get("sig")
	expStr := q.Get("exp")
	if sig == "" || expStr == "" {
		return false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > exp {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s\n%d", r.URL.Path, exp)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
