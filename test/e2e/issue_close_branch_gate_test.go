package e2e

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestIssueCloseBranchGate covers TK-1504 (IS-824 #2 and #4): close-issue is
// blocked when the project has active named version branches that lack a
// resolution from this issue, and succeeds once all are resolved.
func TestIssueCloseBranchGate(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to seed named branches")
	}

	slug := "e2e-branch-gate"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Branch Gate"}, nil))

	addIssue := func(issueType string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      "gate test " + issueType,
				"context":    "context",
				"issue_type": issueType,
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}

	addNamedBranch := func(projectSlug, branchName string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, srv.DSN)
		if err != nil {
			t.Fatalf("pgx connect: %v", err)
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx,
			`INSERT INTO zdx_version_branches (project_id, name, type, status)
			 SELECT id, $2, 'named', 'active' FROM zdx_projects WHERE slug = $1`,
			projectSlug, branchName); err != nil {
			t.Fatalf("insert named branch %q: %v", branchName, err)
		}
	}

	addResolution := func(id int32, branch string) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
			map[string]any{
				"slug":   slug,
				"id":     id,
				"shas":   []string{"deadbeef"},
				"branch": branch,
				"source": "manual",
			}, nil))
	}

	closeIssue := func(id int32, body map[string]any) *http.Response {
		t.Helper()
		body["slug"] = slug
		body["id"] = id
		return apiDo(t, http.MethodPost, "/api/dx/todo/issue/close", body, nil)
	}

	readBody := func(resp *http.Response) string {
		t.Helper()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}

	// 1. Named branch blocks close until resolved.
	t.Run("named_branch_blocks_close", func(t *testing.T) {
		id := addIssue("tracker")
		addNamedBranch(slug, "v1.0.x")

		resp := closeIssue(id, map[string]any{})
		if resp.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf("want 422, got %d: %s", resp.StatusCode, readBody(resp))
		}
		body := readBody(resp)
		if !strings.Contains(body, "unresolved_branches:") {
			t.Errorf("error body missing 'unresolved_branches:': %s", body)
		}
		if !strings.Contains(body, "v1.0.x") {
			t.Errorf("error body missing branch name 'v1.0.x': %s", body)
		}
	})

	// 2. Close succeeds after resolving the named branch.
	t.Run("resolves_after_resolution", func(t *testing.T) {
		id := addIssue("tracker")
		// v1.0.x already seeded by prior sub-test; reuse it.
		addResolution(id, "v1.0.x")

		resp := closeIssue(id, map[string]any{})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})

	// 3. Regression: project with only a dev branch (no named branches) still closes.
	t.Run("no_named_branches_closes_normally", func(t *testing.T) {
		devSlug := "e2e-branch-gate-dev"
		mustOK(t, apiDo(t, http.MethodPost, "/api/project",
			map[string]string{"slug": devSlug, "name": "E2E Branch Gate Dev Only"}, nil))

		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       devSlug,
				"title":      "plain close",
				"context":    "context",
				"issue_type": "tracker",
				"auto_ready": true,
			}, &resp))

		httpResp := apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
			map[string]any{"slug": devSlug, "id": resp.ID}, nil)
		if httpResp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", httpResp.StatusCode, readBody(httpResp))
		}
	})

	// 4. Force bypass: --force skips the branch gate.
	t.Run("force_bypasses_gate", func(t *testing.T) {
		id := addIssue("tracker")
		// v1.0.x is still active; issue has no resolution on it.
		force := true
		reason := "wontfix"
		resp := closeIssue(id, map[string]any{"force": force, "reason": reason})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", resp.StatusCode, readBody(resp))
		}
	})
}
