package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitCommit represents a single git log entry.
type GitCommit struct {
	SHA     string `json:"sha"`
	Short   string `json:"short"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

// GitEnv returns a minimal env for git commands (no SSH agent forwarding issues).
func GitEnv() []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
		"GIT_TERMINAL_PROMPT=0",
	}
	if sshAuth := os.Getenv("SSH_AUTH_SOCK"); sshAuth != "" {
		env = append(env, "SSH_AUTH_SOCK="+sshAuth)
	}
	return env
}

// EnsureRepo clones the repo if absent, or fetches the latest otherwise.
// gitURL may embed a token: https://<token>@github.com/org/repo.git
func EnsureRepo(dir, gitURL, branch string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		cmd := exec.Command("git", "-C", dir, "fetch", "--prune", "origin")
		cmd.Env = GitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch: %w\n%s", err, out)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--no-tags", "--single-branch",
		"--branch", branch, gitURL, dir)
	cmd.Env = GitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}
	return nil
}

// RecentCommits returns the last n commits on the default remote branch.
func RecentCommits(dir, branch string, n int) ([]GitCommit, error) {
	ref := "origin/" + branch
	args := []string{
		"-C", dir, "log", ref,
		fmt.Sprintf("-n%d", n),
		"--format=%H\t%h\t%s\t%an\t%ai",
	}
	cmd := exec.Command("git", args...)
	cmd.Env = GitEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	var commits []GitCommit
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 5 {
			continue
		}
		commits = append(commits, GitCommit{
			SHA:     parts[0],
			Short:   parts[1],
			Message: parts[2],
			Author:  parts[3],
			Date:    parts[4],
		})
	}
	return commits, nil
}
