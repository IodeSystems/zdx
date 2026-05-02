package project

import (
	"testing"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ptr(s string) *string { return &s }

func mkBranch(name, role, source string) dxclient.VersionBranchItem {
	b := dxclient.VersionBranchItem{Name: name, Type: "named", Status: "active"}
	if role != "" {
		b.Role = ptr(role)
	}
	if source != "" {
		b.SourceBranchName = ptr(source)
	}
	return b
}

func TestClassifyBranchRung(t *testing.T) {
	cases := []struct {
		name     string
		branches []dxclient.VersionBranchItem
		wantRung int
	}{
		{"rung0_empty", nil, 0},
		{"rung1_rolling_only", []dxclient.VersionBranchItem{mkBranch("main", "rolling-release", "")}, 1},
		{"rung1_dev_orphan", []dxclient.VersionBranchItem{mkBranch("dev", "dev", "")}, 1},
		{"rung1_release_and_dev_unlinked", []dxclient.VersionBranchItem{
			mkBranch("main", "rolling-release", ""),
			mkBranch("dev", "dev", ""),
		}, 1},
		{"rung2_main_tracks_dev", []dxclient.VersionBranchItem{
			mkBranch("dev", "dev", ""),
			mkBranch("main", "rolling-release", "dev"),
		}, 2},
		{"rung3_pr_target_present", []dxclient.VersionBranchItem{
			mkBranch("dev", "dev", ""),
			mkBranch("main", "rolling-release", "dev"),
			mkBranch("pr-target", "pr-target", "dev"),
		}, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := classifyBranchRung(tc.branches)
			if got != tc.wantRung {
				t.Errorf("classifyBranchRung = %d, want %d", got, tc.wantRung)
			}
		})
	}
}

func TestClassifyBranchRungReleaseFeederName(t *testing.T) {
	bs := []dxclient.VersionBranchItem{
		mkBranch("dev", "dev", ""),
		mkBranch("main", "rolling-release", "dev"),
	}
	rung, info := classifyBranchRung(bs)
	if rung != 2 {
		t.Fatalf("rung = %d, want 2", rung)
	}
	if info.releaseFeeder == nil || info.releaseFeeder.Name != "main" {
		t.Errorf("releaseFeeder = %+v, want main", info.releaseFeeder)
	}
	if info.dev == nil || info.dev.Name != "dev" {
		t.Errorf("dev = %+v, want dev", info.dev)
	}
}
