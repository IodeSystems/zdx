package e2e

import (
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_GoalListVision demonstrates spec 109: when goals are listed for a
// project that has a vision set, the response includes the vision (title +
// description) so agents and reviewers can evaluate goals against project purpose.
// A project with no vision set returns empty vision fields without error.
func TestDemoAPI_GoalListVision(t *testing.T) {
	rec := newApiRecorder(t, "goal-list-vision")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "list-goals handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-goal-vision", "name": "Demo Goal Vision"}, nil))

	mustOK(t, rec.Do(http.MethodPut, "/api/project-vision", map[string]any{
		"slug":        "demo-goal-vision",
		"title":       "Principled SDLC for every developer",
		"description": "Give any developer product management superpowers without requiring a PM background",
	}, nil))

	mustOK(t, rec.Do(http.MethodPost, "/api/goal", map[string]any{
		"slug":        "demo-goal-vision",
		"title":       "100% developer adoption",
		"description": "Every developer uses zdx daily",
		"priority":    1,
		"status":      "active",
	}, nil))

	var list struct {
		Goals []struct {
			Title string `json:"title"`
		} `json:"goals"`
		Vision struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"vision"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/goals?slug=demo-goal-vision", nil, &list))

	if list.Vision.Title != "Principled SDLC for every developer" {
		t.Errorf("vision.title: want %q got %q", "Principled SDLC for every developer", list.Vision.Title)
	}
	if list.Vision.Description == "" {
		t.Error("vision.description: want non-empty")
	}
	if len(list.Goals) == 0 {
		t.Error("goals: want at least one goal")
	}

	// Project with no vision set — empty vision fields, no error.
	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-goal-no-vision", "name": "Demo Goal No Vision"}, nil))
	mustOK(t, rec.Do(http.MethodPost, "/api/goal", map[string]any{
		"slug":        "demo-goal-no-vision",
		"title":       "Some goal",
		"description": "",
		"priority":    1,
		"status":      "active",
	}, nil))

	var listNoVision struct {
		Vision struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		} `json:"vision"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/goals?slug=demo-goal-no-vision", nil, &listNoVision))
	if listNoVision.Vision.Title != "" {
		t.Errorf("no-vision project: vision.title want empty, got %q", listNoVision.Vision.Title)
	}
}

// TestDemoAPI_GoalCreatePersists demonstrates spec 48: when a goal is created
// with title/description/priority/status, the response carries every field
// (including server-set created_at/updated_at) and a follow-up list query
// returns the same persisted row.
func TestDemoAPI_GoalCreatePersists(t *testing.T) {
	rec := newApiRecorder(t, "goal-create-persists")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "create-goal handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-goal-create", "name": "Demo Goal Create"}, nil))

	var created struct {
		ID          int32  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int32  `json:"priority"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/goal", map[string]any{
		"slug":        "demo-goal-create",
		"title":       "Reach 99.9% uptime",
		"description": "Drive reliability across all production services",
		"priority":    1,
		"status":      "active",
	}, &created))

	if created.ID == 0 {
		t.Error("id: want non-zero")
	}
	if created.Title != "Reach 99.9% uptime" {
		t.Errorf("title: want %q got %q", "Reach 99.9% uptime", created.Title)
	}
	if created.Description != "Drive reliability across all production services" {
		t.Errorf("description: want non-empty match, got %q", created.Description)
	}
	if created.Priority != 1 {
		t.Errorf("priority: want 1 got %d", created.Priority)
	}
	if created.Status != "active" {
		t.Errorf("status: want %q got %q", "active", created.Status)
	}
	if created.CreatedAt == "" {
		t.Error("created_at: want non-empty timestamp")
	}
	if created.UpdatedAt == "" {
		t.Error("updated_at: want non-empty timestamp")
	}
	if _, err := time.Parse(time.RFC3339, created.CreatedAt); err != nil {
		t.Errorf("created_at: parse error %v", err)
	}
	if _, err := time.Parse(time.RFC3339, created.UpdatedAt); err != nil {
		t.Errorf("updated_at: parse error %v", err)
	}

	var list struct {
		Goals []struct {
			ID          int32  `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int32  `json:"priority"`
			Status      string `json:"status"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"goals"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/goals?slug=demo-goal-create", nil, &list))
	found := false
	for _, g := range list.Goals {
		if g.ID != created.ID {
			continue
		}
		found = true
		if g.Title != created.Title {
			t.Errorf("list title: want %q got %q", created.Title, g.Title)
		}
		if g.Description != created.Description {
			t.Errorf("list description: want %q got %q", created.Description, g.Description)
		}
		if g.Priority != created.Priority {
			t.Errorf("list priority: want %d got %d", created.Priority, g.Priority)
		}
		if g.Status != created.Status {
			t.Errorf("list status: want %q got %q", created.Status, g.Status)
		}
		if g.CreatedAt != created.CreatedAt {
			t.Errorf("list created_at: want %q got %q", created.CreatedAt, g.CreatedAt)
		}
	}
	if !found {
		t.Fatal("created goal not returned by /api/goals")
	}
}

// TestDemoAPI_GoalUpdatePersists demonstrates spec 51 for goals: when an
// existing goal is updated, title/description/priority/status round-trip
// through a list query and updated_at advances past the original timestamp.
func TestDemoAPI_GoalUpdatePersists(t *testing.T) {
	rec := newApiRecorder(t, "goal-update-persists")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "update-goal handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-goal-update", "name": "Demo Goal Update"}, nil))

	var created struct {
		ID        int32  `json:"id"`
		UpdatedAt string `json:"updated_at"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/goal", map[string]any{
		"slug":        "demo-goal-update",
		"title":       "Reach 99.9% uptime",
		"description": "Drive reliability across all production services",
		"priority":    1,
		"status":      "active",
	}, &created))
	if created.ID == 0 {
		t.Fatal("create: id want non-zero")
	}
	preUpdate, err := time.Parse(time.RFC3339, created.UpdatedAt)
	if err != nil {
		t.Fatalf("create updated_at parse: %v", err)
	}

	// Sleep so updated_at can advance past create timestamp at second resolution.
	time.Sleep(1100 * time.Millisecond)

	mustOK(t, rec.Do(http.MethodPut, "/api/goal", map[string]any{
		"id":          created.ID,
		"title":       "Reach 99.99% uptime",
		"description": "Tighten reliability bar across critical paths",
		"priority":    2,
		"status":      "paused",
	}, nil))

	var list struct {
		Goals []struct {
			ID          int32  `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int32  `json:"priority"`
			Status      string `json:"status"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"goals"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/goals?slug=demo-goal-update", nil, &list))
	found := false
	for _, g := range list.Goals {
		if g.ID != created.ID {
			continue
		}
		found = true
		if g.Title != "Reach 99.99% uptime" {
			t.Errorf("title: want %q got %q", "Reach 99.99% uptime", g.Title)
		}
		if g.Description != "Tighten reliability bar across critical paths" {
			t.Errorf("description: want updated, got %q", g.Description)
		}
		if g.Priority != 2 {
			t.Errorf("priority: want 2 got %d", g.Priority)
		}
		if g.Status != "paused" {
			t.Errorf("status: want %q got %q", "paused", g.Status)
		}
		postUpdate, err := time.Parse(time.RFC3339, g.UpdatedAt)
		if err != nil {
			t.Errorf("updated_at parse: %v", err)
		} else if !postUpdate.After(preUpdate) {
			t.Errorf("updated_at: want advance past %s, got %s", preUpdate, postUpdate)
		}
	}
	if !found {
		t.Fatal("updated goal not returned by /api/goals")
	}
}

// TestDemoAPI_GoalDeleteRemovesFromList demonstrates spec 52: when an existing
// goal is deleted, a follow-up list query no longer returns that goal.
func TestDemoAPI_GoalDeleteRemovesFromList(t *testing.T) {
	rec := newApiRecorder(t, "goal-delete-removes-from-list")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "delete-goal handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-goal-delete", "name": "Demo Goal Delete"}, nil))

	var created struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/goal", map[string]any{
		"slug":        "demo-goal-delete",
		"title":       "Reach 99.9% uptime",
		"description": "Drive reliability across all production services",
		"priority":    1,
		"status":      "active",
	}, &created))
	if created.ID == 0 {
		t.Fatal("create: id want non-zero")
	}

	mustOK(t, rec.Do(http.MethodDelete, "/api/goal", map[string]any{"id": created.ID}, nil))

	var list struct {
		Goals []struct {
			ID int32 `json:"id"`
		} `json:"goals"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/goals?slug=demo-goal-delete", nil, &list))
	for _, g := range list.Goals {
		if g.ID == created.ID {
			t.Fatal("deleted goal must not appear in list")
		}
	}
}

// TestDemoAPI_ConstraintDeleteRemovesFromList demonstrates spec 52: when an
// existing constraint is deleted, a follow-up list query no longer returns it.
// func TestDemoAPI_ConstraintDeleteRemovesFromList(t *testing.T) { — removed in IS-627: zdx_project_constraints + /api/constraint dropped.

func TestGoalCRUD(t *testing.T) {
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-goals", "name": "E2E Goals"}, nil)

	var goal struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/goal",
		map[string]any{"slug": "e2e-goals", "title": "Ship fast", "description": "Move quickly", "priority": 1, "status": "active"},
		&goal))
	if goal.Title != "Ship fast" {
		t.Fatalf("title: want %q got %q", "Ship fast", goal.Title)
	}
	if goal.CreatedAt == "" {
		t.Fatal("created_at should be set")
	}

	preUpdateAt, _ := time.Parse(time.RFC3339, goal.UpdatedAt)

	// Update — change status to "paused" to verify round-trip
	mustOK(t, apiDo(t, http.MethodPut, "/api/goal",
		map[string]any{"id": goal.ID, "title": "Ship safely", "description": "Move carefully", "priority": 2, "status": "paused"}, nil))

	// Verify update via list
	var list struct {
		Goals []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
			Status      string `json:"status"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"goals"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/goals?slug=e2e-goals", nil, &list))
	found := false
	for _, g := range list.Goals {
		if g.ID == goal.ID {
			found = true
			if g.Title != "Ship safely" {
				t.Errorf("updated title: want %q got %q", "Ship safely", g.Title)
			}
			if g.Description != "Move carefully" {
				t.Errorf("updated description: want %q got %q", "Move carefully", g.Description)
			}
			if g.Priority != 2 {
				t.Errorf("updated priority: want 2 got %d", g.Priority)
			}
			if g.Status != "paused" {
				t.Errorf("updated status: want %q got %q", "paused", g.Status)
			}
			postUpdateAt, err := time.Parse(time.RFC3339, g.UpdatedAt)
			if err != nil {
				t.Errorf("updated_at parse error: %v", err)
			} else if postUpdateAt.Before(preUpdateAt) {
				t.Errorf("updated_at should not regress: pre=%s post=%s", preUpdateAt, postUpdateAt)
			}
		}
	}
	if !found {
		t.Fatal("goal not found in list")
	}

	// Delete
	mustOK(t, apiDo(t, http.MethodDelete, "/api/goal",
		map[string]any{"id": goal.ID}, nil))

	// Verify deletion
	mustOK(t, apiDo(t, http.MethodGet, "/api/goals?slug=e2e-goals", nil, &list))
	for _, g := range list.Goals {
		if g.ID == goal.ID {
			t.Fatal("goal should be deleted")
		}
	}
}

// TestDemoAPI_ConstraintCreatePersists demonstrates spec 49: when a constraint is
// created with title/description/priority/status, the response carries every field
// (including server-set created_at/updated_at) and a follow-up list query returns
// the same persisted row.
// func TestDemoAPI_ConstraintCreatePersists(t *testing.T) { — removed in IS-627: zdx_project_constraints + /api/constraint dropped.

// TestDemoAPI_ConstraintUpdatePersists demonstrates spec 51 for constraints:
// when an existing constraint is updated, title/description/priority/status
// round-trip through a list query and updated_at advances past the original
// timestamp.
// func TestDemoAPI_ConstraintUpdatePersists(t *testing.T) { — removed in IS-627: zdx_project_constraints + /api/constraint dropped.

// func TestConstraintCRUD(t *testing.T) { — removed in IS-627: zdx_project_constraints + /api/constraint dropped.

// TestDemoAPI_GoalConstraintListOrdering demonstrates spec 50: given existing
// goals and constraints, when listed for a project, they are returned ordered
// by priority (ascending) then title (alphabetical) within each priority tier.
// func TestDemoAPI_GoalConstraintListOrdering(t *testing.T) { — removed in IS-627: zdx_project_constraints + /api/constraint dropped.

// func TestConstraintListOrdering(t *testing.T) { — removed in IS-627: zdx_project_constraints + /api/constraint dropped.

func TestGoalListOrdering(t *testing.T) {
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-goal-order", "name": "E2E Goal Order"}, nil)

	mustOK(t, apiDo(t, http.MethodPost, "/api/goal",
		map[string]any{"slug": "e2e-goal-order", "title": "Zebra", "description": "", "priority": 2, "status": "active"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/goal",
		map[string]any{"slug": "e2e-goal-order", "title": "Alpha", "description": "", "priority": 2, "status": "active"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/goal",
		map[string]any{"slug": "e2e-goal-order", "title": "Top priority", "description": "", "priority": 1, "status": "active"}, nil))

	var list struct {
		Goals []struct {
			Title    string `json:"title"`
			Priority int    `json:"priority"`
		} `json:"goals"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/goals?slug=e2e-goal-order", nil, &list))

	if len(list.Goals) < 3 {
		t.Fatalf("expected at least 3 goals, got %d", len(list.Goals))
	}
	if list.Goals[0].Title != "Top priority" {
		t.Errorf("first goal should be highest priority, got %q", list.Goals[0].Title)
	}
	if list.Goals[1].Title != "Alpha" {
		t.Errorf("second goal should be Alpha (p2, alphabetical), got %q", list.Goals[1].Title)
	}
	if list.Goals[2].Title != "Zebra" {
		t.Errorf("third goal should be Zebra (p2, alphabetical), got %q", list.Goals[2].Title)
	}
}
