package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"

	"github.com/iodesystems/zdx-go/internal/db"
)

type stubDeployRequestStore struct {
	mu      sync.Mutex
	project db.ZdxProject
	env     db.GetEnvironmentRow
	issues  map[string]db.ZdxIssue
	rows    []db.CreateDeployRequestParams

	envErr error
}

func (s *stubDeployRequestStore) GetProjectBySlug(_ context.Context, slug string) (db.ZdxProject, error) {
	if s.project.Slug != slug {
		return db.ZdxProject{}, fmt.Errorf("project not found")
	}
	return s.project, nil
}

func (s *stubDeployRequestStore) GetEnvironment(_ context.Context, arg db.GetEnvironmentParams) (db.GetEnvironmentRow, error) {
	if s.envErr != nil {
		return db.GetEnvironmentRow{}, s.envErr
	}
	if arg.ProjectID != s.project.ID || arg.Name != s.env.Name {
		return db.GetEnvironmentRow{}, fmt.Errorf("env not found")
	}
	return s.env, nil
}

func (s *stubDeployRequestStore) GetIssue(_ context.Context, arg db.GetIssueParams) (db.ZdxIssue, error) {
	if iss, ok := s.issues[arg.ID]; ok && iss.ProjectID == arg.ProjectID {
		return iss, nil
	}
	return db.ZdxIssue{}, fmt.Errorf("issue not found")
}

func (s *stubDeployRequestStore) CreateDeployRequest(_ context.Context, arg db.CreateDeployRequestParams) (db.ZdxDeployRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rows = append(s.rows, arg)
	return db.ZdxDeployRequest{
		ID:                int32(len(s.rows)),
		EnvID:             arg.EnvID,
		CommitSha:         arg.CommitSha,
		RequestedByUserID: arg.RequestedByUserID,
		Reason:            arg.Reason,
		BlockingIssueID:   arg.BlockingIssueID,
		Status:            "pending",
	}, nil
}

func newDeployRequestStore() *stubDeployRequestStore {
	return &stubDeployRequestStore{
		project: db.ZdxProject{ID: 1, Slug: "demo"},
		env:     db.GetEnvironmentRow{ID: 42, Name: "prod"},
		issues:  map[string]db.ZdxIssue{},
	}
}

func newDeployRequestAPI(t *testing.T, store DeployRequestStore, ctxMods ...func(context.Context) context.Context) (humatest.TestAPI, context.Context) {
	t.Helper()
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{DeployRequestStore: store}}
	h.registerDeployRequestRoutes(api)
	ctx := context.Background()
	ctx = context.WithValue(ctx, CtxAPIKeyID, int32(7))
	ctx = context.WithValue(ctx, CtxUserID, int32(11))
	ctx = context.WithValue(ctx, CtxUserRole, "user")
	ctx = context.WithValue(ctx, CtxProjectScope, []string{"demo"})
	for _, m := range ctxMods {
		ctx = m(ctx)
	}
	return api, ctx
}

func TestCreateDeployRequest_WorkerToken_201(t *testing.T) {
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store)
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678", "reason": "ship hotfix"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	for _, want := range []string{`"env_slug":"prod"`, `"commit_sha":"abc1234def5678"`, `"status":"pending"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q: %s", want, body)
		}
	}
	if len(store.rows) != 1 {
		t.Fatalf("want 1 persisted row, got %d", len(store.rows))
	}
	if got := store.rows[0].EnvID; got != 42 {
		t.Errorf("env_id = %d, want 42", got)
	}
	if got := store.rows[0].RequestedByUserID; !got.Valid || got.Int32 != 11 {
		t.Errorf("requested_by_user_id = %+v, want user 11", got)
	}
}

func TestCreateDeployRequest_EnvAgentScope_403(t *testing.T) {
	// Until IS-1228 lands first-class scope plumbing, the multi-project /
	// empty-scope shape is what stands in for "non-requester token" — the
	// handler refuses any caller whose project_scope isn't exactly one
	// project. An env-agent token (per-env, project-empty by design)
	// hits this branch and is rejected with 403.
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store, func(ctx context.Context) context.Context {
		return context.WithValue(ctx, CtxProjectScope, []string(nil))
	})
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	if len(store.rows) != 0 {
		t.Errorf("env-agent caller should not persist a row, got %d", len(store.rows))
	}
}

func TestCreateDeployRequest_Viewer_403(t *testing.T) {
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store, func(ctx context.Context) context.Context {
		return context.WithValue(ctx, CtxUserRole, "viewer")
	})
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678"})
	if resp.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
}

func TestCreateDeployRequest_UnknownSlug_404(t *testing.T) {
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store)
	resp := api.PostCtx(ctx, "/api/dx/envs/staging/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678"})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
}

func TestCreateDeployRequest_BadCommitSha_422(t *testing.T) {
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store)
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "nope"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
}

func TestCreateDeployRequest_BlockingIssue_Resolves(t *testing.T) {
	store := newDeployRequestStore()
	store.issues["IS-1234"] = db.ZdxIssue{ID: "IS-1234", ProjectID: 1}
	api, ctx := newDeployRequestAPI(t, store)
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678", "blocking_issue_id": "IS-1234"})
	if resp.Code != http.StatusCreated {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	if got := store.rows[0].BlockingIssueID; !got.Valid || got.String != "IS-1234" {
		t.Errorf("blocking_issue_id = %+v, want IS-1234", got)
	}
}

func TestCreateDeployRequest_BlockingIssue_Unknown_422(t *testing.T) {
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store)
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678", "blocking_issue_id": "IS-9999"})
	if resp.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
}

func TestCreateDeployRequest_Unauthenticated_401(t *testing.T) {
	store := newDeployRequestStore()
	api, ctx := newDeployRequestAPI(t, store, func(ctx context.Context) context.Context {
		return context.WithValue(ctx, CtxAPIKeyID, int32(0))
	})
	resp := api.PostCtx(ctx, "/api/dx/envs/prod/deploy-requests",
		map[string]any{"commit_sha": "abc1234def5678"})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
}
