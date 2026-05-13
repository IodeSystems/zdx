package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTruncateSeedBody(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"empty stays empty", "", 10, ""},
		{"under limit unchanged", "hello", 10, "hello"},
		{"equal to limit unchanged", "12345", 5, "12345"},
		{"over limit gets ellipsis", "abcdefgh", 4, "abcd…"},
		{"zero max returns input", "abc", 0, "abc"},
		{"negative max returns input", "abc", -1, "abc"},
		// Multi-byte: 3 emoji = 3 runes; clamp at 2 should keep first 2 runes intact.
		{"multi-byte clamped on rune boundary", "🙂🙃😀", 2, "🙂🙃…"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncateSeedBody(tc.in, tc.max); got != tc.want {
				t.Errorf("truncateSeedBody(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

func TestEvaluateDiffEmptySlices(t *testing.T) {
	diff := EvaluateDiff{
		Added:     []AgentQueueItem{},
		Removed:   []TodoItem{},
		Changed:   []EvaluateChange{},
		Unchanged: []AgentQueueItem{},
	}
	b, err := json.Marshal(diff)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, field := range []string{`"added":[]`, `"removed":[]`, `"changed":[]`, `"unchanged":[]`} {
		if !strings.Contains(s, field) {
			t.Errorf("expected %q in JSON, got: %s", field, s)
		}
	}
}

func TestLaneOffsetFor(t *testing.T) {
	cases := []struct {
		name             string
		kind             string
		issuePrioritized bool
		want             int32
	}{
		{"triage always triage lane", "product:triage", false, laneOffsetTriage},
		{"triage with prioritized issue still triage", "product:triage", true, laneOffsetTriage},
		{"dev prioritized → priority lane", "dev", true, laneOffsetPriority},
		{"dev unprioritized → other lane", "dev", false, laneOffsetOther},
		{"add prioritized → priority lane", "add", true, laneOffsetPriority},
		{"add unprioritized → other lane", "add", false, laneOffsetOther},
		{"closable prioritized → priority lane", "closable", true, laneOffsetPriority},
		{"close:tracker prioritized → priority lane", "close:tracker", true, laneOffsetPriority},
		{"tech:decompose-tracker prioritized → priority lane", "tech:decompose-tracker", true, laneOffsetPriority},
		{"tech:decompose-tracker unprioritized → other", "tech:decompose-tracker", false, laneOffsetOther},
		{"read:comments → other regardless", "read:comments", true, laneOffsetOther},
		{"owner:spec → other", "owner:spec", false, laneOffsetOther},
		{"unknown kind → other", "totally-unknown", true, laneOffsetOther},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := laneOffsetFor(tc.kind, tc.issuePrioritized)
			if got != tc.want {
				t.Errorf("laneOffsetFor(%q, %v) = %d, want %d", tc.kind, tc.issuePrioritized, got, tc.want)
			}
		})
	}
}

// TestSortQueueCandidates_EmptyPriorityLane covers the IS-1104 matrix row
// "every open issue unprioritized" — priority lane is empty, triage drains
// next, and unprioritized dev/add work falls through to the "other" lane.
func TestSortQueueCandidates_EmptyPriorityLane(t *testing.T) {
	candidates := []agentCandidate{
		{Key: "add-IS-2", Kind: "add", IssueRef: "IS-2", Priority: 38},
		{Key: "triage-IS-1", Kind: "product:triage", IssueRef: "IS-1", Priority: 20},
		{Key: "triage-IS-2", Kind: "product:triage", IssueRef: "IS-2", Priority: 20},
		{Key: "dev-TK-9", Kind: "dev", IssueRef: "IS-1", Priority: 40},
	}
	sortQueueCandidates(candidates, map[string]bool{})

	wantOrder := []string{"triage-IS-1", "triage-IS-2", "add-IS-2", "dev-TK-9"}
	for i, want := range wantOrder {
		if candidates[i].Key != want {
			t.Errorf("position %d: got %s, want %s (full order: %v)", i, candidates[i].Key, want, keysOf(candidates))
		}
	}
	if candidates[0].Priority != 20+laneOffsetTriage {
		t.Errorf("expected triage priority %d, got %d", 20+laneOffsetTriage, candidates[0].Priority)
	}
}

// TestSortQueueCandidates_PriorityBeatsTriage covers the mixed matrix row:
// some issues are P1 with ready dev work, others are unprioritized with
// triage candidates. Without lanes the P1-folded dev (priority 20) tied with
// triage (also 20). With lanes, dev claims first.
func TestSortQueueCandidates_PriorityBeatsTriage(t *testing.T) {
	candidates := []agentCandidate{
		{Key: "triage-IS-1", Kind: "product:triage", IssueRef: "IS-1", Priority: 20},
		// foldIssuePriority(40, "1") = 20 — pre-TK-1757 this tied with triage.
		{Key: "dev-TK-2", Kind: "dev", IssueRef: "IS-2", Priority: 20},
		// foldIssuePriority(38, "2") = 23
		{Key: "add-IS-3", Kind: "add", IssueRef: "IS-3", Priority: 23},
		{Key: "comment-IS-4", Kind: "read:comments", IssueRef: "IS-4", Priority: 5},
	}
	prioritized := map[string]bool{"IS-2": true, "IS-3": true}
	sortQueueCandidates(candidates, prioritized)

	wantOrder := []string{"dev-TK-2", "add-IS-3", "triage-IS-1", "comment-IS-4"}
	for i, want := range wantOrder {
		if candidates[i].Key != want {
			t.Errorf("position %d: got %s, want %s (full order: %v)", i, candidates[i].Key, want, keysOf(candidates))
		}
	}
}

// TestSortQueueCandidates_Promotion covers the promotion matrix row: before
// the operator sets a priority on IS-1, its triage candidate sits in the
// triage lane and its add candidate is "other"; after the priority is set,
// the regenerated queue drops the triage candidate (issue no longer matches
// `iss.Priority == ""` in generateAgentQueue) and add:IS-1 moves into the
// priority lane.
func TestSortQueueCandidates_Promotion(t *testing.T) {
	before := []agentCandidate{
		{Key: "triage-IS-1", Kind: "product:triage", IssueRef: "IS-1", Priority: 20},
		{Key: "add-IS-1", Kind: "add", IssueRef: "IS-1", Priority: 38},
	}
	sortQueueCandidates(before, map[string]bool{})
	if before[0].Key != "triage-IS-1" {
		t.Fatalf("pre-promotion: expected triage-IS-1 first, got %s (full order: %v)", before[0].Key, keysOf(before))
	}
	if before[1].Key != "add-IS-1" {
		t.Fatalf("pre-promotion: expected add-IS-1 second, got %s", before[1].Key)
	}

	// After promotion: triage candidate no longer emitted; add:IS-1 with P1
	// folded priority enters priority lane.
	after := []agentCandidate{
		{Key: "add-IS-1", Kind: "add", IssueRef: "IS-1", Priority: foldIssuePriority(38, "1")},
	}
	sortQueueCandidates(after, map[string]bool{"IS-1": true})
	if after[0].Key != "add-IS-1" {
		t.Fatalf("post-promotion: expected add-IS-1 first, got %s", after[0].Key)
	}
	if want := foldIssuePriority(38, "1") + laneOffsetPriority; after[0].Priority != want {
		t.Fatalf("post-promotion: expected priority %d (priority lane), got %d", want, after[0].Priority)
	}
}

// TestSortQueueCandidates_BranchTiebreaker confirms the existing dev-first /
// alphabetical branch tiebreaker still applies within a lane when priorities
// match (spec 178). Without this, the lane refactor would silently change
// claim order for multi-branch projects.
func TestSortQueueCandidates_BranchTiebreaker(t *testing.T) {
	candidates := []agentCandidate{
		{Key: "dev-TK-aaa", Kind: "dev", IssueRef: "IS-1", Priority: 20, TargetBranch: "release/3.0"},
		{Key: "dev-TK-bbb", Kind: "dev", IssueRef: "IS-1", Priority: 20, TargetBranch: "dev"},
		{Key: "dev-TK-ccc", Kind: "dev", IssueRef: "IS-1", Priority: 20, TargetBranch: "feature/x"},
	}
	sortQueueCandidates(candidates, map[string]bool{"IS-1": true})
	wantOrder := []string{"dev-TK-bbb", "dev-TK-ccc", "dev-TK-aaa"}
	for i, want := range wantOrder {
		if candidates[i].Key != want {
			t.Errorf("position %d: got %s, want %s (full order: %v)", i, candidates[i].Key, want, keysOf(candidates))
		}
	}
}

func keysOf(c []agentCandidate) []string {
	out := make([]string, len(c))
	for i, x := range c {
		out[i] = x.Key
	}
	return out
}

func TestFoldIssuePriority(t *testing.T) {
	cases := []struct {
		name     string
		base     int32
		priority string
		want     int32
	}{
		{"dev P1", 40, "1", 20},
		{"dev P2", 40, "2", 25},
		{"dev P3", 40, "3", 30},
		{"dev P4", 40, "4", 35},
		{"dev untriaged", 40, "", 35},
		{"closable P1", 35, "1", 15},
		{"add P2", 38, "2", 23},
		{"close:tracker P3", 36, "3", 26},
		{"unknown priority treated as P4", 40, "garbage", 35},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := foldIssuePriority(tc.base, tc.priority)
			if got != tc.want {
				t.Errorf("foldIssuePriority(%d, %q) = %d, want %d", tc.base, tc.priority, got, tc.want)
			}
		})
	}
}
