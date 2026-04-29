package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestDemoAPI_ShipTodoTargetsReleaseBranch is the demo for spec 157:
// Ship button click creates a ship todo targeting the environment's release branch.
//
// Covers two cases:
//  1. Environment with release_branch set — title includes branch name, text references it.
//  2. Environment without release_branch — fallback title/text, no branch wording.
func TestDemoAPI_ShipTodoTargetsReleaseBranch(t *testing.T) {
	rec := newApiRecorder(t, "ship-todo-targets-release-branch")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_ship_todo_test.go", Note: "spec 157 demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_environments.go", Note: "POST /api/dx/projects/{slug}/environments/{name}/request — creates ship todo"})
	t.Cleanup(rec.Save)

	const slug = "demo-ship-todo-rel"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Ship Todo",
	}, nil))

	// --- case 1: environment with release_branch ---
	mustOK(t, rec.Do(http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]any{
			"name":           "staging",
			"release_branch": "release/staging",
		}, nil))

	var todo1 struct {
		ID         int32  `json:"id"`
		Title      string `json:"title"`
		Text       string `json:"text"`
		Kind       string `json:"kind"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
	}
	mustOK(t, rec.Do(http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments/staging/request", slug),
		map[string]any{"kind": "ship"},
		&todo1,
	))

	if todo1.ID == 0 {
		t.Fatal("expected non-zero todo id")
	}
	if todo1.Kind != "ship" {
		t.Errorf("kind: want ship, got %q", todo1.Kind)
	}
	if todo1.TargetType != "environment" {
		t.Errorf("target_type: want environment, got %q", todo1.TargetType)
	}
	if todo1.TargetID != "staging" {
		t.Errorf("target_id: want staging, got %q", todo1.TargetID)
	}
	if todo1.Title != "Ship to staging from release/staging" {
		t.Errorf("title: want %q, got %q", "Ship to staging from release/staging", todo1.Title)
	}
	if !strings.Contains(todo1.Text, "release/staging") {
		t.Errorf("text should reference release branch, got: %q", todo1.Text)
	}

	// --- case 2: environment without release_branch (fallback) ---
	mustOK(t, rec.Do(http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]any{"name": "prod"}, nil))

	var todo2 struct {
		ID    int32  `json:"id"`
		Title string `json:"title"`
		Text  string `json:"text"`
		Kind  string `json:"kind"`
	}
	mustOK(t, rec.Do(http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments/prod/request", slug),
		map[string]any{"kind": "ship"},
		&todo2,
	))

	if todo2.ID == 0 {
		t.Fatal("expected non-zero todo id for fallback case")
	}
	if todo2.Kind != "ship" {
		t.Errorf("fallback kind: want ship, got %q", todo2.Kind)
	}
	if todo2.Title != "Ship to prod" {
		t.Errorf("fallback title: want %q, got %q", "Ship to prod", todo2.Title)
	}
	if strings.Contains(todo2.Text, "release_branch") || strings.Contains(todo2.Text, "release/") {
		t.Errorf("fallback text should not reference a release branch, got: %q", todo2.Text)
	}
}
