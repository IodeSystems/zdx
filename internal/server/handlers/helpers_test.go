package handlers

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

type stubProjectGetter struct {
	projects map[string]db.ZdxProject
	calls    int
}

func (s *stubProjectGetter) GetProjectBySlug(_ context.Context, slug string) (db.ZdxProject, error) {
	s.calls++
	p, ok := s.projects[slug]
	if !ok {
		return db.ZdxProject{}, errors.New("not found")
	}
	return p, nil
}

func TestGetProjectScopeEnforcement_OutOfScope403(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxProjectScope, []string{"proj-a"})
	q := &stubProjectGetter{projects: map[string]db.ZdxProject{
		"proj-b": {ID: 2, Slug: "proj-b"},
	}}

	_, err := getProject(ctx, q, "proj-b")
	if err == nil {
		t.Fatal("expected error for out-of-scope slug")
	}
	var se huma.StatusError
	if !errors.As(err, &se) || se.GetStatus() != http.StatusForbidden {
		t.Fatalf("expected huma 403, got %v", err)
	}
	if q.calls != 0 {
		t.Errorf("scope check should short-circuit before DB; got %d calls", q.calls)
	}
}

func TestGetProjectScopeEnforcement_InScopeSuccess(t *testing.T) {
	ctx := context.WithValue(context.Background(), CtxProjectScope, []string{"proj-a"})
	q := &stubProjectGetter{projects: map[string]db.ZdxProject{
		"proj-a": {ID: 1, Slug: "proj-a"},
	}}

	p, err := getProject(ctx, q, "proj-a")
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if p.ID != 1 || p.Slug != "proj-a" {
		t.Fatalf("unexpected project: %+v", p)
	}
}

func TestGetProjectScopeEnforcement_AdminBypass(t *testing.T) {
	// No CtxProjectScope set → unrestricted (admin token).
	ctx := context.Background()
	q := &stubProjectGetter{projects: map[string]db.ZdxProject{
		"any-slug": {ID: 7, Slug: "any-slug"},
	}}

	p, err := getProject(ctx, q, "any-slug")
	if err != nil {
		t.Fatalf("admin (nil scope) should bypass scope check, got %v", err)
	}
	if p.ID != 7 {
		t.Fatalf("unexpected project: %+v", p)
	}
}

func TestGetProjectScopeEnforcement_InScopeButMissing404(t *testing.T) {
	// Slug is in scope but not in DB — must surface as 404, not 403.
	ctx := context.WithValue(context.Background(), CtxProjectScope, []string{"proj-a"})
	q := &stubProjectGetter{projects: map[string]db.ZdxProject{}}

	_, err := getProject(ctx, q, "proj-a")
	if err == nil {
		t.Fatal("expected error for missing slug")
	}
	var se huma.StatusError
	if !errors.As(err, &se) || se.GetStatus() != http.StatusNotFound {
		t.Fatalf("expected huma 404, got %v", err)
	}
}
