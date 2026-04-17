package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type EnvironmentItem struct {
	ID                 int32  `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	CurrentBuildSha    string `json:"current_build_sha"`
	CurrentBuildBranch string `json:"current_build_branch"`
	DeployedAt         string `json:"deployed_at"`
	CreatedAt          string `json:"created_at"`
}

type DeployItem struct {
	ID               int32  `json:"id"`
	EnvironmentID    int32  `json:"environment_id"`
	BuildSha         string `json:"build_sha"`
	BuildBranch      string `json:"build_branch"`
	DeployedAt       string `json:"deployed_at"`
	DeployedByUserID int32  `json:"deployed_by_user_id"`
	Status           string `json:"status"`
}

func toEnvironmentItem(e db.ZdxEnvironment) EnvironmentItem {
	return EnvironmentItem{
		ID:                 e.ID,
		Name:               e.Name,
		URL:                e.Url,
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
			items := make([]EnvironmentItem, len(rows))
			for i, r := range rows {
				items[i] = toEnvironmentItem(r)
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
			return &struct{ Body EnvironmentItem }{Body: toEnvironmentItem(env)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-environment", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/environments"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Body struct {
				Name string `json:"name"`
				URL  string `json:"url,omitempty"`
			}
		}) (*struct{ Body EnvironmentItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			env, err := h.Q.CreateEnvironment(ctx, db.CreateEnvironmentParams{
				ProjectID: p.ID,
				Name:      in.Body.Name,
				Url:       in.Body.URL,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body EnvironmentItem }{Body: toEnvironmentItem(env)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-environment", Method: http.MethodPut, Path: "/api/dx/projects/{slug}/environments/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
			Body struct {
				URL string `json:"url,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UpdateEnvironment(ctx, db.UpdateEnvironmentParams{
				Url:       in.Body.URL,
				ProjectID: p.ID,
				Name:      in.Name,
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
			rows, err := h.Q.ListDeploys(ctx, db.ListDeploysParams{
				EnvironmentID: env.ID,
				Limit:         limit,
				Offset:        offset,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			total, _ := h.Q.CountDeploys(ctx, env.ID)
			items := make([]DeployItem, len(rows))
			for i, r := range rows {
				items[i] = toDeployItem(r)
			}
			out := &struct {
				Body struct {
					Items []DeployItem `json:"items"`
					Total int64        `json:"total"`
				}
			}{}
			out.Body.Items = items
			out.Body.Total = total
			return out, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-environment-deploy", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/environments/{name}/deploys"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Name string `path:"name"`
			Body struct {
				BuildSha    string `json:"build_sha"`
				BuildBranch string `json:"build_branch,omitempty"`
				Status      string `json:"status,omitempty"`
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
}
