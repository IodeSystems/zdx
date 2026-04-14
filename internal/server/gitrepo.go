package server

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

// repoDir returns the local clone path for a project.
func (s *Server) repoDir(slug string) string {
	base := os.Getenv("REPO_DIR")
	if base == "" {
		if zdxHome := os.Getenv("ZDX_HOME"); zdxHome != "" {
			base = zdxHome + "/data/repos"
		} else {
			base = "repos"
		}
	}
	return filepath.Join(base, slug)
}

// ensureRepo clones the repo if absent, or fetches the latest otherwise.
// gitURL may embed a token: https://<token>@github.com/org/repo.git
func ensureRepo(dir, gitURL, branch string) error {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		// Already cloned — fetch.
		cmd := exec.Command("git", "-C", dir, "fetch", "--prune", "origin")
		cmd.Env = gitEnv()
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git fetch: %w\n%s", err, out)
		}
		return nil
	}
	// Clone.
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--no-tags", "--single-branch",
		"--branch", branch, gitURL, dir)
	cmd.Env = gitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, out)
	}
	return nil
}

// recentCommits returns the last n commits on the default remote branch.
func recentCommits(dir, branch string, n int) ([]GitCommit, error) {
	ref := "origin/" + branch
	args := []string{
		"-C", dir, "log", ref,
		fmt.Sprintf("-n%d", n),
		"--format=%H\t%h\t%s\t%an\t%ai",
	}
	cmd := exec.Command("git", args...)
	cmd.Env = gitEnv()
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

// gitLsRemote verifies reachability of gitURL and presence of branch without cloning.
// gitURL may embed a token: https://<token>@github.com/org/repo.git
func gitLsRemote(gitURL, branch string) error {
	if gitURL == "" {
		return fmt.Errorf("git URL is required")
	}
	if branch == "" {
		branch = "main"
	}
	cmd := exec.Command("git", "ls-remote", "--heads", "--exit-code", gitURL, branch)
	cmd.Env = gitEnv()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("branch %q not found on remote", branch)
	}
	return nil
}

// gitEnv returns a minimal env for git commands (no SSH agent forwarding issues).
func gitEnv() []string {
	// Inherit PATH and HOME; suppress interactive prompts.
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
