package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerThemeRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-themes", Method: http.MethodGet, Path: "/api/dx/themes"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Themes []ThemeItem `json:"themes"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListThemes(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ThemeItem, len(rows))
			for i, r := range rows {
				blockers, _ := r.Blockers.(string)
				out[i] = ThemeItem{
					ID:          r.ID,
					Name:        r.Name,
					Description: r.Description,
					Priority:    r.Priority,
					Status:      r.Status,
					Blockers:    blockers,
					CreatedAt:   fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Themes []ThemeItem `json:"themes"`
				}
			}{Body: struct {
				Themes []ThemeItem `json:"themes"`
			}{Themes: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-theme", Method: http.MethodPost, Path: "/api/dx/themes/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Blockers    string `json:"blockers"`
			}
		}) (*struct{ Body ThemeItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.CreateTheme(ctx, db.CreateThemeParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ThemeItem }{Body: ThemeItem{
				ID:          row.ID,
				Name:        row.Name,
				Description: row.Description,
				Priority:    row.Priority,
				Status:      row.Status,
				CreatedAt:   fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-theme-status", Method: http.MethodPost, Path: "/api/dx/themes/status"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				Theme  string `json:"theme"` // "TH-N" or name
				Status string `json:"status"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			theme, err := s.resolveTheme(ctx, p.ID, in.Body.Theme)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateThemeStatus(ctx, db.UpdateThemeStatusParams{
				ProjectID: p.ID,
				ID:        theme.ID,
				Status:    in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-theme-blocker", Method: http.MethodPost, Path: "/api/dx/themes/block"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Theme string `json:"theme"`
				Issue string `json:"issue"` // "IS-N"
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			theme, err := s.resolveTheme(ctx, p.ID, in.Body.Theme)
			if err != nil {
				return nil, err
			}
			if err := s.q.AddThemeBlocker(ctx, db.AddThemeBlockerParams{
				ThemeID: theme.ID,
				IssueID: in.Body.Issue,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "remove-theme-blocker", Method: http.MethodPost, Path: "/api/dx/themes/unblock"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Theme string `json:"theme"`
				Issue string `json:"issue"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			theme, err := s.resolveTheme(ctx, p.ID, in.Body.Theme)
			if err != nil {
				return nil, err
			}
			if err := s.q.RemoveThemeBlocker(ctx, db.RemoveThemeBlockerParams{
				ThemeID: theme.ID,
				IssueID: in.Body.Issue,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}

// ── Internal helper ───────────────────────────────────────────────────────

func (s *Server) resolveTheme(ctx context.Context, projectID int32, ref string) (db.ZdxTheme, error) {
	// "TH-N" → integer lookup
	if strings.HasPrefix(ref, "TH-") {
		id := intFromPrefixed(ref, "TH-")
		t, err := s.q.GetThemeByID(ctx, db.GetThemeByIDParams{ProjectID: projectID, ID: id})
		if err != nil {
			return db.ZdxTheme{}, apiErr(http.StatusNotFound, "theme not found: "+ref)
		}
		return t, nil
	}
	t, err := s.q.GetThemeByName(ctx, db.GetThemeByNameParams{ProjectID: projectID, Name: ref})
	if err != nil {
		return db.ZdxTheme{}, apiErr(http.StatusNotFound, "theme not found: "+ref)
	}
	return t, nil
}
