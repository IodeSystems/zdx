package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoAPI_IssueDefaultTargetBranch is the demo for spec 147 on feature
// version-branches: all new issues target the dev branch by default unless an
// explicit branch is specified at creation. Verifies that both the create
// response and the subsequent show response carry target_branch = "dev" when no
// branch is given.
func TestDemoAPI_IssueDefaultTargetBranch(t *testing.T) {
	rec := newApiRecorder(t, "issue-default-target-branch")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_issue_default_branch_test.go", Note: "issue-default-target-branch demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_issues.go", Note: "POST /api/dx/todo/issue/add — target_branch defaults to dev via schema"})
	t.Cleanup(rec.Save)

	const slug = "demo-issue-branch-default"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Issue Branch Default",
	}, nil))

	var created struct {
		ID           int32   `json:"id"`
		TargetBranch *string `json:"target_branch"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":  slug,
		"title": "Fix login timeout",
	}, &created))

	if created.TargetBranch == nil || *created.TargetBranch != "dev" {
		got := "<nil>"
		if created.TargetBranch != nil {
			got = *created.TargetBranch
		}
		t.Errorf("create response target_branch: want %q got %q", "dev", got)
	}

	var show struct {
		Issue struct {
			ID           int32   `json:"id"`
			TargetBranch *string `json:"target_branch"`
		} `json:"issue"`
	}
	mustOK(t, rec.Do(http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/show?slug=%s&id=IS-%d", slug, created.ID),
		nil, &show,
	))

	if show.Issue.TargetBranch == nil || *show.Issue.TargetBranch != "dev" {
		got := "<nil>"
		if show.Issue.TargetBranch != nil {
			got = *show.Issue.TargetBranch
		}
		t.Errorf("show response target_branch: want %q got %q", "dev", got)
	}
}
