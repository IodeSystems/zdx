package cli

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/iodesystems/zdx-go/internal/dxclient"
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

// CollectBranchState collects current git state and returns a BranchState for
// terminal endpoint calls. Returns nil if claimBaseSHA is empty or git is unavailable.
func CollectBranchState(claimBaseSHA string) *dxclient.BranchState {
	if claimBaseSHA == "" {
		return nil
	}
	headOut, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return nil
	}
	branchOut, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return nil
	}
	porcelain, _ := exec.Command("git", "status", "--porcelain").Output()
	bs := &dxclient.BranchState{
		HeadSha:    strings.TrimSpace(string(headOut)),
		HeadBranch: strings.TrimSpace(string(branchOut)),
		TreeClean:  len(strings.TrimSpace(string(porcelain))) == 0,
	}
	logOut, _ := exec.Command("git", "log", claimBaseSHA+"..HEAD", "--format=%H").Output()
	if raw := strings.TrimSpace(string(logOut)); raw != "" {
		lines := strings.Split(raw, "\n")
		bs.CommitsSinceBase = &lines
	}
	return bs
}

// ClaimBase holds the git context recorded when a todo was claimed.
type ClaimBase struct {
	SHA    string
	Branch string
	Kind   string
}

// FetchClaimBase looks up the active claim for issueRef from the list-claims
// endpoint. Returns zero-value ClaimBase if unavailable.
func (c *Client) FetchClaimBase(slug, issueRef string) ClaimBase {
	resp, err := c.AgentListClaimsWithResponse(context.Background(), &dxclient.AgentListClaimsParams{Slug: slug})
	if err != nil || resp.JSON200 == nil || resp.JSON200.Todos == nil {
		return ClaimBase{}
	}
	for _, t := range *resp.JSON200.Todos {
		if t.IssueRef != issueRef {
			continue
		}
		base := ClaimBase{Kind: t.Kind}
		if t.ClaimBaseSha != nil {
			base.SHA = *t.ClaimBaseSha
		}
		if t.ClaimBaseBranch != nil {
			base.Branch = *t.ClaimBaseBranch
		}
		return base
	}
	return ClaimBase{}
}

// FetchClaimBaseSHA looks up the active claim for issueRef and returns the sha.
func (c *Client) FetchClaimBaseSHA(slug, issueRef string) string {
	return c.FetchClaimBase(slug, issueRef).SHA
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
