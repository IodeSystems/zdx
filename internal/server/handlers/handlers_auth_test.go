package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
)

// newAuthAPI registers only the two global routes under test against a humatest
// API. h.Q is left nil — scope-rejection tests never reach the DB; unscoped
// tests merely assert "status != 403" so a panic-recovered 500 is acceptable.
func newAuthAPI(t *testing.T) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{}}
	h.registerAuthRoutes(api)
	return api
}

func scopedCtx(slugs ...string) context.Context {
	return context.WithValue(context.Background(), CtxProjectScope, slugs)
}

func TestGlobalRouteScopeGuard_ListProjects_ScopedToken403(t *testing.T) {
	api := newAuthAPI(t)
	resp := api.GetCtx(scopedCtx("proj-a"), "/api/projects")
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body: %s", resp.Code, resp.Body)
	}
	if !strings.Contains(resp.Body.String(), "scoped tokens cannot list all projects") {
		t.Errorf("body missing scope-block msg: %s", resp.Body)
	}
}

func TestGlobalRouteScopeGuard_CreateProject_ScopedToken403(t *testing.T) {
	api := newAuthAPI(t)
	resp := api.PostCtx(scopedCtx("proj-a"), "/api/project",
		map[string]any{"slug": "new-proj", "name": "New"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body: %s", resp.Code, resp.Body)
	}
	if !strings.Contains(resp.Body.String(), "scoped tokens cannot create projects") {
		t.Errorf("body missing scope-block msg: %s", resp.Body)
	}
}

// callPastGuard invokes fn and reports whether it returned without the guard
// rejecting the call. h.Q is nil in the test API, so once execution flows past
// the scope guard the next h.Q.* call panics with a nil-pointer deref. That
// panic is itself proof the guard let the request through. Returns true if the
// guard *did not* short-circuit.
func callPastGuard(fn func()) (passed bool) {
	defer func() {
		if r := recover(); r != nil {
			passed = true
		}
	}()
	fn()
	return passed
}

func TestGlobalRouteScopeGuard_ListProjects_UnscopedTokenPassesGuard(t *testing.T) {
	api := newAuthAPI(t)
	var resp *httptest.ResponseRecorder
	pastGuard := callPastGuard(func() {
		resp = api.GetCtx(context.Background(), "/api/projects")
	})
	if !pastGuard && resp != nil && resp.Code == http.StatusForbidden {
		t.Fatalf("unscoped ctx should bypass guard; got 403 body=%s", resp.Body)
	}
}

func TestGlobalRouteScopeGuard_CreateProject_UnscopedTokenPassesGuard(t *testing.T) {
	api := newAuthAPI(t)
	var resp *httptest.ResponseRecorder
	pastGuard := callPastGuard(func() {
		resp = api.PostCtx(context.Background(), "/api/project",
			map[string]any{"slug": "new-proj", "name": "New"})
	})
	if !pastGuard && resp != nil && resp.Code == http.StatusForbidden {
		t.Fatalf("unscoped ctx should bypass guard; got 403 body=%s", resp.Body)
	}
}
