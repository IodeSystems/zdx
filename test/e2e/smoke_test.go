package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		t.Fatalf("want 200, got %d", resp.StatusCode)
	}
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestHealth(t *testing.T) {
	resp, err := http.Get(srv.URL + "/api/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", resp.StatusCode)
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
