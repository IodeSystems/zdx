package e2e

import (
	"net/http"
	"testing"
)

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

	// Update
	mustOK(t, apiDo(t, http.MethodPut, "/api/goal",
		map[string]any{"id": goal.ID, "title": "Ship safely", "description": "Move carefully", "priority": 2, "status": "active"}, nil))

	// Verify update via list
	var list struct {
		Goals []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
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
			if g.Priority != 2 {
				t.Errorf("updated priority: want 2 got %d", g.Priority)
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

	// Update
	mustOK(t, apiDo(t, http.MethodPut, "/api/constraint",
		map[string]any{"id": constraint.ID, "title": "Minimal downtime", "description": "Under 5 min", "priority": 2, "status": "active"}, nil))

	// Verify update via list
	var list struct {
		Constraints []struct {
			ID          int    `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Priority    int    `json:"priority"`
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
			if c.Priority != 2 {
				t.Errorf("updated priority: want 2 got %d", c.Priority)
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
