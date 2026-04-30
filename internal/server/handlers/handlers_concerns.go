package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

type ConcernItem struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

func toConcernItem(r db.ZdxConcern) ConcernItem {
	return ConcernItem{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   fmtTS(r.CreatedAt),
	}
}

func (h *Handler) registerConcernRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-concerns", Method: http.MethodGet, Path: "/api/dx/concerns"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Concerns []ConcernItem `json:"concerns"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListConcerns(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]ConcernItem, len(rows))
			for i, r := range rows {
				items[i] = toConcernItem(r)
			}
			return &struct {
				Body struct {
					Concerns []ConcernItem `json:"concerns"`
				}
			}{Body: struct {
				Concerns []ConcernItem `json:"concerns"`
			}{Concerns: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-concern", Method: http.MethodGet, Path: "/api/dx/concern"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Name string `query:"name" required:"true"`
		}) (*struct{ Body ConcernItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetConcernByName(ctx, db.GetConcernByNameParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "concern not found: "+in.Name)
			}
			return &struct{ Body ConcernItem }{Body: toConcernItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-concern", Method: http.MethodPost, Path: "/api/dx/concern"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
		}) (*struct{ Body ConcernItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.CreateConcern(ctx, db.CreateConcernParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ConcernItem }{Body: toConcernItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "link-concern", Method: http.MethodPost, Path: "/api/dx/concern/link"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				ConcernName string `json:"concern_name"`
				Feature     string `json:"feature,omitempty"`
				SpecID      *int32 `json:"spec_id,omitempty"`
				PatternID   *int32 `json:"pattern_id,omitempty"`
				Issue       string `json:"issue,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			concern, err := h.Q.GetConcernByName(ctx, db.GetConcernByNameParams{ProjectID: p.ID, Name: in.Body.ConcernName})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "concern not found: "+in.Body.ConcernName)
			}
			if in.Body.Feature != "" {
				f, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
				if err != nil {
					return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Body.Feature)
				}
				if err := h.Q.LinkConcernFeature(ctx, db.LinkConcernFeatureParams{ConcernID: concern.ID, FeatureID: f.ID}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else if in.Body.SpecID != nil {
				if err := h.Q.LinkConcernSpec(ctx, db.LinkConcernSpecParams{ConcernID: concern.ID, SpecID: *in.Body.SpecID}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else if in.Body.PatternID != nil {
				if err := h.Q.LinkConcernPattern(ctx, db.LinkConcernPatternParams{ConcernID: concern.ID, PatternID: *in.Body.PatternID}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else if in.Body.Issue != "" {
				if err := h.Q.LinkConcernIssue(ctx, db.LinkConcernIssueParams{ConcernID: concern.ID, IssueID: in.Body.Issue}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else {
				return nil, apiErr(http.StatusBadRequest, "one of feature, spec_id, pattern_id, or issue must be set")
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "unlink-concern", Method: http.MethodDelete, Path: "/api/dx/concern/link"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				ConcernName string `json:"concern_name"`
				Feature     string `json:"feature,omitempty"`
				SpecID      *int32 `json:"spec_id,omitempty"`
				PatternID   *int32 `json:"pattern_id,omitempty"`
				Issue       string `json:"issue,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			concern, err := h.Q.GetConcernByName(ctx, db.GetConcernByNameParams{ProjectID: p.ID, Name: in.Body.ConcernName})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "concern not found: "+in.Body.ConcernName)
			}
			if in.Body.Feature != "" {
				f, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
				if err != nil {
					return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Body.Feature)
				}
				if err := h.Q.UnlinkConcernFeature(ctx, db.UnlinkConcernFeatureParams{ConcernID: concern.ID, FeatureID: f.ID}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else if in.Body.SpecID != nil {
				if err := h.Q.UnlinkConcernSpec(ctx, db.UnlinkConcernSpecParams{ConcernID: concern.ID, SpecID: *in.Body.SpecID}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else if in.Body.PatternID != nil {
				if err := h.Q.UnlinkConcernPattern(ctx, db.UnlinkConcernPatternParams{ConcernID: concern.ID, PatternID: *in.Body.PatternID}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else if in.Body.Issue != "" {
				if err := h.Q.UnlinkConcernIssue(ctx, db.UnlinkConcernIssueParams{ConcernID: concern.ID, IssueID: in.Body.Issue}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			} else {
				return nil, apiErr(http.StatusBadRequest, "one of feature, spec_id, pattern_id, or issue must be set")
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-concerns-for-feature", Method: http.MethodGet, Path: "/api/dx/concerns/feature"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			Feature string `query:"feature" required:"true"`
		}) (*struct {
			Body struct {
				Concerns []ConcernItem `json:"concerns"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			f, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Feature)
			}
			rows, err := h.Q.ListConcernsForFeature(ctx, f.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]ConcernItem, len(rows))
			for i, r := range rows {
				items[i] = toConcernItem(r)
			}
			return &struct {
				Body struct {
					Concerns []ConcernItem `json:"concerns"`
				}
			}{Body: struct {
				Concerns []ConcernItem `json:"concerns"`
			}{Concerns: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-concerns-for-spec", Method: http.MethodGet, Path: "/api/dx/concerns/spec"},
		func(ctx context.Context, in *struct {
			SpecID int32 `query:"spec_id" required:"true"`
		}) (*struct {
			Body struct {
				Concerns []ConcernItem `json:"concerns"`
			}
		}, error) {
			rows, err := h.Q.ListConcernsForSpec(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]ConcernItem, len(rows))
			for i, r := range rows {
				items[i] = toConcernItem(r)
			}
			return &struct {
				Body struct {
					Concerns []ConcernItem `json:"concerns"`
				}
			}{Body: struct {
				Concerns []ConcernItem `json:"concerns"`
			}{Concerns: items}}, nil
		})
}
