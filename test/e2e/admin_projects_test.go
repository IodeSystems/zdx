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

// TestDemoAPI_AdminProjectsCreate demonstrates that submitting a new project with
// name + classification (no upstream_url) creates it with that classification and
// git_enabled=false — covering spec 104.
func TestDemoAPI_AdminProjectsCreate(t *testing.T) {
	rec := newApiRecorder(t, "admin-projects-create")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_projects_test.go", Note: "API demo source"})
	t.Cleanup(rec.Save)

	var created struct {
		Slug           string `json:"slug"`
		Name           string `json:"name"`
		Classification string `json:"classification"`
		GitEnabled     bool   `json:"git_enabled"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-admin-create-saas",
		"name":           "Demo Admin Create SaaS",
		"classification": "saas",
	}, &created))
	if created.Classification != "saas" {
		t.Errorf("classification: want %q, got %q", "saas", created.Classification)
	}
	if created.GitEnabled {
		t.Error("git_enabled: want false when upstream_url is omitted")
	}

	var list struct {
		Projects []struct {
			Slug           string `json:"slug"`
			Classification string `json:"classification"`
			GitEnabled     bool   `json:"git_enabled"`
		} `json:"projects"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/projects", nil, &list))
	found := false
	for _, p := range list.Projects {
		if p.Slug == "demo-admin-create-saas" {
			found = true
			if p.Classification != "saas" {
				t.Errorf("list classification: want %q, got %q", "saas", p.Classification)
			}
			if p.GitEnabled {
				t.Error("list git_enabled: want false")
			}
			break
		}
	}
	if !found {
		t.Fatal("created project not in /api/projects list")
	}
}

// TestDemoAPI_AdminProjectsAddExisting demonstrates that submitting a GitHub URL
// via the Add Existing tab creates a project with upstream_url set and git_enabled=true
// — covering spec 105.
func TestDemoAPI_AdminProjectsAddExisting(t *testing.T) {
	rec := newApiRecorder(t, "admin-projects-add-existing")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_projects_test.go", Note: "API demo source"})
	t.Cleanup(rec.Save)

	var created struct {
		Slug       string `json:"slug"`
		Name       string `json:"name"`
		GitEnabled bool   `json:"git_enabled"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-add-existing-gh",
		"name":           "Demo Add Existing GitHub",
		"classification": "tool",
		"upstream_url":   "https://github.com/example/demo-add-existing.git",
	}, &created))
	if !created.GitEnabled {
		t.Error("git_enabled: want true when upstream_url is a GitHub URL")
	}
}

// TestDemoAPI_AdminProjectsInvalidClassification demonstrates that submitting a
// create-project request with an unknown classification is rejected with 400 —
// covering spec 106.
func TestDemoAPI_AdminProjectsInvalidClassification(t *testing.T) {
	rec := newApiRecorder(t, "admin-projects-invalid-classification")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_projects_test.go", Note: "API demo source"})
	t.Cleanup(rec.Save)

	resp := rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug":           "demo-admin-invalid-class",
		"name":           "Demo Admin Invalid Classification",
		"classification": "bogus",
	}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", resp.StatusCode)
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

// TestDemoAPI_ProjectDescriptionPersists demonstrates spec 108: a project has a
// description field (title + description) that captures its vision/tagline and
// persists after being set via PUT /api/project-vision.
func TestDemoAPI_ProjectDescriptionPersists(t *testing.T) {
	rec := newApiRecorder(t, "project-description-persists")
	rec.AddCoderef(coderef{FilePath: "test/e2e/admin_projects_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "set-project-vision handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": "demo-proj-vision",
		"name": "Demo Project Vision",
	}, nil))

	// Verify title/description are empty before setting vision.
	var listBefore struct {
		Projects []struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"projects"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/projects", nil, &listBefore))
	foundBefore := false
	for _, p := range listBefore.Projects {
		if p.Slug == "demo-proj-vision" {
			foundBefore = true
			if p.Title != "" {
				t.Errorf("title before set: want empty, got %q", p.Title)
			}
			if p.Description != "" {
				t.Errorf("description before set: want empty, got %q", p.Description)
			}
			break
		}
	}
	if !foundBefore {
		t.Fatal("project not found in /api/projects before vision set")
	}

	// Set vision and verify the response echoes back the values.
	var vision struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	mustOK(t, rec.Do(http.MethodPut, "/api/project-vision", map[string]any{
		"slug":        "demo-proj-vision",
		"title":       "Principled SDLC for everyone",
		"description": "Ad-hoc platform giving developers product management superpowers",
	}, &vision))
	if vision.Title != "Principled SDLC for everyone" {
		t.Errorf("vision response title: want %q, got %q", "Principled SDLC for everyone", vision.Title)
	}
	if vision.Description != "Ad-hoc platform giving developers product management superpowers" {
		t.Errorf("vision response description: want non-empty match, got %q", vision.Description)
	}

	// Verify title/description persist in the project list.
	var listAfter struct {
		Projects []struct {
			Slug        string `json:"slug"`
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"projects"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/projects", nil, &listAfter))
	foundAfter := false
	for _, p := range listAfter.Projects {
		if p.Slug == "demo-proj-vision" {
			foundAfter = true
			if p.Title != vision.Title {
				t.Errorf("list title: want %q, got %q", vision.Title, p.Title)
			}
			if p.Description != vision.Description {
				t.Errorf("list description: want %q, got %q", vision.Description, p.Description)
			}
			break
		}
	}
	if !foundAfter {
		t.Fatal("project not found in /api/projects after vision set")
	}
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
