package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/iodesystems/zdx-go/internal/db"
)

type stubAdminTokenStore struct {
	project   db.ZdxProject
	keys      []db.AdminListApiKeysRow
	scopeByID map[int32][]string
	nextID    int32
}

func (s *stubAdminTokenStore) GetProjectBySlug(_ context.Context, slug string) (db.ZdxProject, error) {
	if s.project.Slug != slug {
		return db.ZdxProject{}, fmt.Errorf("not found")
	}
	return s.project, nil
}

func (s *stubAdminTokenStore) CreateApiKey(_ context.Context, arg db.CreateApiKeyParams) (db.CreateApiKeyRow, error) {
	s.nextID++
	row := db.CreateApiKeyRow{
		ID:           s.nextID,
		UserID:       arg.UserID,
		Token:        arg.Token,
		Name:         arg.Name,
		ProjectScope: arg.ProjectScope,
	}
	s.keys = append(s.keys, db.AdminListApiKeysRow{
		ID:           row.ID,
		Name:         row.Name,
		UserEmail:    "admin@test",
		ProjectScope: row.ProjectScope,
	})
	if s.scopeByID == nil {
		s.scopeByID = map[int32][]string{}
	}
	s.scopeByID[row.ID] = arg.ProjectScope
	return row, nil
}

func (s *stubAdminTokenStore) AdminListApiKeys(_ context.Context) ([]db.AdminListApiKeysRow, error) {
	return s.keys, nil
}

func (s *stubAdminTokenStore) AdminDeleteApiKey(_ context.Context, id int32) (int64, error) {
	for i, k := range s.keys {
		if k.ID == id {
			s.keys = append(s.keys[:i], s.keys[i+1:]...)
			delete(s.scopeByID, id)
			return 1, nil
		}
	}
	return 0, nil
}

func (s *stubAdminTokenStore) GetApiKeyProjectScope(_ context.Context, id int32) ([]string, error) {
	return s.scopeByID[id], nil
}

func newAdminTokenAPI(t *testing.T, store AdminTokenStore, callerKeyID int32, callerUID int32) (humatest.TestAPI, context.Context) {
	t.Helper()
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{AdminTokenStore: store}}
	h.registerAdminTokenRoutes(api)
	ctx := context.WithValue(context.Background(), CtxAPIKeyID, callerKeyID)
	ctx = context.WithValue(ctx, CtxUserID, callerUID)
	return api, ctx
}

func newAdminTokenStore() *stubAdminTokenStore {
	return &stubAdminTokenStore{
		project:   db.ZdxProject{ID: 1, Slug: "demo"},
		scopeByID: map[int32][]string{100: nil}, // caller key 100 is admin (NULL scope)
	}
}

func TestCreateAdminToken_Success(t *testing.T) {
	store := newAdminTokenStore()
	api, ctx := newAdminTokenAPI(t, store, 100, 7)
	resp := api.PostCtx(ctx, "/api/admin/tokens",
		map[string]any{"project_slug": "demo", "name": "agent-1"})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	for _, want := range []string{`"name":"agent-1"`, `"project_scope":["demo"]`, `"token":"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q: %s", want, body)
		}
	}
}

func TestCreateAdminToken_RejectsScopedCaller(t *testing.T) {
	store := newAdminTokenStore()
	store.scopeByID[100] = []string{"other"} // caller's key is project-scoped → block
	api, ctx := newAdminTokenAPI(t, store, 100, 7)
	resp := api.PostCtx(ctx, "/api/admin/tokens",
		map[string]any{"project_slug": "demo"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d, body: %s", resp.Code, resp.Body)
	}
	if !strings.Contains(resp.Body.String(), "project_scope must be NULL") {
		t.Errorf("body missing scope-must-be-null msg: %s", resp.Body)
	}
}

func TestCreateAdminToken_UnknownSlug(t *testing.T) {
	store := newAdminTokenStore()
	api, ctx := newAdminTokenAPI(t, store, 100, 7)
	resp := api.PostCtx(ctx, "/api/admin/tokens",
		map[string]any{"project_slug": "ghost"})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d, body: %s", resp.Code, resp.Body)
	}
}

func TestListAndDeleteAdminTokens(t *testing.T) {
	store := newAdminTokenStore()
	api, ctx := newAdminTokenAPI(t, store, 100, 7)

	mint := api.PostCtx(ctx, "/api/admin/tokens",
		map[string]any{"project_slug": "demo", "name": "k1"})
	if mint.Code != http.StatusOK {
		t.Fatalf("mint failed: %d %s", mint.Code, mint.Body)
	}

	list := api.GetCtx(ctx, "/api/admin/tokens")
	if list.Code != http.StatusOK {
		t.Fatalf("list failed: %d %s", list.Code, list.Body)
	}
	body := list.Body.String()
	for _, want := range []string{`"name":"k1"`, `"user_email":"admin@test"`, `"project_scope":["demo"]`} {
		if !strings.Contains(body, want) {
			t.Errorf("list body missing %q: %s", want, body)
		}
	}

	del := api.DeleteCtx(ctx, "/api/admin/tokens/1")
	if del.Code != http.StatusNoContent && del.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", del.Code, del.Body)
	}

	del2 := api.DeleteCtx(ctx, "/api/admin/tokens/1")
	if del2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 on second delete, got %d %s", del2.Code, del2.Body)
	}
}
