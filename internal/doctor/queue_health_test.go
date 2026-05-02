package doctor

import (
	"strings"
	"testing"
)

// findQueueFinding returns a pointer to the finding for a given check name,
// or nil if absent. (vines_test.go declares findFinding with a different
// signature; this avoids the name clash while keeping the assertion style.)
func findQueueFinding(findings []Finding, name string) *Finding {
	for i := range findings {
		if findings[i].Check.Name == name {
			return &findings[i]
		}
	}
	return nil
}

// TestQueueHasClaimableWork_AllBlockedFails — the IS-955 case: open issues
// exist, todos exist, but every one is blocked. Rung must fail and surface
// the dominant blocked reason.
func TestQueueHasClaimableWork_AllBlockedFails(t *testing.T) {
	state := &ProjectState{
		Classification: ClassTool,
		OpenIssueCount: 30,
		TodoQueueHealth: TodoQueueHealth{
			TotalOpen:             30,
			BlockedCount:          30,
			UnblockedCount:        0,
			DominantBlockedReason: "Reopened 3+ times",
		},
	}

	findings := Evaluate(state)
	f := findQueueFinding(findings, "queue_has_claimable_work")
	if f == nil {
		t.Fatal("queue_has_claimable_work finding missing")
	}
	if f.Status != StatusFail {
		t.Fatalf("expected StatusFail, got %s", f.Status)
	}
	if !strings.Contains(f.Message, "30/30") {
		t.Errorf("expected message to include 30/30, got %q", f.Message)
	}
	if !strings.Contains(f.Message, "Reopened 3+ times") {
		t.Errorf("expected dominant reason in message, got %q", f.Message)
	}
	if f.Proposal == "" {
		t.Error("expected non-empty Proposal")
	}
}

// TestQueueHasClaimableWork_NoIssuesPass — when there are no open issues, the
// queue being empty is expected: rung passes.
func TestQueueHasClaimableWork_NoIssuesPass(t *testing.T) {
	state := &ProjectState{
		Classification: ClassTool,
		OpenIssueCount: 0,
		TodoQueueHealth: TodoQueueHealth{
			TotalOpen:      0,
			BlockedCount:   0,
			UnblockedCount: 0,
		},
	}

	findings := Evaluate(state)
	f := findQueueFinding(findings, "queue_has_claimable_work")
	if f == nil {
		t.Fatal("queue_has_claimable_work finding missing")
	}
	if f.Status != StatusPass {
		t.Fatalf("expected StatusPass, got %s (msg=%q)", f.Status, f.Message)
	}
}

// TestQueueHasClaimableWork_DominantReasonFallback — when TotalOpen>0 but the
// dominant reason is empty, the message should still surface a non-empty
// reason (fallback to "unknown") so the rung remains actionable.
func TestQueueHasClaimableWork_DominantReasonFallback(t *testing.T) {
	state := &ProjectState{
		Classification: ClassTool,
		OpenIssueCount: 5,
		TodoQueueHealth: TodoQueueHealth{
			TotalOpen:             5,
			BlockedCount:          5,
			UnblockedCount:        0,
			DominantBlockedReason: "",
		},
	}

	findings := Evaluate(state)
	f := findQueueFinding(findings, "queue_has_claimable_work")
	if f == nil || f.Status != StatusFail {
		t.Fatalf("expected fail finding, got %+v", f)
	}
	if !strings.Contains(f.Message, "unknown") {
		t.Errorf("expected fallback 'unknown' reason in message, got %q", f.Message)
	}
}

// TestQueueBlockedRatioOk_HighRatioFails — 9/10 blocked = 90% > 80% threshold.
func TestQueueBlockedRatioOk_HighRatioFails(t *testing.T) {
	state := &ProjectState{
		Classification: ClassTool,
		OpenIssueCount: 5,
		TodoQueueHealth: TodoQueueHealth{
			TotalOpen:      10,
			BlockedCount:   9,
			UnblockedCount: 1,
		},
	}

	findings := Evaluate(state)
	f := findQueueFinding(findings, "queue_blocked_ratio_ok")
	if f == nil {
		t.Fatal("queue_blocked_ratio_ok finding missing")
	}
	if f.Status != StatusFail {
		t.Fatalf("expected StatusFail, got %s (msg=%q)", f.Status, f.Message)
	}
	if !strings.Contains(f.Message, "90%") {
		t.Errorf("expected 90%% in message, got %q", f.Message)
	}
	if !strings.Contains(f.Message, "9/10") {
		t.Errorf("expected 9/10 in message, got %q", f.Message)
	}
}

// TestQueueBlockedRatioOk_LowRatioPasses — 7/10 blocked = 70% < 80% threshold.
func TestQueueBlockedRatioOk_LowRatioPasses(t *testing.T) {
	state := &ProjectState{
		Classification: ClassTool,
		OpenIssueCount: 5,
		TodoQueueHealth: TodoQueueHealth{
			TotalOpen:      10,
			BlockedCount:   7,
			UnblockedCount: 3,
		},
	}

	findings := Evaluate(state)
	f := findQueueFinding(findings, "queue_blocked_ratio_ok")
	if f == nil || f.Status != StatusPass {
		t.Fatalf("expected pass finding, got %+v", f)
	}
}

// TestQueueBlockedRatioOk_EmptyQueuePasses — division-by-zero guard: an empty
// queue has no ratio, the check should pass.
func TestQueueBlockedRatioOk_EmptyQueuePasses(t *testing.T) {
	state := &ProjectState{
		Classification: ClassTool,
		TodoQueueHealth: TodoQueueHealth{
			TotalOpen:      0,
			BlockedCount:   0,
			UnblockedCount: 0,
		},
	}

	findings := Evaluate(state)
	f := findQueueFinding(findings, "queue_blocked_ratio_ok")
	if f == nil || f.Status != StatusPass {
		t.Fatalf("expected pass finding, got %+v", f)
	}
}

// TestQueueHealthRungInVine — the rung must appear in the common vine for
// every classification (it's a base rung, not classification-specific).
func TestQueueHealthRungInVine(t *testing.T) {
	for _, c := range AllClassifications {
		t.Run(string(c), func(t *testing.T) {
			vine := Vine(c)
			var found *Rung
			for i := range vine {
				if vine[i].Name == "queue_health" {
					found = &vine[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("queue_health rung missing from %s vine", c)
			}
			if len(found.Checks) != 2 {
				t.Errorf("expected 2 checks on queue_health rung, got %d", len(found.Checks))
			}
		})
	}
}
