package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

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
	Source  string      `json:"source,omitempty"`
	Via     string      `json:"via,omitempty"`
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

func (h *Handler) registerPatternRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "add-pattern", Method: http.MethodPost, Path: "/api/dx/patterns/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string          `json:"slug"`
				Name        string          `json:"name"`
				Description string          `json:"description"`
				CodeRefs    json.RawMessage `json:"code_refs,omitempty"`
			}
		}) (*struct{ Body PatternItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			refs := in.Body.CodeRefs
			if len(refs) == 0 {
				refs = json.RawMessage("[]")
			}
			row, err := h.Q.InsertPattern(ctx, db.InsertPatternParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				CodeRefs:    refs,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertPattern(context.Background(), p.ID, row.ID, row.Name+" "+row.Description)
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			refs := in.Body.CodeRefs
			if len(refs) == 0 {
				refs = json.RawMessage("[]")
			}
			row, err := h.Q.UpdatePattern(ctx, db.UpdatePatternParams{
				ProjectID:   p.ID,
				ID:          in.Body.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				CodeRefs:    refs,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertPattern(context.Background(), p.ID, row.ID, row.Name+" "+row.Description)
			return &struct{ Body PatternItem }{Body: toPatternItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-pattern", Method: http.MethodGet, Path: "/api/dx/patterns/get"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   int32  `query:"id" required:"true"`
		}) (*struct{ Body PatternItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetPattern(ctx, db.GetPatternParams{ProjectID: p.ID, ID: in.ID})
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.DeletePattern(ctx, db.DeletePatternParams{ProjectID: p.ID, ID: in.Body.ID}); err != nil {
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)

			var out []PatternItem
			var total int64
			if in.Search != "" {
				rows, err := h.Q.SearchPatterns(ctx, db.SearchPatternsParams{
					ProjectID: p.ID,
					Column2:   pgtype.Text{String: in.Search, Valid: true},
					Limit:     limit,
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				total = int64(len(rows))
				out = make([]PatternItem, len(rows))
				for i, r := range rows {
					out[i] = toPatternItem(r)
				}
			} else {
				b := db.WrapListPatterns(p.ID).
					ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
				res, err := mqpgx.Scan[db.ZdxPattern](ctx, h.Pool, b)
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				total = res.Meta.Pagination.Total
				out = make([]PatternItem, len(res.Data))
				for i, r := range res.Data {
					out[i] = toPatternItem(r)
				}
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
				Slug       string `json:"slug"`
				Text       string `json:"text"`
				N          int    `json:"n,omitempty"`
				TargetType string `json:"target_type,omitempty"`
				TargetId   string `json:"target_id,omitempty"`
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			// Resolve concern-linked patterns for the target.
			var concernLinked []SimilarPatternItem
			if in.Body.TargetType != "" && in.Body.TargetId != "" {
				concernLinked, _ = h.findConcernLinkedPatterns(ctx, in.Body.TargetType, in.Body.TargetId)
			}

			// Similarity search; skip patterns already concern-linked.
			seenIDs := map[int32]bool{}
			for _, r := range concernLinked {
				seenIDs[r.Pattern.ID] = true
			}
			similar, err := h.findSimilarPatterns(ctx, p.ID, in.Body.Text, n)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			var filtered []SimilarPatternItem
			for _, r := range similar {
				if !seenIDs[r.Pattern.ID] {
					filtered = append(filtered, r)
				}
			}

			results := append(concernLinked, filtered...)
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			patterns, err := h.Q.ListPatterns(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, pat := range patterns {
				h.Emb.UpsertPattern(ctx, p.ID, pat.ID, pat.Name+" "+pat.Description)
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

func (h *Handler) findSimilarPatterns(ctx context.Context, projectID int32, text string, n int) ([]SimilarPatternItem, error) {
	results, err := h.Emb.TopNPatterns(ctx, projectID, text, n)
	if err != nil {
		return nil, err
	}
	out := make([]SimilarPatternItem, 0, len(results))
	for _, r := range results {
		row, err := h.Q.GetPattern(ctx, db.GetPatternParams{ProjectID: projectID, ID: int32(r.ID)}) //nolint:gosec
		if err != nil {
			continue
		}
		out = append(out, SimilarPatternItem{
			Pattern: toPatternItem(row),
			Score:   float64(r.Score),
			Source:  "similarity",
		})
	}
	return out, nil
}

func (h *Handler) findConcernLinkedPatterns(ctx context.Context, targetType, targetID string) ([]SimilarPatternItem, error) {
	var concerns []db.ZdxConcern
	switch targetType {
	case "issue":
		var err error
		concerns, err = h.Q.ListConcernsForIssue(ctx, targetID)
		if err != nil {
			return nil, err
		}
	case "task":
		taskRow, err := h.Q.GetTask(ctx, targetID)
		if err != nil {
			return nil, err
		}
		if taskRow.Spec != "" {
			specID, err := strconv.ParseInt(taskRow.Spec, 10, 32)
			if err == nil {
				concerns, err = h.Q.ListConcernsForSpec(ctx, int32(specID)) //nolint:gosec
				if err != nil {
					return nil, err
				}
			}
		}
	}

	seen := map[int32]bool{}
	var out []SimilarPatternItem
	for _, c := range concerns {
		patterns, err := h.Q.GetPatternsForConcern(ctx, c.ID)
		if err != nil {
			continue
		}
		for _, p := range patterns {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			out = append(out, SimilarPatternItem{
				Pattern: toPatternItem(p),
				Score:   0,
				Source:  "concern",
				Via:     c.Name,
			})
		}
	}
	return out, nil
}
