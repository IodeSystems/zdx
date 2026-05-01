package handlers

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
)

// statusAndMessage extracts the HTTP status and message from a huma error.
// Returns (0, "") when err is nil.
func statusAndMessage(t *testing.T, err error) (int, string) {
	t.Helper()
	if err == nil {
		return 0, ""
	}
	var se huma.StatusError
	if !errors.As(err, &se) {
		t.Fatalf("expected huma.StatusError, got %T: %v", err, err)
	}
	return se.GetStatus(), se.Error()
}

func TestValidateClaimContract_SkipPaths(t *testing.T) {
	cases := []struct {
		name            string
		kind            string
		claimBaseSha    string
		claimBaseBranch string
		bs              *BranchState
	}{
		{
			name:            "branch_state absent (nil) skips validation",
			kind:            "dev",
			claimBaseSha:    "abc12345",
			claimBaseBranch: "dev",
			bs:              nil,
		},
		{
			name:            "claim_base_sha empty (unclaimed) skips validation",
			kind:            "dev",
			claimBaseSha:    "",
			claimBaseBranch: "",
			// Even with a populated, "violating" branch state, an empty claim base
			// means the todo was never claimed and validation must short-circuit.
			bs: &BranchState{TreeClean: false, HeadBranch: "elsewhere"},
		},
		{
			name:            "metadata kind with nil branch_state skips",
			kind:            "triage",
			claimBaseSha:    "deadbeef",
			claimBaseBranch: "dev",
			bs:              nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateClaimContract(tc.kind, tc.claimBaseSha, tc.claimBaseBranch, tc.bs); err != nil {
				t.Fatalf("expected nil error, got %v", err)
			}
		})
	}
}

func TestValidateClaimContract_MetadataViolations(t *testing.T) {
	const (
		baseSha    = "abc12345def67890abc12345def67890abc12345"
		baseBranch = "dev"
	)

	cases := []struct {
		name        string
		kind        string
		bs          *BranchState
		wantPhrase  string
		wantContext string
	}{
		{
			name: "tree not clean",
			kind: "triage",
			bs: &BranchState{
				HeadSHA:    baseSha,
				HeadBranch: baseBranch,
				TreeClean:  false,
			},
			wantPhrase:  "tree must be clean",
			wantContext: "metadata",
		},
		{
			name: "head_branch mismatch",
			kind: "triage",
			bs: &BranchState{
				HeadSHA:    baseSha,
				HeadBranch: "feature/elsewhere",
				TreeClean:  true,
			},
			wantPhrase:  "branch mismatch",
			wantContext: "metadata",
		},
		{
			name: "head_sha mismatch",
			kind: "triage",
			bs: &BranchState{
				HeadSHA:    "ffffffffffffffffffffffffffffffffffffffff",
				HeadBranch: baseBranch,
				TreeClean:  true,
			},
			wantPhrase:  "SHA mismatch",
			wantContext: "metadata",
		},
		{
			name: "unknown kind defaults to metadata contract",
			kind: "totally-not-a-real-kind",
			bs: &BranchState{
				HeadSHA:    baseSha,
				HeadBranch: baseBranch,
				TreeClean:  false,
			},
			wantPhrase:  "tree must be clean",
			wantContext: "metadata",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClaimContract(tc.kind, baseSha, baseBranch, tc.bs)
			status, msg := statusAndMessage(t, err)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got status=%d msg=%q", status, msg)
			}
			if !strings.Contains(msg, tc.wantPhrase) {
				t.Errorf("expected message to contain %q, got %q", tc.wantPhrase, msg)
			}
			if !strings.Contains(msg, tc.wantContext) {
				t.Errorf("expected message to contain context %q, got %q", tc.wantContext, msg)
			}
		})
	}
}

func TestValidateClaimContract_DevViolations(t *testing.T) {
	const (
		baseSha    = "abc12345def67890abc12345def67890abc12345"
		baseBranch = "dev"
	)

	cases := []struct {
		name       string
		bs         *BranchState
		wantPhrase string
	}{
		{
			name: "no commits since base",
			bs: &BranchState{
				HeadSHA:          baseSha,
				HeadBranch:       baseBranch,
				TreeClean:        true,
				CommitsSinceBase: nil,
			},
			wantPhrase: "at least one commit",
		},
		{
			name: "branch mismatch on dev kind",
			bs: &BranchState{
				HeadSHA:          baseSha,
				HeadBranch:       "feature/wrong",
				TreeClean:        true,
				CommitsSinceBase: []string{"deadbeef"},
			},
			wantPhrase: "branch mismatch",
		},
		{
			name: "tree not clean on dev kind",
			bs: &BranchState{
				HeadSHA:          baseSha,
				HeadBranch:       baseBranch,
				TreeClean:        false,
				CommitsSinceBase: []string{"deadbeef"},
			},
			wantPhrase: "tree must be clean",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateClaimContract("dev", baseSha, baseBranch, tc.bs)
			status, msg := statusAndMessage(t, err)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("expected 422, got status=%d msg=%q", status, msg)
			}
			if !strings.Contains(msg, tc.wantPhrase) {
				t.Errorf("expected message to contain %q, got %q", tc.wantPhrase, msg)
			}
			if !strings.Contains(msg, "dev") {
				t.Errorf("expected dev contract context in message, got %q", msg)
			}
		})
	}
}

func TestValidateClaimContract_DevAncestorViolation(t *testing.T) {
	// Use SHAs that are not present in the test's git repo so
	// `git merge-base --is-ancestor` errors and the validator surfaces
	// the "not an ancestor" 422. claimBaseSha != HeadSHA forces the
	// ancestor check; otherwise the equality short-circuit skips it.
	const (
		claimBaseSha = "0000000000000000000000000000000000000001"
		headSha      = "0000000000000000000000000000000000000002"
		baseBranch   = "dev"
	)

	bs := &BranchState{
		HeadSHA:          headSha,
		HeadBranch:       baseBranch,
		TreeClean:        true,
		CommitsSinceBase: []string{"deadbeef"},
	}

	err := validateClaimContract("dev", claimBaseSha, baseBranch, bs)
	status, msg := statusAndMessage(t, err)
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got status=%d msg=%q", status, msg)
	}
	if !strings.Contains(msg, "not an ancestor") {
		t.Errorf("expected message to contain %q, got %q", "not an ancestor", msg)
	}
}

func TestValidateClaimContract_DevHeadEqualsBaseSkipsAncestorCheck(t *testing.T) {
	// When HeadSHA == claimBaseSha, the ancestor check is skipped.
	// This guards regression of the equality short-circuit at handlers_claim_contract.go:68.
	const (
		baseSha    = "0000000000000000000000000000000000000001"
		baseBranch = "dev"
	)

	bs := &BranchState{
		HeadSHA:          baseSha,
		HeadBranch:       baseBranch,
		TreeClean:        true,
		CommitsSinceBase: []string{"abc12345"},
	}

	if err := validateClaimContract("dev", baseSha, baseBranch, bs); err != nil {
		t.Fatalf("expected nil (ancestor skipped when HeadSHA==claimBaseSha), got %v", err)
	}
}

func TestValidateClaimContract_DevHeadEmptySkipsAncestorCheck(t *testing.T) {
	// When HeadSHA is empty (client didn't report a SHA), the ancestor
	// check is also skipped — best-effort per handlers_claim_contract.go:68.
	const (
		baseSha    = "0000000000000000000000000000000000000001"
		baseBranch = "dev"
	)

	bs := &BranchState{
		HeadSHA:          "",
		HeadBranch:       baseBranch,
		TreeClean:        true,
		CommitsSinceBase: []string{"abc12345"},
	}

	if err := validateClaimContract("dev", baseSha, baseBranch, bs); err != nil {
		t.Fatalf("expected nil (ancestor skipped when HeadSHA empty), got %v", err)
	}
}
