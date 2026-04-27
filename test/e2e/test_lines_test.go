package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestTestLinesBranchTracking verifies that test results submitted with branch+sha
// are persisted and returned in the test list with correct last_failed_at tracking.
func TestTestLinesBranchTracking(t *testing.T) {
	const slug = "e2e-test-lines"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Test Lines"},
		nil,
	)

	submit := func(name, status, branch, sha string) {
		t.Helper()
		body := map[string]any{
			"slug": slug,
			"results": []map[string]any{
				{
					"test_name":   name,
					"status":      status,
					"driver":      "go",
					"duration_ms": 100,
					"branch":      branch,
					"git_sha":     sha,
					"feature":     "",
				},
			},
		}
		resp := apiDo(t, http.MethodPost, "/api/dx/test-results/submit", body, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("submit %s/%s: want 200, got %d", name, status, resp.StatusCode)
		}
	}

	submit("pkg/auth/TestLogin", "pass", "main", "abc123")
	submit("pkg/auth/TestLogout", "fail", "feature/logout-fix", "def456")

	var list struct {
		Tests []struct {
			Name             string  `json:"name"`
			Status           string  `json:"status"`
			LastRunBranch    string  `json:"last_run_branch"`
			LastRunSha       string  `json:"last_run_sha"`
			LastFailedAt     *string `json:"last_failed_at"`
			LastFailedBranch string  `json:"last_failed_branch"`
		} `json:"tests"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/dx/tests?slug=%s", slug), nil, &list))

	byName := map[string]struct {
		Status           string
		LastRunBranch    string
		LastFailedAt     *string
		LastFailedBranch string
	}{}
	for _, tst := range list.Tests {
		byName[tst.Name] = struct {
			Status           string
			LastRunBranch    string
			LastFailedAt     *string
			LastFailedBranch string
		}{tst.Status, tst.LastRunBranch, tst.LastFailedAt, tst.LastFailedBranch}
	}

	login := byName["pkg/auth/TestLogin"]
	if login.Status != "pass" {
		t.Errorf("TestLogin: want status=pass, got %q", login.Status)
	}
	if login.LastRunBranch != "main" {
		t.Errorf("TestLogin: want last_run_branch=main, got %q", login.LastRunBranch)
	}
	if login.LastFailedAt != nil {
		t.Errorf("TestLogin: want last_failed_at=nil for passing test, got %v", login.LastFailedAt)
	}

	logout := byName["pkg/auth/TestLogout"]
	if logout.Status != "fail" {
		t.Errorf("TestLogout: want status=fail, got %q", logout.Status)
	}
	if logout.LastRunBranch != "feature/logout-fix" {
		t.Errorf("TestLogout: want last_run_branch=feature/logout-fix, got %q", logout.LastRunBranch)
	}
	if logout.LastFailedAt == nil {
		t.Errorf("TestLogout: want last_failed_at set for failing test, got nil")
	}
	if logout.LastFailedBranch != "feature/logout-fix" {
		t.Errorf("TestLogout: want last_failed_branch=feature/logout-fix, got %q", logout.LastFailedBranch)
	}

	// Re-submit TestLogout as passing on main — last_failed_at should persist.
	submit("pkg/auth/TestLogout", "pass", "main", "ghi789")
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/dx/tests?slug=%s", slug), nil, &list))
	for _, tst := range list.Tests {
		if tst.Name == "pkg/auth/TestLogout" {
			if tst.LastFailedAt == nil {
				t.Error("TestLogout: last_failed_at should persist after passing re-run")
			}
			if tst.LastRunBranch != "main" {
				t.Errorf("TestLogout: last_run_branch should update to main after passing run, got %q", tst.LastRunBranch)
			}
		}
	}
}

// TestEnvironmentCRUD verifies environment create/deploy/list/delete lifecycle.
func TestEnvironmentCRUD(t *testing.T) {
	const slug = "e2e-env-crud"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Env CRUD"},
		nil,
	)

	// Create environment
	var env struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]string{"name": "production", "url": "https://example.com"},
		&env,
	))
	if env.Name != "production" {
		t.Errorf("want name=production, got %q", env.Name)
	}

	// Record a deploy
	var deploy struct {
		Status      string `json:"status"`
		BuildBranch string `json:"build_branch"`
		BuildSha    string `json:"build_sha"`
	}
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments/production/deploys", slug),
		map[string]any{"build_sha": "deadbeef1234", "build_branch": "main", "status": "success"},
		&deploy,
	))
	if deploy.Status != "success" {
		t.Errorf("want status=success, got %q", deploy.Status)
	}
	if deploy.BuildBranch != "main" {
		t.Errorf("want branch=main, got %q", deploy.BuildBranch)
	}

	// Get environment — current_build_sha should reflect the deploy
	var got struct {
		Name               string `json:"name"`
		CurrentBuildSha    string `json:"current_build_sha"`
		CurrentBuildBranch string `json:"current_build_branch"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/projects/%s/environments/production", slug),
		nil, &got,
	))
	if got.CurrentBuildSha != "deadbeef1234" {
		t.Errorf("want current_build_sha=deadbeef1234, got %q", got.CurrentBuildSha)
	}
	if got.CurrentBuildBranch != "main" {
		t.Errorf("want current_build_branch=main, got %q", got.CurrentBuildBranch)
	}

	// List deploy history
	var deploys struct {
		Items []struct {
			BuildSha string `json:"build_sha"`
			Status   string `json:"status"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/projects/%s/environments/production/deploys", slug),
		nil, &deploys,
	))
	if deploys.Total < 1 {
		t.Error("want at least 1 deploy in history")
	}
	if len(deploys.Items) == 0 || deploys.Items[0].BuildSha != "deadbeef1234" {
		t.Errorf("want first deploy sha=deadbeef1234")
	}

	// Delete environment
	mustOK(t, apiDo(t, http.MethodDelete,
		fmt.Sprintf("/api/dx/projects/%s/environments/production", slug),
		nil, nil,
	))
	resp := apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/projects/%s/environments/production", slug),
		nil, nil,
	)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("want 404 after delete, got %d", resp.StatusCode)
	}
}
