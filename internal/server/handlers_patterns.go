package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type PatternItem struct {
	ID          int32           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	CodeRefs    json.RawMessage `json:"code_refs"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

type SimilarPatternItem struct {
	Pattern PatternItem `json:"pattern"`
	Score   float64     `json:"score"`
}

func toPatternItem(r db.ZdxPattern) PatternItem {
	refs := json.RawMessage(r.CodeRefs)
	if len(refs) == 0 {
		refs = json.RawMessage("[]")
	}
	return PatternItem{
		ID:          r.ID,
		Name:        r.Name,
		Description: r.Description,
		CodeRefs:    refs,
		CreatedAt:   fmtTS(r.CreatedAt),
		UpdatedAt:   fmtTS(r.UpdatedAt),
	}
}

func (s *Server) registerPatternRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "add-pattern", Method: http.MethodPost, Path: "/api/dx/patterns/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string          `json:"slug"`
				Name        string          `json:"name"`
				Description string          `json:"description"`
				CodeRefs    json.RawMessage `json:"code_refs,omitempty"`
			}
		}) (*struct{ Body PatternItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			refs := in.Body.CodeRefs
			if len(refs) == 0 {
				refs = json.RawMessage("[]")
			}
			row, err := s.q.InsertPattern(ctx, db.InsertPatternParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				CodeRefs:    refs,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go s.emb.upsertPattern(context.Background(), p.ID, row.ID, row.Name+" "+row.Description)
			return &struct{ Body PatternItem }{Body: toPatternItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-pattern", Method: http.MethodPost, Path: "/api/dx/patterns/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string          `json:"slug"`
				ID          int32           `json:"id"`
				Name        string          `json:"name"`
				Description string          `json:"description"`
				CodeRefs    json.RawMessage `json:"code_refs,omitempty"`
			}
		}) (*struct{ Body PatternItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			refs := in.Body.CodeRefs
			if len(refs) == 0 {
				refs = json.RawMessage("[]")
			}
			row, err := s.q.UpdatePattern(ctx, db.UpdatePatternParams{
				ProjectID:   p.ID,
				ID:          in.Body.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				CodeRefs:    refs,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go s.emb.upsertPattern(context.Background(), p.ID, row.ID, row.Name+" "+row.Description)
			return &struct{ Body PatternItem }{Body: toPatternItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-pattern", Method: http.MethodGet, Path: "/api/dx/patterns/get"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   int32  `query:"id" required:"true"`
		}) (*struct{ Body PatternItem }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.GetPattern(ctx, db.GetPatternParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(404, "pattern not found")
			}
			return &struct{ Body PatternItem }{Body: toPatternItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-pattern", Method: http.MethodPost, Path: "/api/dx/patterns/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				ID   int32  `json:"id"`
			}
		}) (*struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.DeletePattern(ctx, db.DeletePatternParams{ProjectID: p.ID, ID: in.Body.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					OK bool `json:"ok"`
				}
			}{Body: struct {
				OK bool `json:"ok"`
			}{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-patterns", Method: http.MethodGet, Path: "/api/dx/patterns"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Patterns []PatternItem `json:"patterns"`
				Total    int64         `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountPatterns(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)

			var rows []db.ZdxPattern
			if in.Search != "" {
				rows, err = s.q.SearchPatterns(ctx, db.SearchPatternsParams{
					ProjectID: p.ID,
					Column2:   pgtype.Text{String: in.Search, Valid: true},
					Limit:     limit,
				})
			} else {
				rows, err = s.q.ListPatternsPaginated(ctx, db.ListPatternsPaginatedParams{
					ProjectID: p.ID,
					Limit:     limit,
					Offset:    offset,
				})
			}
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]PatternItem, len(rows))
			for i, r := range rows {
				out[i] = toPatternItem(r)
			}
			return &struct {
				Body struct {
					Patterns []PatternItem `json:"patterns"`
					Total    int64         `json:"total"`
				}
			}{
				Body: struct {
					Patterns []PatternItem `json:"patterns"`
					Total    int64         `json:"total"`
				}{Patterns: out, Total: total},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "similar-patterns", Method: http.MethodPost, Path: "/api/dx/patterns/similar"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				Text string `json:"text"`
				N    int    `json:"n,omitempty"`
			}
		}) (*struct {
			Body struct {
				Patterns []SimilarPatternItem `json:"patterns"`
			}
		}, error) {
			n := in.Body.N
			if n <= 0 {
				n = 5
			}
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			results, err := s.findSimilarPatterns(ctx, p.ID, in.Body.Text, n)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Patterns []SimilarPatternItem `json:"patterns"`
				}
			}{Body: struct {
				Patterns []SimilarPatternItem `json:"patterns"`
			}{Patterns: results}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reindex-patterns", Method: http.MethodPost, Path: "/api/dx/patterns/reindex"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
			}
		}) (*struct {
			Body struct {
				Indexed int `json:"indexed"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			patterns, err := s.q.ListPatterns(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, pat := range patterns {
				s.emb.upsertPattern(ctx, p.ID, pat.ID, pat.Name+" "+pat.Description)
			}
			return &struct {
				Body struct {
					Indexed int `json:"indexed"`
				}
			}{Body: struct {
				Indexed int `json:"indexed"`
			}{Indexed: len(patterns)}}, nil
		})
}

func (s *Server) findSimilarPatterns(ctx context.Context, projectID int32, text string, n int) ([]SimilarPatternItem, error) {
	results, err := s.emb.topNPatterns(ctx, projectID, text, n)
	if err != nil {
		return nil, err
	}
	out := make([]SimilarPatternItem, 0, len(results))
	for _, r := range results {
		row, err := s.q.GetPattern(ctx, db.GetPatternParams{ProjectID: projectID, ID: int32(r.ID)}) //nolint:gosec
		if err != nil {
			continue
		}
		out = append(out, SimilarPatternItem{
			Pattern: toPatternItem(row),
			Score:   float64(r.Score),
		})
	}
	return out, nil
}
