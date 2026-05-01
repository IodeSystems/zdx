package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type VersionBranchItem struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Semver    string `json:"semver,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type VersionBranchDetail struct {
	VersionBranchItem
	OpenCount     int `json:"open_count"`
	ResolvedCount int `json:"resolved_count"`
}

func versionBranchItem(r db.ZdxVersionBranch) VersionBranchItem {
	semver := ""
	if r.Semver.Valid {
		semver = r.Semver.String
	}
	return VersionBranchItem{
		ID:        r.ID,
		Name:      r.Name,
		Type:      r.Type,
		Semver:    semver,
		Status:    r.Status,
		CreatedAt: fmtTS(r.CreatedAt),
	}
}

func (h *Handler) registerBranchRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "create-version-branch", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/branches"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Body struct {
				Name   string `json:"name"`
				Semver string `json:"semver,omitempty"`
			}
		}) (*struct{ Body VersionBranchItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.Name == "" {
				return nil, apiErr(http.StatusUnprocessableEntity, "name is required")
			}
			semver := pgtype.Text{}
			if in.Body.Semver != "" {
				semver = pgtype.Text{String: in.Body.Semver, Valid: true}
			}
			row, err := h.Q.CreateVersionBranch(ctx, db.CreateVersionBranchParams{
				ProjectID: p.ID,
				Name:      in.Body.Name,
				Type:      "named",
				Semver:    semver,
				Status:    "active",
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body VersionBranchItem }{Body: versionBranchItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-version-branches", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/branches"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Branches []VersionBranchItem `json:"branches"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListVersionBranches(ctx, p.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			out := make([]VersionBranchItem, len(rows))
			for i, r := range rows {
				out[i] = versionBranchItem(r)
			}
			return &struct {
				Body struct {
					Branches []VersionBranchItem `json:"branches"`
				}
			}{Body: struct {
				Branches []VersionBranchItem `json:"branches"`
			}{Branches: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "show-version-branch", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/branches/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
		}) (*struct{ Body VersionBranchDetail }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetVersionBranchByName(ctx, db.GetVersionBranchByNameParams{
				ProjectID: p.ID,
				Name:      in.Name,
			})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "branch not found: "+in.Name)
			}
			issues, err := h.Q.ListIssues(ctx, p.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			var openCount, resolvedCount int
			for _, iss := range issues {
				if iss.TargetBranch != in.Name {
					continue
				}
				if iss.Status == "open" || iss.Status == "wip" {
					openCount++
				} else if iss.Status == "closed" {
					resolvedCount++
				}
			}
			detail := VersionBranchDetail{
				VersionBranchItem: versionBranchItem(row),
				OpenCount:         openCount,
				ResolvedCount:     resolvedCount,
			}
			return &struct{ Body VersionBranchDetail }{Body: detail}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-version-branch-eol", Method: http.MethodPatch, Path: "/api/dx/projects/{slug}/branches/{name}/eol"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.MarkVersionBranchEOL(ctx, db.MarkVersionBranchEOLParams{
				ProjectID: p.ID,
				Name:      in.Name,
			}); err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}
