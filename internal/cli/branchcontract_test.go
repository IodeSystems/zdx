package cli

import (
	"testing"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ptr[T any](v T) *T { return &v }

func TestValidateBranchState_SkipsWhenNoBaseSHA(t *testing.T) {
	bs := &dxclient.BranchState{HeadSha: "abc", HeadBranch: "main", TreeClean: true}
	if err := ValidateBranchState("dev", bs, "", "main"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateBranchState_SkipsWhenBranchStateNil(t *testing.T) {
	if err := ValidateBranchState("dev", nil, "abc123", "main"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateBranchState_DevViolation_DirtyTree(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:          "abc123",
		HeadBranch:       "dev",
		TreeClean:        false,
		CommitsSinceBase: ptr([]string{"abc123"}),
	}
	err := ValidateBranchState("dev", bs, "base456", "dev")
	if err == nil {
		t.Fatal("expected violation error, got nil")
	}
}

func TestValidateBranchState_DevViolation_NoCommits(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:    "abc123",
		HeadBranch: "dev",
		TreeClean:  true,
	}
	err := ValidateBranchState("dev", bs, "base456", "dev")
	if err == nil {
		t.Fatal("expected violation error, got nil")
	}
}

func TestValidateBranchState_DevViolation_BranchMismatch(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:          "abc123",
		HeadBranch:       "feature",
		TreeClean:        true,
		CommitsSinceBase: ptr([]string{"abc123"}),
	}
	err := ValidateBranchState("dev", bs, "base456", "dev")
	if err == nil {
		t.Fatal("expected violation error, got nil")
	}
}

func TestValidateBranchState_MetadataViolation_DirtyTree(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:    "abc123",
		HeadBranch: "dev",
		TreeClean:  false,
	}
	err := ValidateBranchState("product:triage", bs, "abc123", "dev")
	if err == nil {
		t.Fatal("expected violation error, got nil")
	}
}

func TestValidateBranchState_MetadataViolation_SHAMismatch(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:    "changed456",
		HeadBranch: "dev",
		TreeClean:  true,
	}
	err := ValidateBranchState("product:triage", bs, "original123", "dev")
	if err == nil {
		t.Fatal("expected violation error, got nil")
	}
}

func TestValidateBranchState_MetadataClean(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:    "abc123",
		HeadBranch: "dev",
		TreeClean:  true,
	}
	if err := ValidateBranchState("product:triage", bs, "abc123", "dev"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestValidateBranchState_UnknownKindUsesMetadata(t *testing.T) {
	bs := &dxclient.BranchState{
		HeadSha:    "abc123",
		HeadBranch: "dev",
		TreeClean:  true,
	}
	// unknown kind → metadata contract; clean tree + matching SHA/branch = ok
	if err := ValidateBranchState("some-unknown-kind", bs, "abc123", "dev"); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
