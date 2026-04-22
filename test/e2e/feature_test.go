package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func TestFeatureAdd(t *testing.T) {
	slug := "e2e-feat-add"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Feature Add Test"}, nil)

	var feat struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug":        slug,
		"name":        "test-feature",
		"description": "A test feature",
	}, &feat))
	if feat.Name != "test-feature" {
		t.Errorf("name: want %q got %q", "test-feature", feat.Name)
	}
	if feat.ID == 0 {
		t.Error("expected non-zero feature ID")
	}
}

func TestFeatureList(t *testing.T) {
	slug := "e2e-feat-list"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Feature List Test"}, nil)

	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "feat-alpha", "description": "Alpha feature",
	}, nil)
	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "feat-beta", "description": "Beta feature",
	}, nil)

	var resp struct {
		Features []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"features"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/features?slug="+slug, nil, &resp))
	if len(resp.Features) < 2 {
		t.Fatalf("want >= 2 features, got %d", len(resp.Features))
	}
	names := map[string]bool{}
	for _, f := range resp.Features {
		names[f.Name] = true
	}
	if !names["feat-alpha"] || !names["feat-beta"] {
		t.Errorf("missing expected features in list: %v", names)
	}
}

func TestFeatureShow(t *testing.T) {
	slug := "e2e-feat-show"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Feature Show Test"}, nil)

	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "show-me", "description": "Showable feature",
	}, nil)

	var resp struct {
		Features []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"features"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/features?slug="+slug, nil, &resp))
	found := false
	for _, f := range resp.Features {
		if f.Name == "show-me" {
			found = true
			if f.Description != "Showable feature" {
				t.Errorf("desc: want %q got %q", "Showable feature", f.Description)
			}
		}
	}
	if !found {
		t.Error("feature 'show-me' not found in list")
	}
}

func TestFeatureSet(t *testing.T) {
	slug := "e2e-feat-set"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Feature Set Test"}, nil)

	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "settable", "description": "Original desc",
	}, nil)

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/features/field", map[string]any{
		"slug": slug, "feature": "settable", "field": "what", "value": "It does things",
	}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/features/field", map[string]any{
		"slug": slug, "feature": "settable", "field": "why", "value": "Because reasons",
	}, nil))

	var resp struct {
		Features []struct {
			Name string `json:"name"`
			What string `json:"what"`
			Why  string `json:"why"`
		} `json:"features"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/features?slug="+slug, nil, &resp))
	for _, f := range resp.Features {
		if f.Name == "settable" {
			if f.What != "It does things" {
				t.Errorf("what: want %q got %q", "It does things", f.What)
			}
			if f.Why != "Because reasons" {
				t.Errorf("why: want %q got %q", "Because reasons", f.Why)
			}
			return
		}
	}
	t.Error("feature 'settable' not found")
}

func TestSpecDeferralAcceptsWipIssue(t *testing.T) {
	slug := "e2e-spec-defer-wip"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Spec Deferral Wip Test"}, nil)
	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "defer-feat", "description": "Feature for deferral test",
	}, nil)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/update", map[string]any{
		"slug": slug, "feature": "defer-feat", "field": "must", "value": "spec to defer",
	}, nil))

	var featResp struct {
		Specs []struct {
			ID int32 `json:"id"`
		} `json:"specs"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/dx/feature?slug=%s&name=defer-feat", slug), nil, &featResp))
	if len(featResp.Specs) == 0 {
		t.Fatal("expected at least one spec")
	}
	specID := featResp.Specs[0].ID

	// wip issue (auto_ready: false keeps status=wip)
	var issResp struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "wip issue", "auto_ready": false}, &issResp))
	wipID := fmt.Sprintf("IS-%d", issResp.ID)

	resp := apiDo(t, http.MethodPost, "/api/dx/specs/deferrals/add",
		map[string]any{"slug": slug, "spec_id": specID, "issue_id": wipID}, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("defer to wip issue: want 200, got %d", resp.StatusCode)
	}
}

func TestSpecDeferralRejectsClosedIssue(t *testing.T) {
	slug := "e2e-spec-defer-closed"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Spec Deferral Closed Test"}, nil)
	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "defer-feat", "description": "Feature for deferral test",
	}, nil)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/update", map[string]any{
		"slug": slug, "feature": "defer-feat", "field": "must", "value": "spec to defer",
	}, nil))

	var featResp struct {
		Specs []struct {
			ID int32 `json:"id"`
		} `json:"specs"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/dx/feature?slug=%s&name=defer-feat", slug), nil, &featResp))
	if len(featResp.Specs) == 0 {
		t.Fatal("expected at least one spec")
	}
	specID := featResp.Specs[0].ID

	// open issue, then close it
	var issResp struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "soon closed issue", "auto_ready": true}, &issResp))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{"slug": slug, "id": issResp.ID, "reason": "done"}, nil))
	closedID := fmt.Sprintf("IS-%d", issResp.ID)

	resp := apiDo(t, http.MethodPost, "/api/dx/specs/deferrals/add",
		map[string]any{"slug": slug, "spec_id": specID, "issue_id": closedID}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("defer to closed issue: want 400, got %d", resp.StatusCode)
	}
}

func TestFeatureReview(t *testing.T) {
	slug := "e2e-feat-review"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Feature Review Test"}, nil)

	apiDo(t, http.MethodPost, "/api/feature", map[string]any{
		"slug": slug, "name": "reviewable", "description": "Reviewable feature",
	}, nil)

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/feature/review", map[string]any{
		"slug": slug, "feature": "reviewable",
	}, nil))

	// Verify the feature still exists after review (review is idempotent).
	var list struct {
		Features []struct {
			Name string `json:"name"`
		} `json:"features"`
	}
	mustOK(t, apiDo(t, http.MethodGet, fmt.Sprintf("/api/features?slug=%s", slug), nil, &list))
	found := false
	for _, f := range list.Features {
		if f.Name == "reviewable" {
			found = true
		}
	}
	if !found {
		t.Error("feature 'reviewable' not found after review")
	}
}
