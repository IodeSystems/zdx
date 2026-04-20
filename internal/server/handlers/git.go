package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// SnippetResult is the payload returned by ReadSnippet.
type SnippetResult struct {
	Content    string `json:"content"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	TotalLines int    `json:"total_lines"`
	HashUsed   string `json:"hash_used"`
	Truncated  bool   `json:"truncated"`
}

// SnippetError distinguishes cases the handler layer maps to HTTP codes.
type SnippetError struct {
	Code string
	Msg  string
}

func (e *SnippetError) Error() string { return e.Msg }

const (
	SnippetErrBadPath     = "bad_path"
	SnippetErrHashMissing = "hash_not_in_checkout"
	SnippetErrFileMissing = "file_not_in_checkout"
	SnippetErrRead        = "read_error"
)

// maxSnippetBytes caps the response body to avoid accidental multi-MB returns.
const maxSnippetBytes = 256 * 1024

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
		cmd2 := exec.Command("git", "-C", dir, "reset", "--hard", "origin/"+branch)
		cmd2.Env = GitEnv()
		if out, err := cmd2.CombinedOutput(); err != nil {
			return fmt.Errorf("git reset: %w\n%s", err, out)
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

// HasRepo reports whether dir contains a git checkout.
func HasRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// safeRepoPath rejects absolute paths and parent-traversal segments. Returns
// the cleaned path or a SnippetError.
func safeRepoPath(p string) (string, error) {
	if p == "" {
		return "", &SnippetError{Code: SnippetErrBadPath, Msg: "path is required"}
	}
	if strings.HasPrefix(p, "/") || strings.Contains(p, "\x00") {
		return "", &SnippetError{Code: SnippetErrBadPath, Msg: "absolute path not allowed"}
	}
	cleaned := filepath.ToSlash(filepath.Clean(p))
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", &SnippetError{Code: SnippetErrBadPath, Msg: "path escapes repo"}
	}
	return cleaned, nil
}

// hashExists reports whether a commit/tree/blob object is in the local repo.
func hashExists(dir, ref string) bool {
	cmd := exec.Command("git", "-C", dir, "cat-file", "-e", ref)
	cmd.Env = GitEnv()
	return cmd.Run() == nil
}

// ReadSnippet returns the contents of path at commit hash, windowed to a line
// range with padding. hash may be empty; HEAD (or the supplied defaultRef) is
// used instead. On a missing hash, a single `git fetch` is attempted.
func ReadSnippet(dir, gitURL, defaultBranch, hash, path string, start, end, pad int) (*SnippetResult, error) {
	clean, err := safeRepoPath(path)
	if err != nil {
		return nil, err
	}
	if !HasRepo(dir) {
		if gitURL == "" {
			return nil, &SnippetError{Code: SnippetErrRead, Msg: "no local checkout and no git url"}
		}
		if err := EnsureRepo(dir, gitURL, defaultBranch); err != nil {
			return nil, &SnippetError{Code: SnippetErrRead, Msg: "clone: " + err.Error()}
		}
	}
	ref := hash
	if ref == "" {
		ref = "HEAD"
	}
	if hash != "" && !hashExists(dir, hash+"^{commit}") {
		if gitURL != "" {
			// Try a targeted fetch first (works on servers with uploadpack.allowAnySHA1InWant).
			fetch := exec.Command("git", "-C", dir, "fetch", "--no-tags", "origin", hash)
			fetch.Env = GitEnv()
			_ = fetch.Run()
			// GitHub does not support fetching by SHA, so fall back to a full fetch.
			if !hashExists(dir, hash+"^{commit}") {
				full := exec.Command("git", "-C", dir, "fetch", "--prune", "origin")
				full.Env = GitEnv()
				_ = full.Run()
			}
		}
		if !hashExists(dir, hash+"^{commit}") {
			return nil, &SnippetError{Code: SnippetErrHashMissing, Msg: "commit " + hash + " not in local checkout"}
		}
	}
	show := exec.Command("git", "-C", dir, "show", ref+":"+clean)
	show.Env = GitEnv()
	out, err := show.Output()
	if err != nil {
		return nil, &SnippetError{Code: SnippetErrFileMissing, Msg: "file " + clean + " not found at " + ref}
	}
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	total := len(lines)
	if start <= 0 {
		start = 1
	}
	if end <= 0 {
		end = start
	}
	if end < start {
		end = start
	}
	if pad < 0 {
		pad = 0
	}
	lo := start - pad
	if lo < 1 {
		lo = 1
	}
	hi := end + pad
	if hi > total {
		hi = total
	}
	if lo > total {
		lo = total
	}
	window := strings.Join(lines[lo-1:hi], "\n")
	truncated := false
	if len(window) > maxSnippetBytes {
		window = window[:maxSnippetBytes]
		truncated = true
	}
	used := hash
	if used == "" {
		revParse := exec.Command("git", "-C", dir, "rev-parse", ref)
		revParse.Env = GitEnv()
		if h, err := revParse.Output(); err == nil {
			used = strings.TrimSpace(string(h))
		}
	}
	return &SnippetResult{
		Content:    window,
		StartLine:  lo,
		EndLine:    hi,
		TotalLines: total,
		HashUsed:   used,
		Truncated:  truncated,
	}, nil
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
