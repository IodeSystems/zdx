package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func MustClient() *Client {
	c, err := DefaultClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	return c
}

func Fatal(err error) {
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
	os.Exit(1)
}

// GitRepoRoot shells out to `git rev-parse --show-toplevel`.
func GitRepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Truncate returns s if shorter than n, otherwise s[:n-3]+"...".
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
