package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerProjectRoutes(api huma.API) {
	// ── Goals & Constraints ──────────────────────────────────────────────────

	type GoalItem struct {
		ID          int32  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Priority    int32  `json:"priority"`
		Status      string `json:"status"`
		MetricName  string `json:"metric_name"`
		MetricUnit  string `json:"metric_unit"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	type VisionItem struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-goals", Method: http.MethodGet, Path: "/api/goals"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Goals  []GoalItem `json:"goals"`
				Vision VisionItem `json:"vision"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListProjectGoals(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]GoalItem, len(rows))
			for i, r := range rows {
				out[i] = GoalItem{ID: r.ID, Title: r.Title, Description: r.Description, Priority: r.Priority, Status: r.Status, MetricName: r.MetricName, MetricUnit: r.MetricUnit, CreatedAt: fmtTS(r.CreatedAt), UpdatedAt: fmtTS(r.UpdatedAt)}
			}
			return &struct {
				Body struct {
					Goals  []GoalItem `json:"goals"`
					Vision VisionItem `json:"vision"`
				}
			}{Body: struct {
				Goals  []GoalItem `json:"goals"`
				Vision VisionItem `json:"vision"`
			}{Goals: out, Vision: VisionItem{Title: p.Title, Description: p.Description}}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-goal", Method: http.MethodPost, Path: "/api/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Status      string `json:"status"`
				MetricName  string `json:"metric_name" required:"false"`
				MetricUnit  string `json:"metric_unit" required:"false"`
			}
		}) (*struct{ Body GoalItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Body.Status
			if status == "" {
				status = "active"
			}
			row, err := h.Q.CreateProjectGoal(ctx, db.CreateProjectGoalParams{
				ProjectID:   p.ID,
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
				Status:      status,
				MetricName:  in.Body.MetricName,
				MetricUnit:  in.Body.MetricUnit,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body GoalItem }{Body: GoalItem{ID: row.ID, Title: row.Title, Description: row.Description, Priority: row.Priority, Status: row.Status, MetricName: row.MetricName, MetricUnit: row.MetricUnit, CreatedAt: fmtTS(row.CreatedAt), UpdatedAt: fmtTS(row.UpdatedAt)}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-goal", Method: http.MethodPut, Path: "/api/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID          int32  `json:"id"`
				Title       string `json:"title"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Status      string `json:"status"`
				MetricName  string `json:"metric_name" required:"false"`
				MetricUnit  string `json:"metric_unit" required:"false"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.UpdateProjectGoal(ctx, db.UpdateProjectGoalParams{
				ID:          in.Body.ID,
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
				Status:      in.Body.Status,
				MetricName:  in.Body.MetricName,
				MetricUnit:  in.Body.MetricUnit,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-goal", Method: http.MethodDelete, Path: "/api/goal"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.DeleteProjectGoal(ctx, in.Body.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Project vision ──────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "set-project-vision", Method: http.MethodPut, Path: "/api/project-vision"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Title       string `json:"title"`
				Description string `json:"description"`
			}
		}) (*struct {
			Body struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			}
		}, error) {
			if err := h.Q.SetProjectVision(ctx, db.SetProjectVisionParams{
				Title:       in.Body.Title,
				Description: in.Body.Description,
				Slug:        in.Body.Slug,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Title       string `json:"title"`
					Description string `json:"description"`
				}
			}{Body: struct {
				Title       string `json:"title"`
				Description string `json:"description"`
			}{Title: in.Body.Title, Description: in.Body.Description}}, nil
		})

}
