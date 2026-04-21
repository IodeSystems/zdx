package e2e

import (
	"net/http"
	"testing"
)

func TestAdminProjects_CreateWithClassification(t *testing.T) {
	var created struct {
		ID             int32  `json:"id"`
		Slug           string `json:"slug"`
		Name           string `json:"name"`
		Classification string `json:"classification"`
		GitEnabled     bool   `json:"git_enabled"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/project", map[string]any{
		"slug":           "e2e-admin-proj-service",
		"name":           "E2E Admin Project Service",
		"classification": "service",
	}, &created))
	if created.Slug != "e2e-admin-proj-service" {
		t.Errorf("slug: want %q got %q", "e2e-admin-proj-service", created.Slug)
	}
	if created.Classification != "service" {
		t.Errorf("classification: want %q got %q", "service", created.Classification)
	}
	if created.GitEnabled {
		t.Error("git_enabled: want false for project created without upstream_url")
	}

	var list struct {
		Projects []struct {
			Slug           string `json:"slug"`
			Classification string `json:"classification"`
			GitEnabled     bool   `json:"git_enabled"`
		} `json:"projects"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/projects", nil, &list))
	var found *struct {
		Slug           string `json:"slug"`
		Classification string `json:"classification"`
		GitEnabled     bool   `json:"git_enabled"`
	}
	for i := range list.Projects {
		if list.Projects[i].Slug == "e2e-admin-proj-service" {
			found = &list.Projects[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("created project not in /api/projects list")
	}
	if found.Classification != "service" {
		t.Errorf("list classification: want %q got %q", "service", found.Classification)
	}
	if found.GitEnabled {
		t.Error("list git_enabled: want false")
	}
}

func TestAdminProjects_CreateBindExisting(t *testing.T) {
	var created struct {
		Slug       string `json:"slug"`
		GitEnabled bool   `json:"git_enabled"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/project", map[string]any{
		"slug":           "e2e-admin-proj-bound",
		"name":           "E2E Admin Bound Repo",
		"classification": "tool",
		"upstream_url":   "https://github.com/example/e2e-bound.git",
	}, &created))
	if !created.GitEnabled {
		t.Error("git_enabled: want true when upstream_url is set")
	}

	var list struct {
		Projects []struct {
			Slug       string `json:"slug"`
			GitEnabled bool   `json:"git_enabled"`
		} `json:"projects"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/projects", nil, &list))
	for _, p := range list.Projects {
		if p.Slug == "e2e-admin-proj-bound" {
			if !p.GitEnabled {
				t.Error("list git_enabled: want true for bound project")
			}
			return
		}
	}
	t.Fatal("bound project not in /api/projects list")
}

func TestAdminProjects_InvalidClassification(t *testing.T) {
	resp := apiDo(t, http.MethodPost, "/api/project", map[string]any{
		"slug":           "e2e-admin-proj-invalid",
		"name":           "E2E Admin Invalid",
		"classification": "bogus",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
	}
}
