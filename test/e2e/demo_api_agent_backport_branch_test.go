package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_SoloBackportTargetBranch is the demo for spec 177 on feature
// dx-todo-queue: given backport tasks for supported version branches, when the
// agent queue evaluates, then backport tasks surface with the target branch name
// and version context so agents know which branch to work on.
//
// Scenario: triage an issue against the v1.2 release branch, add a ready task,
// then evaluate scoped to that issue and confirm the dev item carries
// target_branch="v1.2". A second project covers the negative path: an issue
// triaged without an explicit branch defaults to "dev".
func TestDemoAPI_SoloBackportTargetBranch(t *testing.T) {
	rec := newApiRecorder(t, "agent-backport-target-branch")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_solo_backport_branch_test.go", Note: "spec 177 demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_solo.go", LineStart: 591, LineEnd: 612, Note: "dev candidate inherits TargetBranch from parent issue"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_issues.go", Note: "POST /api/dx/todo/owner/triage accepts target_branch"})
	t.Cleanup(rec.Save)

	// --- case 1: backport scenario — issue triaged against the v1.2 branch ---
	const slugBackport = "demo-agent-backport-v12"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slugBackport,
		"name": "Demo Agent Backport v1.2",
	}, nil))

	var backportIssue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       slugBackport,
		"title":      "Backport login fix to v1.2",
		"context":    "cherry-pick the login fix onto the v1.2 release branch",
		"auto_ready": true,
	}, &backportIssue))

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/owner/triage", map[string]any{
		"slug":          slugBackport,
		"id":            backportIssue.ID,
		"priority":      2,
		"target_branch": "v1.2",
	}, nil))

	backportRef := fmt.Sprintf("IS-%d", backportIssue.ID)
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
		"slug":       slugBackport,
		"text":       "Cherry-pick the login fix onto v1.2",
		"issue":      backportRef,
		"auto_ready": true,
		"force":      true,
	}, nil))

	// Each mutation fires refreshQueueAsync; let the goroutines finish so the
	// /evaluate diff buckets aren't racing the background persist (matches the
	// pattern in TestDemoAPI_AgentEvaluateDiff).
	time.Sleep(50 * time.Millisecond)

	dev := evaluateDevItem(t, rec, slugBackport, backportRef)
	if dev.TargetBranch != "v1.2" {
		t.Errorf("backport dev item target_branch: want %q got %q", "v1.2", dev.TargetBranch)
	}
	if dev.IssueRef != backportRef {
		t.Errorf("backport dev item issue_ref: want %q got %q", backportRef, dev.IssueRef)
	}

	// --- case 2: default — issue triaged without an explicit branch ---
	const slugDefault = "demo-agent-backport-dev"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slugDefault,
		"name": "Demo Agent Backport Default",
	}, nil))

	var defaultIssue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       slugDefault,
		"title":      "Normal fix on dev",
		"context":    "no explicit branch — should default to dev",
		"auto_ready": true,
	}, &defaultIssue))

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/owner/triage", map[string]any{
		"slug":     slugDefault,
		"id":       defaultIssue.ID,
		"priority": 2,
	}, nil))

	defaultRef := fmt.Sprintf("IS-%d", defaultIssue.ID)
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
		"slug":       slugDefault,
		"text":       "Implement the fix",
		"issue":      defaultRef,
		"auto_ready": true,
		"force":      true,
	}, nil))

	time.Sleep(50 * time.Millisecond)

	defDev := evaluateDevItem(t, rec, slugDefault, defaultRef)
	if defDev.TargetBranch != "dev" {
		t.Errorf("default dev item target_branch: want %q got %q", "dev", defDev.TargetBranch)
	}
}

// evaluateDevItem POSTs /api/dx/agent/evaluate scoped to issueRef and returns
// the dev item from whichever diff bucket carries it (Added/Unchanged/Changed).
func evaluateDevItem(t *testing.T, rec *ApiDemoRecorder, slug, issueRef string) AgentQueueItem {
	t.Helper()
	var diff EvaluateDiffResult
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/evaluate", map[string]any{
		"slug":  slug,
		"issue": issueRef,
	}, &diff))

	candidates := append([]AgentQueueItem{}, diff.Added...)
	candidates = append(candidates, diff.Unchanged...)
	for _, c := range diff.Changed {
		candidates = append(candidates, c.After)
	}
	if dev := findKind(candidates, "dev"); dev != nil {
		return *dev
	}
	t.Fatalf("evaluate %s: missing dev item; diff=%+v", issueRef, diff)
	return AgentQueueItem{}
}
