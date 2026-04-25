package e2e

import (
	"net/http"
	"testing"
	"time"
)

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
func TestDemoAPI_ConstraintDeleteRemovesFromList(t *testing.T) {
	rec := newApiRecorder(t, "constraint-delete-removes-from-list")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "delete-constraint handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-constraint-delete", "name": "Demo Constraint Delete"}, nil))

	var created struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/constraint", map[string]any{
		"slug":        "demo-constraint-delete",
		"title":       "No breaking API changes",
		"description": "All public API changes must be backward compatible",
		"priority":    1,
		"status":      "active",
	}, &created))
	if created.ID == 0 {
		t.Fatal("create: id want non-zero")
	}

	mustOK(t, rec.Do(http.MethodDelete, "/api/constraint", map[string]any{"id": created.ID}, nil))

	var list struct {
		Constraints []struct {
			ID int32 `json:"id"`
		} `json:"constraints"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/constraints?slug=demo-constraint-delete", nil, &list))
	for _, c := range list.Constraints {
		if c.ID == created.ID {
			t.Fatal("deleted constraint must not appear in list")
		}
	}
}

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
func TestDemoAPI_ConstraintCreatePersists(t *testing.T) {
	rec := newApiRecorder(t, "constraint-create-persists")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "create-constraint handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-constraint-create", "name": "Demo Constraint Create"}, nil))

	var created struct {
		ID          int32  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int32  `json:"priority"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/constraint", map[string]any{
		"slug":        "demo-constraint-create",
		"title":       "No breaking API changes",
		"description": "All public API changes must be backward compatible",
		"priority":    1,
		"status":      "active",
	}, &created))

	if created.ID == 0 {
		t.Error("id: want non-zero")
	}
	if created.Title != "No breaking API changes" {
		t.Errorf("title: want %q got %q", "No breaking API changes", created.Title)
	}
	if created.Description != "All public API changes must be backward compatible" {
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
		Constraints []struct {
			ID          int32  `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int32  `json:"priority"`
			Status      string `json:"status"`
			CreatedAt   string `json:"created_at"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"constraints"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/constraints?slug=demo-constraint-create", nil, &list))
	found := false
	for _, c := range list.Constraints {
		if c.ID != created.ID {
			continue
		}
		found = true
		if c.Title != created.Title {
			t.Errorf("list title: want %q got %q", created.Title, c.Title)
		}
		if c.Description != created.Description {
			t.Errorf("list description: want %q got %q", created.Description, c.Description)
		}
		if c.Priority != created.Priority {
			t.Errorf("list priority: want %d got %d", created.Priority, c.Priority)
		}
		if c.Status != created.Status {
			t.Errorf("list status: want %q got %q", created.Status, c.Status)
		}
		if c.CreatedAt != created.CreatedAt {
			t.Errorf("list created_at: want %q got %q", created.CreatedAt, c.CreatedAt)
		}
	}
	if !found {
		t.Fatal("created constraint not returned by /api/constraints")
	}
}

// TestDemoAPI_ConstraintUpdatePersists demonstrates spec 51 for constraints:
// when an existing constraint is updated, title/description/priority/status
// round-trip through a list query and updated_at advances past the original
// timestamp.
func TestDemoAPI_ConstraintUpdatePersists(t *testing.T) {
	rec := newApiRecorder(t, "constraint-update-persists")
	rec.AddCoderef(coderef{FilePath: "test/e2e/goals_test.go", Note: "API demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_projects.go", Note: "update-constraint handler"})
	t.Cleanup(rec.Save)

	mustOK(t, rec.Do(http.MethodPost, "/api/project",
		map[string]any{"slug": "demo-constraint-update", "name": "Demo Constraint Update"}, nil))

	var created struct {
		ID        int32  `json:"id"`
		UpdatedAt string `json:"updated_at"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/constraint", map[string]any{
		"slug":        "demo-constraint-update",
		"title":       "No breaking API changes",
		"description": "All public API changes must be backward compatible",
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

	time.Sleep(1100 * time.Millisecond)

	mustOK(t, rec.Do(http.MethodPut, "/api/constraint", map[string]any{
		"id":          created.ID,
		"title":       "Minimize breaking API changes",
		"description": "Breaking changes require a deprecation window",
		"priority":    2,
		"status":      "paused",
	}, nil))

	var list struct {
		Constraints []struct {
			ID          int32  `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int32  `json:"priority"`
			Status      string `json:"status"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"constraints"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/constraints?slug=demo-constraint-update", nil, &list))
	found := false
	for _, c := range list.Constraints {
		if c.ID != created.ID {
			continue
		}
		found = true
		if c.Title != "Minimize breaking API changes" {
			t.Errorf("title: want %q got %q", "Minimize breaking API changes", c.Title)
		}
		if c.Description != "Breaking changes require a deprecation window" {
			t.Errorf("description: want updated, got %q", c.Description)
		}
		if c.Priority != 2 {
			t.Errorf("priority: want 2 got %d", c.Priority)
		}
		if c.Status != "paused" {
			t.Errorf("status: want %q got %q", "paused", c.Status)
		}
		postUpdate, err := time.Parse(time.RFC3339, c.UpdatedAt)
		if err != nil {
			t.Errorf("updated_at parse: %v", err)
		} else if !postUpdate.After(preUpdate) {
			t.Errorf("updated_at: want advance past %s, got %s", preUpdate, postUpdate)
		}
	}
	if !found {
		t.Fatal("updated constraint not returned by /api/constraints")
	}
}

func TestConstraintCRUD(t *testing.T) {
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": "e2e-constraints", "name": "E2E Constraints"}, nil)

	var constraint struct {
		ID          int    `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int    `json:"priority"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/constraint",
		map[string]any{"slug": "e2e-constraints", "title": "No downtime", "description": "Zero-downtime deploys", "priority": 1, "status": "active"},
		&constraint))
	if constraint.Title != "No downtime" {
		t.Fatalf("title: want %q got %q", "No downtime", constraint.Title)
	}
	if constraint.CreatedAt == "" {
		t.Fatal("created_at should be set")
	}

	preUpdateAt, _ := time.Parse(time.RFC3339, constraint.UpdatedAt)

	// Update — change status to "paused" to verify round-trip
	mustOK(t, apiDo(t, http.MethodPut, "/api/constraint",
		map[string]any{"id": constraint.ID, "title": "Minimal downtime", "description": "Under 5 min", "priority": 2, "status": "paused"}, nil))

	// Verify update via list
	var list struct {
		Constraints []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
			Status      string `json:"status"`
			UpdatedAt   string `json:"updated_at"`
		} `json:"constraints"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/constraints?slug=e2e-constraints", nil, &list))
	found := false
	for _, c := range list.Constraints {
		if c.ID == constraint.ID {
			found = true
			if c.Title != "Minimal downtime" {
				t.Errorf("updated title: want %q got %q", "Minimal downtime", c.Title)
			}
			if c.Description != "Under 5 min" {
				t.Errorf("updated description: want %q got %q", "Under 5 min", c.Description)
			}
			if c.Priority != 2 {
				t.Errorf("updated priority: want 2 got %d", c.Priority)
			}
			if c.Status != "paused" {
				t.Errorf("updated status: want %q got %q", "paused", c.Status)
			}
			postUpdateAt, err := time.Parse(time.RFC3339, c.UpdatedAt)
			if err != nil {
				t.Errorf("updated_at parse error: %v", err)
			} else if postUpdateAt.Before(preUpdateAt) {
				t.Errorf("updated_at should not regress: pre=%s post=%s", preUpdateAt, postUpdateAt)
			}
		}
	}
	if !found {
		t.Fatal("constraint not found in list")
	}

	// Delete
	mustOK(t, apiDo(t, http.MethodDelete, "/api/constraint",
		map[string]any{"id": constraint.ID}, nil))

	// Verify deletion
	mustOK(t, apiDo(t, http.MethodGet, "/api/constraints?slug=e2e-constraints", nil, &list))
	for _, c := range list.Constraints {
		if c.ID == constraint.ID {
			t.Fatal("constraint should be deleted")
		}
	}
}

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
