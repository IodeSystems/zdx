package ws

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const tokenTTL = 5 * time.Minute

func GenerateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Token format: base64(channel)|userID|exp.signature
func SignChannel(secret, channel string, userID int64) string {
	exp := time.Now().Add(tokenTTL).Unix()
	payload := fmt.Sprintf("%s|%d|%d", channel, userID, exp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig)
}

func VerifyToken(secret, token string) (channel string, userID int64, err error) {
	idx := strings.LastIndex(token, ".")
	if idx < 0 {
		return "", 0, fmt.Errorf("malformed token")
	}
	payload := token[:idx]
	sig := token[idx+1:]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", 0, fmt.Errorf("invalid signature")
	}

	parts := strings.Split(payload, "|")
	if len(parts) != 3 {
		return "", 0, fmt.Errorf("malformed payload")
	}
	channel = parts[0]
	userID, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid user_id")
	}
	exp, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		return "", 0, fmt.Errorf("invalid expiry")
	}
	if time.Now().Unix() > exp {
		return "", 0, fmt.Errorf("token expired")
	}
	return channel, userID, nil
}
