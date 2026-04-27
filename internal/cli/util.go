package cli

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

// QuerySlug builds url.Values with the client's project slug.
func QuerySlug(c *Client) url.Values {
	return url.Values{"slug": {c.SlugOrDie()}}
}

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

// GitTreeDirty returns true if the working tree has staged, unstaged, or
// untracked changes.
func GitTreeDirty() bool {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return false // can't tell — don't block
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// Truncate returns s if shorter than n, otherwise s[:n-3]+"...".
func Truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// RunShell runs cmd via `/bin/sh -c` wired to the current stdio, optionally
// inside cwd. Used by spec run (cli) and build steps (devtools).
func RunShell(cmd, cwd string) error {
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if cwd != "" {
		c.Dir = cwd
	}
	return c.Run()
}
