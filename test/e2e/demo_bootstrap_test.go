package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// TestDemoInitBootstrap is the demo for spec 136:
// Given a new project, when BootstrapOnboardingIssue runs, it creates exactly
// one onboarding issue with four blocker questions covering goals, architecture,
// constraints, and workflow. A second invocation is a no-op (skips silently).
func TestDemoInitBootstrap(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_bootstrap_test.go", Note: "bootstrap demo source (spec 136)"},
		{FilePath: "internal/cli/bootstrap.go", Note: "BootstrapOnboardingIssue: creates onboarding issue + 4 BQs"},
	})

	const slug = "e2e-bootstrap-136"
	NewApiDriver(t, slug, "E2E Bootstrap Spec 136")

	c := cli.NewClientWithSlug(srv.URL, srv.AdminToken, slug)

	// First invocation: should create 1 issue + 4 BQs.
	if err := cli.BootstrapOnboardingIssue(c); err != nil {
		t.Fatalf("first bootstrap: %v", err)
	}

	// Verify exactly one issue with "onboarding" in the title.
	var issueListResp struct {
		Issues []struct {
			ID    int32  `json:"id"`
			Title string `json:"title"`
		} `json:"issues"`
		Total int64 `json:"total"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/list?slug=%s", slug), nil, &issueListResp))
	if issueListResp.Total != 1 {
		t.Fatalf("want 1 issue after bootstrap, got %d", issueListResp.Total)
	}
	if !strings.Contains(strings.ToLower(issueListResp.Issues[0].Title), "onboarding") {
		t.Errorf("issue title should contain 'onboarding', got %q", issueListResp.Issues[0].Title)
	}
	issueID := fmt.Sprintf("IS-%d", issueListResp.Issues[0].ID)

	// Verify exactly four pending blocker questions on that issue.
	var bqResp struct {
		Questions []struct {
			TargetType string `json:"target_type"`
			TargetID   string `json:"target_id"`
			Context    string `json:"context"`
		} `json:"questions"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/blocker-questions/list?slug=%s&status=pending", slug), nil, &bqResp))

	var issueBQs []string
	for _, q := range bqResp.Questions {
		if q.TargetType == "issue" && q.TargetID == issueID {
			issueBQs = append(issueBQs, q.Context)
		}
	}
	if len(issueBQs) != 4 {
		t.Fatalf("want 4 blocker questions on %s, got %d", issueID, len(issueBQs))
	}

	// Verify topic coverage: goals, architecture, constraints, workflow.
	for keyword, label := range map[string]string{
		"goal":         "goals",
		"architecture": "architecture",
		"constraint":   "constraints",
		"workflow":     "workflow",
	} {
		found := false
		for _, ctx := range issueBQs {
			if strings.Contains(strings.ToLower(ctx), keyword) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no blocker question covers topic %q (keyword %q)", label, keyword)
		}
	}

	// Second invocation: no-op — issue count stays at 1.
	if err := cli.BootstrapOnboardingIssue(c); err != nil {
		t.Fatalf("second bootstrap: %v", err)
	}

	var issueListResp2 struct {
		Total int64 `json:"total"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/list?slug=%s", slug), nil, &issueListResp2))
	if issueListResp2.Total != 1 {
		t.Fatalf("second bootstrap: want still 1 issue (no-op), got %d", issueListResp2.Total)
	}
}
