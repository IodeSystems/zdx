package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func apiDo(t *testing.T, method, path string, body any, out any) *http.Response {
	t.Helper()
	var b []byte
	if body != nil {
		var err error
		b, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
	}
	req, _ := http.NewRequest(method, srv.URL+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", srv.AdminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if out != nil && resp.StatusCode < 400 {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}
	return resp
}

func mustOK(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, string(body))
	}
}

// ── Tests ──────────────────────────────────────────────────────────────────

// TestHealth verifies spec 59: GET /api/health returns the build SHA and
// status so deployments can be probed. Also asserts the IS-809 subsystems
// shape: subsystems.queue is present and non-fatal (top-level stays ok).
func TestHealth(t *testing.T) {
	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
	var body struct {
		Status     string `json:"status"`
		BuildSHA   string `json:"build_sha"`
		Subsystems map[string]struct {
			State string `json:"state"`
			Depth *int64 `json:"depth,omitempty"`
		} `json:"subsystems"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status: want %q got %q", "ok", body.Status)
	}
	if body.BuildSHA == "" {
		t.Error("build_sha: empty (ldflags wiring lost)")
	}
	q, ok := body.Subsystems["queue"]
	if !ok {
		t.Fatalf("subsystems.queue: missing (got %+v)", body.Subsystems)
	}
	if q.State != "ok" {
		t.Errorf("subsystems.queue.state: want %q got %q", "ok", q.State)
	}
}

func TestProjectCRUD(t *testing.T) {
	var proj struct {
		Slug string `json:"slug"`
		Name string `json:"name"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-smoke", "name": "E2E Smoke"},
		&proj,
	))
	if proj.Slug != "e2e-smoke" {
		t.Errorf("slug: want %q got %q", "e2e-smoke", proj.Slug)
	}

	var list struct {
		Projects []struct {
			Slug string `json:"slug"`
		} `json:"projects"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/projects", nil, &list))
	found := false
	for _, p := range list.Projects {
		if p.Slug == "e2e-smoke" {
			found = true
		}
	}
	if !found {
		t.Error("created project not in list")
	}
}

func TestIssueCRUD(t *testing.T) {
	// Ensure project exists (may already exist from TestProjectCRUD in same run).
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-issues", "name": "E2E Issues"},
		nil,
	)

	var issue struct {
		ID    int    `json:"id"`
		Title string `json:"title"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": "e2e-issues", "title": "smoke issue", "context": "created by e2e test", "auto_ready": true},
		&issue,
	))
	if issue.Title != "smoke issue" {
		t.Errorf("title: want %q got %q", "smoke issue", issue.Title)
	}

	var show struct {
		Issue struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"issue"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/show?slug=e2e-issues&id=IS-%d", issue.ID),
		nil, &show,
	))
	if show.Issue.ID != issue.ID {
		t.Errorf("show id: want %d got %d", issue.ID, show.Issue.ID)
	}
}

// TestIssueDefaultTargetBranch verifies spec 139: issue created without an
// explicit branch target defaults to "dev" in both the create response and the
// subsequent show response.
func TestIssueDefaultTargetBranch(t *testing.T) {
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-branch-default", "name": "E2E Branch Default"},
		nil,
	)

	var created struct {
		ID           int     `json:"id"`
		TargetBranch *string `json:"target_branch"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": "e2e-branch-default", "title": "branch default test"},
		&created,
	))
	if created.TargetBranch == nil || *created.TargetBranch != "dev" {
		got := "<nil>"
		if created.TargetBranch != nil {
			got = *created.TargetBranch
		}
		t.Errorf("create response target_branch: want %q got %q", "dev", got)
	}

	var show struct {
		Issue struct {
			ID           int     `json:"id"`
			TargetBranch *string `json:"target_branch"`
		} `json:"issue"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		fmt.Sprintf("/api/dx/todo/issue/show?slug=e2e-branch-default&id=IS-%d", created.ID),
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

func TestKpiSampleAndTrend(t *testing.T) {
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-kpi", "name": "E2E KPI"},
		nil,
	)

	var s1 struct {
		ID        int64  `json:"id"`
		SampledAt string `json:"sampled_at"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/kpi/sample",
		map[string]any{"slug": "e2e-kpi", "scope": "tech", "check_name": "coverage", "value": 82.5, "unit": "%"},
		&s1,
	))
	if s1.ID == 0 {
		t.Error("first sample: id should be non-zero")
	}
	if s1.SampledAt == "" {
		t.Error("first sample: sampled_at should be non-empty")
	}

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/kpi/sample",
		map[string]any{"slug": "e2e-kpi", "scope": "tech", "check_name": "coverage", "value": 83.0, "unit": "%"},
		nil,
	))

	var trend struct {
		Items []struct {
			ID        int64   `json:"id"`
			SampledAt string  `json:"sampled_at"`
			Scope     string  `json:"scope"`
			CheckName string  `json:"check_name"`
			Value     float64 `json:"value"`
			Unit      string  `json:"unit"`
		} `json:"items"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		"/api/dx/kpi/trend?slug=e2e-kpi&scope=tech&check_name=coverage&n=5",
		nil, &trend,
	))
	if len(trend.Items) != 2 {
		t.Errorf("trend items: want 2, got %d", len(trend.Items))
	}
	if len(trend.Items) >= 2 && trend.Items[0].Value < trend.Items[1].Value {
		t.Error("trend: items should be ordered by sampled_at desc (newest first)")
	}

	var empty struct {
		Items []struct{} `json:"items"`
	}
	mustOK(t, apiDo(t, http.MethodGet,
		"/api/dx/kpi/trend?slug=e2e-kpi&scope=tech&check_name=unknown-check&n=5",
		nil, &empty,
	))
	if len(empty.Items) != 0 {
		t.Errorf("unknown check_name: want 0 items, got %d", len(empty.Items))
	}
}
