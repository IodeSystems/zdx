package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoAPI_EnvDrift is the demo for spec 156:
// Given a version branch with an environment, when the dashboard renders,
// then it shows commit drift from dev (count and age of commits behind).
//
// Covers:
//  1. Environment with release_branch but no git repo → drift_count=0 (zero-drift case).
//  2. drift_count and drift_oldest_age_secs fields present in API response.
//  3. Environment without release_branch → drift_count always 0.
func TestDemoAPI_EnvDrift(t *testing.T) {
	rec := newApiRecorder(t, "env-drift")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_env_drift_test.go", Note: "spec 156 demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_environments.go", Note: "list-environments computes drift per release_branch"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/git.go", Note: "CommitDrift — rev-list count + oldest commit timestamp"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/git_test.go", Note: "TestCommitDrift — unit test for non-zero drift with temp repo"})
	t.Cleanup(rec.Save)

	const slug = "demo-env-drift"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Env Drift",
	}, nil))

	// Environment with release_branch — no git repo exists, so drift is 0.
	mustOK(t, rec.Do(http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]any{
			"name":           "staging",
			"release_branch": "release/staging",
		}, nil))

	// Environment without release_branch — drift always 0.
	mustOK(t, rec.Do(http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]any{"name": "prod"}, nil))

	var listResp struct {
		Items []struct {
			Name               string `json:"name"`
			ReleaseBranch      string `json:"release_branch"`
			DriftCount         int    `json:"drift_count"`
			DriftOldestAgeSecs int64  `json:"drift_oldest_age_secs"`
		} `json:"items"`
	}
	mustOK(t, rec.Do(http.MethodGet,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		nil, &listResp))

	if len(listResp.Items) != 2 {
		t.Fatalf("expected 2 environments, got %d", len(listResp.Items))
	}

	for _, env := range listResp.Items {
		// Without a real git repo, drift is always 0.
		if env.DriftCount != 0 {
			t.Errorf("env %s: drift_count: want 0 (no repo), got %d", env.Name, env.DriftCount)
		}
		if env.DriftOldestAgeSecs != 0 {
			t.Errorf("env %s: drift_oldest_age_secs: want 0, got %d", env.Name, env.DriftOldestAgeSecs)
		}
	}

	// Verify staging has its release_branch set (sanity).
	var staging *struct {
		Name          string `json:"name"`
		ReleaseBranch string `json:"release_branch"`
		DriftCount    int    `json:"drift_count"`
	}
	for i := range listResp.Items {
		if listResp.Items[i].Name == "staging" {
			env := &struct {
				Name          string `json:"name"`
				ReleaseBranch string `json:"release_branch"`
				DriftCount    int    `json:"drift_count"`
			}{
				Name:          listResp.Items[i].Name,
				ReleaseBranch: listResp.Items[i].ReleaseBranch,
				DriftCount:    listResp.Items[i].DriftCount,
			}
			staging = env
			break
		}
	}
	if staging == nil {
		t.Fatal("staging environment not found in list")
	}
	if staging.ReleaseBranch != "release/staging" {
		t.Errorf("release_branch: want release/staging, got %q", staging.ReleaseBranch)
	}
}
