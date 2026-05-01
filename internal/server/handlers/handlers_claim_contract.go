package handlers

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/iodesystems/zdx-go/internal/todoclaim"
)

// BranchState is the optional client-reported git context sent on terminal endpoints.
type BranchState struct {
	HeadSHA          string   `json:"head_sha"`
	HeadBranch       string   `json:"head_branch"`
	TreeClean        bool     `json:"tree_clean"`
	CommitsSinceBase []string `json:"commits_since_base,omitempty"`
}

// validateClaimContract checks branch_state against the contract derived from kind.
// Returns a 422 error if violated. Skips validation when bs is nil or no claim was
// recorded (claimBaseSha empty).
func validateClaimContract(kind, claimBaseSha, claimBaseBranch string, bs *BranchState) error {
	if bs == nil || claimBaseSha == "" {
		return nil
	}
	switch todoclaim.ForKind(kind).Name {
	case todoclaim.Dev:
		return validateDevContract(claimBaseSha, claimBaseBranch, bs)
	default:
		return validateMetadataContract(claimBaseSha, claimBaseBranch, bs)
	}
}

func validateMetadataContract(claimBaseSha, claimBaseBranch string, bs *BranchState) error {
	var violations []string
	if !bs.TreeClean {
		violations = append(violations, "tree must be clean (no uncommitted changes)")
	}
	if bs.HeadBranch != claimBaseBranch {
		violations = append(violations, fmt.Sprintf("branch mismatch: claimed on %q, currently on %q", claimBaseBranch, bs.HeadBranch))
	}
	if bs.HeadSHA != claimBaseSha {
		violations = append(violations, fmt.Sprintf("SHA mismatch: claimed at %.8s, currently at %.8s (metadata todos must not commit to the repo)", claimBaseSha, bs.HeadSHA))
	}
	if len(violations) > 0 {
		return apiErr(http.StatusUnprocessableEntity, "contract violation ("+todoclaim.Metadata+"): "+strings.Join(violations, "; "))
	}
	return nil
}

func validateDevContract(claimBaseSha, claimBaseBranch string, bs *BranchState) error {
	var violations []string
	if !bs.TreeClean {
		violations = append(violations, "tree must be clean (no uncommitted changes)")
	}
	if bs.HeadBranch != claimBaseBranch {
		violations = append(violations, fmt.Sprintf("branch mismatch: claimed on %q, currently on %q", claimBaseBranch, bs.HeadBranch))
	}
	if len(bs.CommitsSinceBase) == 0 {
		violations = append(violations, "dev todos require at least one commit since the claim base")
	}
	if len(violations) > 0 {
		return apiErr(http.StatusUnprocessableEntity, "contract violation ("+todoclaim.Dev+"): "+strings.Join(violations, "; "))
	}
	// Ancestor check: claimBaseSha must be reachable from HeadSHA.
	// Best-effort: skip if git unavailable (merge-train rebase will expose lies).
	if bs.HeadSHA != "" && bs.HeadSHA != claimBaseSha {
		cmd := exec.Command("git", "merge-base", "--is-ancestor", claimBaseSha, bs.HeadSHA)
		if err := cmd.Run(); err != nil {
			return apiErr(http.StatusUnprocessableEntity,
				fmt.Sprintf("contract violation (%s): claim base %.8s is not an ancestor of current HEAD %.8s; history may have been rewritten",
					todoclaim.Dev, claimBaseSha, bs.HeadSHA))
		}
	}
	return nil
}
