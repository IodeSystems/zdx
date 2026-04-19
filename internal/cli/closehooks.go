package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iodesystems/zdx-go/internal/config"
)

const closeCachePath = ".zdx/.close-cache"

// RunCloseHooks executes all close hooks defined in .zdx/config.yaml.
// Called when an issue or task is closed via the CLI. Skips when the git
// worktree hash matches the last successful run (see worktreeHash).
func RunCloseHooks() error {
	cfg := config.Load()
	if cfg == nil {
		return nil
	}
	steps := cfg.AllCloseSteps("")
	if len(steps) == 0 {
		return nil
	}

	root, rootErr := GitRepoRoot()
	hash, hashErr := worktreeHash(root)
	if rootErr == nil && hashErr == nil {
		if cached, _ := os.ReadFile(filepath.Join(root, closeCachePath)); strings.TrimSpace(string(cached)) == hash {
			fmt.Printf("[close] skipped (tree unchanged since %s)\n", hash[:12])
			return nil
		}
	}

	for _, ns := range steps {
		label := ns.Name
		if label == "" {
			label = ns.Run
		}
		fmt.Printf("[close] %s: %s\n", ns.Component, label)
		if err := RunShell(ns.Run, ns.CWD); err != nil {
			return fmt.Errorf("close hook %q failed: %w", label, err)
		}
	}

	if rootErr == nil && hashErr == nil {
		writeCloseCache(root, hash)
	}
	return nil
}

// worktreeHash returns a stable hash of HEAD + uncommitted changes + untracked
// files, so the close-hook cache invalidates whenever anything lint-relevant
// would have changed.
func worktreeHash(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("no git root")
	}
	head, err := runGit(root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	diff, err := runGit(root, "diff", "HEAD")
	if err != nil {
		return "", err
	}
	untrackedList, err := runGit(root, "ls-files", "-o", "--exclude-standard")
	if err != nil {
		return "", err
	}

	h := sha256.New()
	h.Write([]byte(head))
	h.Write([]byte{0})
	h.Write([]byte(diff))
	h.Write([]byte{0})
	for _, p := range strings.Split(untrackedList, "\n") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		if data, rerr := os.ReadFile(filepath.Join(root, p)); rerr == nil {
			h.Write(data)
		}
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func runGit(root string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
}

func writeCloseCache(root, hash string) {
	path := filepath.Join(root, closeCachePath)
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	_ = os.WriteFile(path, []byte(hash+"\n"), 0o644)
}
