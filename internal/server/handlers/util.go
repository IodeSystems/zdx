package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

// integrationTokenPrefix is the human-visible lead-in so secrets are easy to
// grep for in logs and distinguish from other credential formats.
const integrationTokenPrefix = "zdxk_"

// GenerateIntegrationToken returns a fresh opaque bearer token. The prefix
// makes leaks easy to spot; the 32 random bytes (64 hex chars) give 256 bits
// of entropy, well beyond what's needed for an auth token.
func GenerateIntegrationToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return integrationTokenPrefix + hex.EncodeToString(raw[:]), nil
}

// HashIntegrationToken returns the sha256 hex digest stored in the DB.
// Tokens are never stored in plaintext; only the hash and a short prefix.
func HashIntegrationToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// TokenPrefix returns the first 12 chars of the token for display/lookup.
func TokenPrefix(token string) string {
	if len(token) < 12 {
		return token
	}
	return token[:12]
}

// GitLsRemote performs a branch-listing probe against gitURL to validate
// that it is reachable and that the named branch exists on the remote.
func GitLsRemote(gitURL, branch string) error {
	if gitURL == "" {
		return fmt.Errorf("git URL is required")
	}
	if branch == "" {
		branch = "main"
	}
	cmd := exec.Command("git", "ls-remote", "--heads", "--exit-code", gitURL, branch)
	cmd.Env = GitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("branch %q not found on remote", branch)
	}
	return nil
}
