package e2e

import (
	"net/http"
	"testing"
)

// TestDemoAPI_AdminProjectsList demonstrates that GET /api/projects returns the
// full project list with name, slug, classification, and git_enabled — covering spec 103.
func TestDemoAPI_AdminProjectsList(t *testing.T) {
	rec := newApiRecorder(t, "admin-projects-list")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_projects_test.go", Note: "API demo source"})
	t.Cleanup(rec.Save)

	// Create a plain project (no git).
	var plain struct {
		Slug           string `json:"slug"`
		Name           string `json:"name"`
		Classification string `json:"classification"`
		GitEnabled     bool   `json:"git_enabled"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-proj-plain",
		"name":           "Demo Plain Project",
		"classification": "service",
	}, &plain))
	if plain.GitEnabled {
		t.Error("plain project: git_enabled should be false")
	}

	// Create a git-enabled project.
	var gitProj struct {
		Slug       string `json:"slug"`
		GitEnabled bool   `json:"git_enabled"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-proj-git",
		"name":           "Demo Git Project",
		"classification": "tool",
		"upstream_url":   "https://github.com/example/demo-git-proj.git",
	}, &gitProj))
	if !gitProj.GitEnabled {
		t.Error("git project: git_enabled should be true")
	}

	// List all projects and verify both appear with the correct fields.
	var list struct {
		Projects []struct {
			Slug           string `json:"slug"`
			Name           string `json:"name"`
			Classification string `json:"classification"`
			GitEnabled     bool   `json:"git_enabled"`
		} `json:"projects"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/projects", nil, &list))

	found := map[string]bool{}
	for _, p := range list.Projects {
		switch p.Slug {
		case "demo-proj-plain":
			if p.Name == "" {
				t.Error("plain project: missing name in list")
			}
			if p.Classification != "service" {
				t.Errorf("plain project classification: want service, got %q", p.Classification)
			}
			if p.GitEnabled {
				t.Error("plain project: git_enabled should be false in list")
			}
			found["plain"] = true
		case "demo-proj-git":
			if p.Name == "" {
				t.Error("git project: missing name in list")
			}
			if p.Classification != "tool" {
				t.Errorf("git project classification: want tool, got %q", p.Classification)
			}
			if !p.GitEnabled {
				t.Error("git project: git_enabled should be true in list")
			}
			found["git"] = true
		}
	}
	if !found["plain"] {
		t.Error("plain project not found in /api/projects list")
	}
	if !found["git"] {
		t.Error("git project not found in /api/projects list")
	}
}

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
