package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/db"
)

type EnvironmentItem struct {
	ID                 int32  `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	ReleaseBranch      string `json:"release_branch"`
	CurrentBuildSha    string `json:"current_build_sha"`
	CurrentBuildBranch string `json:"current_build_branch"`
	DeployedAt         string `json:"deployed_at"`
	CreatedAt          string `json:"created_at"`
	DriftCount         int    `json:"drift_count"`
	DriftOldestAgeSecs int64  `json:"drift_oldest_age_secs"`
}

type DeployItem struct {
	ID               int32  `json:"id"`
	EnvironmentID    int32  `json:"environment_id"`
	BuildSha         string `json:"build_sha"`
	BuildBranch      string `json:"build_branch"`
	DeployedAt       string `json:"deployed_at"`
	DeployedByUserID int32  `json:"deployed_by_user_id"`
	Status           string `json:"status"`
	DurationSecs     int32  `json:"duration_secs"`
	Log              string `json:"log"`
}

func toEnvironmentItemFromList(e db.ListEnvironmentsRow) EnvironmentItem {
	return EnvironmentItem{
		ID:                 e.ID,
		Name:               e.Name,
		URL:                e.Url,
		ReleaseBranch:      e.ReleaseBranch,
		CurrentBuildSha:    e.CurrentBuildSha,
		CurrentBuildBranch: e.CurrentBuildBranch,
		DeployedAt:         fmtTS(e.DeployedAt),
		CreatedAt:          fmtTS(e.CreatedAt),
	}
}

func toEnvironmentItemFromGet(e db.GetEnvironmentRow) EnvironmentItem {
	return EnvironmentItem{
		ID:                 e.ID,
		Name:               e.Name,
		URL:                e.Url,
		ReleaseBranch:      e.ReleaseBranch,
		CurrentBuildSha:    e.CurrentBuildSha,
		CurrentBuildBranch: e.CurrentBuildBranch,
		DeployedAt:         fmtTS(e.DeployedAt),
		CreatedAt:          fmtTS(e.CreatedAt),
	}
}

func toEnvironmentItemFromCreate(e db.CreateEnvironmentRow) EnvironmentItem {
	return EnvironmentItem{
		ID:                 e.ID,
		Name:               e.Name,
		URL:                e.Url,
		ReleaseBranch:      e.ReleaseBranch,
		CurrentBuildSha:    e.CurrentBuildSha,
		CurrentBuildBranch: e.CurrentBuildBranch,
		DeployedAt:         fmtTS(e.DeployedAt),
		CreatedAt:          fmtTS(e.CreatedAt),
	}
}

func toDeployItem(d db.ZdxDeploy) DeployItem {
	return DeployItem{
		ID:               d.ID,
		EnvironmentID:    d.EnvironmentID,
		BuildSha:         d.BuildSha,
		BuildBranch:      d.BuildBranch,
		DeployedAt:       fmtTS(d.DeployedAt),
		DeployedByUserID: d.DeployedByUserID.Int32,
		Status:           d.Status,
		DurationSecs:     d.DurationSecs,
		Log:              d.Log,
	}
}

func (h *Handler) registerEnvironmentRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-environments", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/environments"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
		}) (*struct {
			Body struct {
				Items []EnvironmentItem `json:"items"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListEnvironments(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			bareDir := filepath.Join(h.ReposDir, in.Slug+".git")
			items := make([]EnvironmentItem, len(rows))
			for i, r := range rows {
				item := toEnvironmentItemFromList(r)
				if r.ReleaseBranch != "" {
					count, oldest, _ := CommitDrift(bareDir, r.ReleaseBranch, "dev")
					item.DriftCount = count
					item.DriftOldestAgeSecs = oldest
				}
				items[i] = item
			}
			out := &struct {
				Body struct {
					Items []EnvironmentItem `json:"items"`
				}
			}{}
			out.Body.Items = items
			return out, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-environment", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/environments/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
		}) (*struct{ Body EnvironmentItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			env, err := h.Q.GetEnvironment(ctx, db.GetEnvironmentParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "environment not found: "+in.Name)
			}
			return &struct{ Body EnvironmentItem }{Body: toEnvironmentItemFromGet(env)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-environment", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/environments"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Body struct {
				Name          string `json:"name"`
				URL           string `json:"url,omitempty"`
				ReleaseBranch string `json:"release_branch,omitempty"`
			}
		}) (*struct{ Body EnvironmentItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			env, err := h.Q.CreateEnvironment(ctx, db.CreateEnvironmentParams{
				ProjectID:     p.ID,
				Name:          in.Body.Name,
				Url:           in.Body.URL,
				ReleaseBranch: in.Body.ReleaseBranch,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body EnvironmentItem }{Body: toEnvironmentItemFromCreate(env)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-environment", Method: http.MethodPut, Path: "/api/dx/projects/{slug}/environments/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
			Body struct {
				URL           string `json:"url,omitempty"`
				ReleaseBranch string `json:"release_branch,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UpdateEnvironment(ctx, db.UpdateEnvironmentParams{
				Url:           in.Body.URL,
				ReleaseBranch: in.Body.ReleaseBranch,
				ProjectID:     p.ID,
				Name:          in.Name,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-environment", Method: http.MethodDelete, Path: "/api/dx/projects/{slug}/environments/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.DeleteEnvironment(ctx, db.DeleteEnvironmentParams{ProjectID: p.ID, Name: in.Name}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-environment-deploys", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/environments/{name}/deploys"},
		func(ctx context.Context, in *struct {
			Slug   string `path:"slug"`
			Name   string `path:"name"`
			Limit  int32  `query:"limit"`
			Offset int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Items []DeployItem `json:"items"`
				Total int64        `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			env, err := h.Q.GetEnvironment(ctx, db.GetEnvironmentParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "environment not found: "+in.Name)
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListDeploys(env.ID).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxDeploy](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]DeployItem, len(res.Data))
			for i, r := range res.Data {
				items[i] = toDeployItem(r)
			}
			out := &struct {
				Body struct {
					Items []DeployItem `json:"items"`
					Total int64        `json:"total"`
				}
			}{}
			out.Body.Items = items
			out.Body.Total = res.Meta.Pagination.Total
			return out, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-environment-deploy", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/environments/{name}/deploys"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
			Body struct {
				BuildSha     string `json:"build_sha"`
				BuildBranch  string `json:"build_branch,omitempty"`
				Status       string `json:"status,omitempty"`
				DurationSecs int32  `json:"duration_secs,omitempty"`
				Log          string `json:"log,omitempty"`
			}
		}) (*struct{ Body DeployItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			env, err := h.Q.GetEnvironment(ctx, db.GetEnvironmentParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "environment not found: "+in.Name)
			}
			status := in.Body.Status
			if status == "" {
				status = "success"
			}
			deploy, err := h.Q.CreateDeploy(ctx, db.CreateDeployParams{
				EnvironmentID: env.ID,
				BuildSha:      in.Body.BuildSha,
				BuildBranch:   in.Body.BuildBranch,
				Status:        status,
				DurationSecs:  in.Body.DurationSecs,
				Log:           in.Body.Log,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := h.Q.UpdateEnvironmentDeploy(ctx, db.UpdateEnvironmentDeployParams{
				ProjectID:          p.ID,
				Name:               in.Name,
				CurrentBuildSha:    in.Body.BuildSha,
				CurrentBuildBranch: in.Body.BuildBranch,
				DeployedByUserID:   pgtype.Int4{},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body DeployItem }{Body: toDeployItem(deploy)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "request-environment-todo", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/environments/{name}/request"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
			Body struct {
				Kind string `json:"kind"`
			}
		}) (*struct{ Body TodoItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			env, err := h.Q.GetEnvironment(ctx, db.GetEnvironmentParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "environment not found: "+in.Name)
			}
			kind := in.Body.Kind
			if kind != "test" && kind != "ship" && kind != "sync" {
				return nil, apiErr(http.StatusBadRequest, "kind must be 'test', 'ship', or 'sync'")
			}
			key := fmt.Sprintf("env:%s:%s:%d", in.Name, kind, time.Now().UnixMilli())
			var title, text string
			switch kind {
			case "test":
				title = fmt.Sprintf("Test %s", in.Name)
				text = fmt.Sprintf("Run tests against the %s environment (SHA: %s, branch: %s)", in.Name, env.CurrentBuildSha, env.CurrentBuildBranch)
			case "ship":
				if env.ReleaseBranch != "" {
					title = fmt.Sprintf("Ship to %s from %s", in.Name, env.ReleaseBranch)
					text = fmt.Sprintf(
						"Deploy to the %s environment from release branch %s:\n"+
							"1. Ensure all tests pass on the main branch\n"+
							"2. Merge main into %s (fast-forward preferred)\n"+
							"3. Build from %s\n"+
							"4. Deploy the build to %s\n"+
							"5. Record the deploy: dx env deploy %s --branch=%s\n"+
							"\nCurrent deployed: %s @ %s",
						in.Name, env.ReleaseBranch,
						env.ReleaseBranch,
						env.ReleaseBranch,
						in.Name,
						in.Name, env.ReleaseBranch,
						env.CurrentBuildSha, env.CurrentBuildBranch,
					)
				} else {
					title = fmt.Sprintf("Ship to %s", in.Name)
					text = fmt.Sprintf(
						"Deploy to the %s environment:\n"+
							"1. Build from the current main branch\n"+
							"2. Deploy the build\n"+
							"3. Record the deploy: dx env deploy %s\n"+
							"\nCurrent deployed: %s @ %s",
						in.Name, in.Name,
						env.CurrentBuildSha, env.CurrentBuildBranch,
					)
				}
			case "sync":
				if env.ReleaseBranch == "" {
					return nil, apiErr(http.StatusBadRequest, "environment has no release_branch configured — set one before syncing")
				}
				title = fmt.Sprintf("Sync %s → %s", "main", env.ReleaseBranch)
				text = fmt.Sprintf(
					"Sync main into the %s release branch (%s) — only if all tests pass on main:\n"+
						"1. Verify all tests are passing on the main branch (check dx test results or run tests)\n"+
						"2. If tests pass: git fetch origin && git checkout %s && git merge --ff-only origin/main\n"+
						"3. Push the updated release branch: git push origin %s\n"+
						"4. If fast-forward fails (diverged), create an issue describing the conflict\n"+
						"\nCurrent deployed: %s @ %s\n"+
						"This sync makes the release branch eligible for the next ship.",
					in.Name, env.ReleaseBranch,
					env.ReleaseBranch,
					env.ReleaseBranch,
					env.CurrentBuildSha, env.CurrentBuildBranch,
				)
			}
			todo, err := h.Q.CreateTodo(ctx, db.CreateTodoParams{
				ProjectID:  p.ID,
				Key:        key,
				Title:      title,
				Text:       text,
				Kind:       kind,
				TargetType: "environment",
				TargetID:   in.Name,
				Priority:   2,
				Status:     "open",
				Persona:    "owner",
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := TodoItem{
				ID:         todo.ID,
				Text:       todo.Text,
				Title:      todo.Title,
				Key:        todo.Key,
				Kind:       todo.Kind,
				TargetType: todo.TargetType,
				TargetID:   todo.TargetID,
				Priority:   todo.Priority,
				Status:     todo.Status,
				CreatedAt:  fmtTS(todo.CreatedAt),
			}
			return &struct{ Body TodoItem }{Body: item}, nil
		})
}

// maybeQueueReleaseSyncs creates sync todos for all environments with a release_branch
// when a batch of test results on the main branch are all passing. Called in a goroutine.
func (h *Handler) maybeQueueReleaseSyncs(ctx context.Context, projectID int32, results []TestResultInput) {
	if len(results) == 0 {
		return
	}
	branch := results[0].Branch
	if branch != "main" && branch != "master" {
		return
	}
	for _, r := range results {
		if r.Status != "pass" && r.Status != "passed" {
			return // any failure → no sync
		}
	}
	envs, err := h.Q.ListEnvironments(ctx, projectID)
	if err != nil {
		return
	}
	sha := results[0].GitSHA
	for _, env := range envs {
		if env.ReleaseBranch == "" {
			return
		}
		key := fmt.Sprintf("env-sync:%s:%s", env.Name, sha)
		title := fmt.Sprintf("Sync %s → %s", branch, env.ReleaseBranch)
		text := fmt.Sprintf(
			"All tests passed on %s (SHA: %s). Sync into the %s release branch (%s):\n"+
				"1. git fetch origin && git checkout %s && git merge --ff-only origin/%s\n"+
				"2. Push: git push origin %s\n"+
				"3. If fast-forward fails (diverged), file an issue describing the conflict.\n"+
				"\nThis was auto-created by test-gated sync after a green main build.",
			branch, sha, env.Name, env.ReleaseBranch,
			env.ReleaseBranch, branch,
			env.ReleaseBranch,
		)
		_, _ = h.Q.CreateTodo(ctx, db.CreateTodoParams{
			ProjectID:  projectID,
			Key:        key,
			Title:      title,
			Text:       text,
			Kind:       "sync",
			TargetType: "environment",
			TargetID:   env.Name,
			Priority:   3,
			Status:     "open",
			Persona:    "agent",
		})
	}
}
