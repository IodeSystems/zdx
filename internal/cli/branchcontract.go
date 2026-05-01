package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/todoclaim"
)

// ValidateBranchState checks bs against the contract for kind and returns an
// actionable error if violated. Returns nil when baseSHA is "" (no claim).
func ValidateBranchState(kind string, bs *dxclient.BranchState, baseSHA, baseBranch string) error {
	if baseSHA == "" || bs == nil {
		return nil
	}
	switch todoclaim.ForKind(kind).Name {
	case todoclaim.Dev:
		return validateDevContract(kind, bs, baseSHA, baseBranch)
	default:
		return validateMetadataContract(kind, bs, baseSHA, baseBranch)
	}
}

func validateDevContract(kind string, bs *dxclient.BranchState, baseSHA, baseBranch string) error {
	var violations []string
	if !bs.TreeClean {
		violations = append(violations, "tree must be clean (no uncommitted changes)")
	}
	if bs.HeadBranch != baseBranch {
		violations = append(violations, fmt.Sprintf("branch mismatch: claimed on %q, currently on %q", baseBranch, bs.HeadBranch))
	}
	commitCount := 0
	if bs.CommitsSinceBase != nil {
		commitCount = len(*bs.CommitsSinceBase)
	}
	if commitCount == 0 {
		violations = append(violations, "dev todos require at least one commit since the claim base")
	}
	if len(violations) > 0 {
		return contractError(todoclaim.Dev, violations, bs, baseSHA, baseBranch)
	}
	// Ancestor check: baseSHA must be reachable from HeadSHA.
	if bs.HeadSha != "" && bs.HeadSha != baseSHA {
		cmd := exec.Command("git", "merge-base", "--is-ancestor", baseSHA, bs.HeadSha)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("contract violation (%s): claim base %.8s is not an ancestor of current HEAD %.8s; history may have been rewritten\nfix: git reset --hard %s, or dx todo release to abandon.",
				todoclaim.Dev, baseSHA, bs.HeadSha, baseSHA)
		}
	}
	return nil
}

func validateMetadataContract(kind string, bs *dxclient.BranchState, baseSHA, baseBranch string) error {
	var violations []string
	if !bs.TreeClean {
		violations = append(violations, "tree must be clean (no uncommitted changes)")
	}
	if bs.HeadBranch != baseBranch {
		violations = append(violations, fmt.Sprintf("branch mismatch: claimed on %q, currently on %q", baseBranch, bs.HeadBranch))
	}
	if bs.HeadSha != baseSHA {
		violations = append(violations, fmt.Sprintf("SHA mismatch: claimed at %.8s, currently at %.8s (metadata todos must not commit to the repo)", baseSHA, bs.HeadSha))
	}
	if len(violations) > 0 {
		return contractError(todoclaim.Metadata, violations, bs, baseSHA, baseBranch)
	}
	return nil
}

func contractError(contract string, violations []string, bs *dxclient.BranchState, baseSHA, baseBranch string) error {
	commitCount := 0
	if bs.CommitsSinceBase != nil {
		commitCount = len(*bs.CommitsSinceBase)
	}
	dirtyFlag := "clean"
	if !bs.TreeClean {
		dirtyFlag = "dirty"
	}
	status := fmt.Sprintf("%d commit(s) since base, tree %s", commitCount, dirtyFlag)
	msg := fmt.Sprintf("%s requires the branch to satisfy: %s.\n  base:   %.8s on %s\n  head:   %.8s on %s\n  status: %s\nfix: git stash && git reset --hard %s, or dx todo release to abandon.",
		contract,
		strings.Join(violations, "; "),
		baseSHA, baseBranch,
		bs.HeadSha, bs.HeadBranch,
		status,
		baseSHA,
	)
	return fmt.Errorf("%s", msg)
}
