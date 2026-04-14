package server

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerErrorRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "report-error", Method: http.MethodPost, Path: "/api/dx/errors/report"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				Source     string `json:"source"`
				Endpoint   string `json:"endpoint"`
				ErrorName  string `json:"error_name"`
				StackTrace string `json:"stack_trace"`
			}
		}) (*struct{ Body ErrorReportItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.InsertErrorReport(ctx, db.InsertErrorReportParams{
				ProjectID:  pgtype.Int4{Int32: p.ID, Valid: true},
				Source:     in.Body.Source,
				Endpoint:   in.Body.Endpoint,
				ErrorName:  in.Body.ErrorName,
				StackTrace: in.Body.StackTrace,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ErrorReportItem }{Body: ErrorReportItem{
				ID:         row.ID,
				Source:     row.Source,
				Endpoint:   row.Endpoint,
				ErrorName:  row.ErrorName,
				StackTrace: row.StackTrace,
				CreatedAt:  fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-errors", Method: http.MethodGet, Path: "/api/dx/errors"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Errors []ErrorReportItem `json:"errors"`
				Total  int64             `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			pid := pgtype.Int4{Int32: p.ID, Valid: true}
			total, _ := s.q.CountErrorReports(ctx, pid)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListErrorReportsPaginated(ctx, db.ListErrorReportsPaginatedParams{ProjectID: pid, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ErrorReportItem, len(rows))
			for i, r := range rows {
				out[i] = ErrorReportItem{
					ID:         r.ID,
					Source:     r.Source,
					Endpoint:   r.Endpoint,
					ErrorName:  r.ErrorName,
					StackTrace: r.StackTrace,
					CreatedAt:  fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Errors []ErrorReportItem `json:"errors"`
					Total  int64             `json:"total"`
				}
			}{Body: struct {
				Errors []ErrorReportItem `json:"errors"`
				Total  int64             `json:"total"`
			}{Errors: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "trigger-error", Method: http.MethodGet, Path: "/api/error"},
		func(ctx context.Context, in *struct{}) (*struct{}, error) {
			return nil, apiErr(500, "test error — this is intentional")
		})

	huma.Register(api, huma.Operation{OperationID: "clear-errors", Method: http.MethodDelete, Path: "/api/dx/errors"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.DeleteErrorReports(ctx, pgtype.Int4{Int32: p.ID, Valid: true}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return nil, nil
		})

	huma.Register(api, huma.Operation{OperationID: "report-slow-query", Method: http.MethodPost, Path: "/api/dx/slow-queries/report"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				SqlHash     string `json:"sql_hash"`
				SqlText     string `json:"sql_text"`
				Endpoint    string `json:"endpoint"`
				DurationMs  int32  `json:"duration_ms"`
				ExplainJson string `json:"explain_json"`
			}
		}) (*struct{ Body SlowQueryItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.InsertSlowQuery(ctx, db.InsertSlowQueryParams{
				ProjectID:   pgtype.Int4{Int32: p.ID, Valid: true},
				SqlHash:     in.Body.SqlHash,
				SqlText:     in.Body.SqlText,
				Endpoint:    in.Body.Endpoint,
				DurationMs:  in.Body.DurationMs,
				ExplainJson: in.Body.ExplainJson,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body SlowQueryItem }{Body: SlowQueryItem{
				ID:          row.ID,
				SqlHash:     row.SqlHash,
				SqlText:     row.SqlText,
				Endpoint:    row.Endpoint,
				DurationMs:  row.DurationMs,
				ExplainJson: row.ExplainJson,
				CreatedAt:   fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-slow-queries", Method: http.MethodGet, Path: "/api/dx/slow-queries"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Queries []SlowQueryItem `json:"queries"`
				Total   int64           `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			pid := pgtype.Int4{Int32: p.ID, Valid: true}
			total, _ := s.q.CountSlowQueries(ctx, pid)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListSlowQueriesPaginated(ctx, db.ListSlowQueriesPaginatedParams{ProjectID: pid, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SlowQueryItem, len(rows))
			for i, r := range rows {
				out[i] = SlowQueryItem{
					ID:          r.ID,
					SqlHash:     r.SqlHash,
					SqlText:     r.SqlText,
					Endpoint:    r.Endpoint,
					DurationMs:  r.DurationMs,
					ExplainJson: r.ExplainJson,
					CreatedAt:   fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Queries []SlowQueryItem `json:"queries"`
					Total   int64           `json:"total"`
				}
			}{Body: struct {
				Queries []SlowQueryItem `json:"queries"`
				Total   int64           `json:"total"`
			}{Queries: out, Total: total}}, nil
		})

	// ── Timed ─────────────────────────────────────────────────────────────────

	type TimedItem struct {
		ID          int64           `json:"id"`
		Name        string          `json:"name"`
		DurationMs  int32           `json:"duration_ms"`
		Count       int32           `json:"count"`
		TotalMs     int64           `json:"total_ms"`
		AvgMs       int32           `json:"avg_ms"`
		Source      string          `json:"source"`
		ContextJson json.RawMessage `json:"context_json"`
		CreatedAt   string          `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-timed", Method: http.MethodGet, Path: "/api/dx/timed"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			Limit     int32  `query:"limit"`
			Offset    int32  `query:"offset"`
			TagFilter string `query:"tag_filter,omitempty"`
		}) (*struct {
			Body struct {
				Items []TimedItem `json:"items"`
				Total int64       `json:"total"`
			}
		}, error) {
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			total, _ := s.q.CountTimed(ctx, db.CountTimedParams{ProjectID: pid, TagFilter: tagFilter})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTimedPaginated(ctx, db.ListTimedPaginatedParams{ProjectID: pid, TagFilter: tagFilter, Lim: limit, Off: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedItem, len(rows))
			for i, r := range rows {
				var avg int32
				if r.Count > 0 {
					avg = int32(r.TotalMs / int64(r.Count)) //nolint:gosec
				}
				out[i] = TimedItem{
					ID: r.ID, Name: r.Name, DurationMs: r.DurationMs,
					Count: r.Count, TotalMs: r.TotalMs, AvgMs: avg,
					Source: r.Source, ContextJson: json.RawMessage(r.ContextJson), CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Items []TimedItem `json:"items"`
					Total int64       `json:"total"`
				}
			}{
				Body: struct {
					Items []TimedItem `json:"items"`
					Total int64       `json:"total"`
				}{Items: out, Total: total},
			}, nil
		})

	type TimedGroupedItem struct {
		GroupValue string `json:"group_value"`
		EntryCount int32  `json:"entry_count"`
		MaxMs      int32  `json:"max_ms"`
		SumTotalMs int64  `json:"sum_total_ms"`
		SumCount   int32  `json:"sum_count"`
		AvgMs      int32  `json:"avg_ms"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-timed-grouped", Method: http.MethodGet, Path: "/api/dx/timed/grouped"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			GroupBy   string `query:"group_by"`
			TagFilter string `query:"tag_filter,omitempty"`
		}) (*struct {
			Body struct {
				Items []TimedGroupedItem `json:"items"`
			}
		}, error) {
			if in.GroupBy == "" {
				return nil, apiErr(400, "group_by is required")
			}
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			rows, err := s.q.ListTimedGrouped(ctx, db.ListTimedGroupedParams{ProjectID: pid, TagFilter: tagFilter, GroupKey: in.GroupBy})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedGroupedItem, len(rows))
			for i, r := range rows {
				gv, _ := r.GroupValue.(string)
				maxMs, _ := r.MaxMs.(int32)
				var avg int32
				if r.SumCount > 0 {
					avg = int32(r.SumTotalMs / int64(r.SumCount)) //nolint:gosec
				}
				out[i] = TimedGroupedItem{
					GroupValue: gv, EntryCount: r.EntryCount,
					MaxMs: maxMs, SumTotalMs: r.SumTotalMs, SumCount: r.SumCount, AvgMs: avg,
				}
			}
			return &struct {
				Body struct {
					Items []TimedGroupedItem `json:"items"`
				}
			}{Body: struct {
				Items []TimedGroupedItem `json:"items"`
			}{Items: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-timed-tag-keys", Method: http.MethodGet, Path: "/api/dx/timed/tags/keys"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug,omitempty"`
		}) (*struct {
			Body struct {
				Keys []string `json:"keys"`
			}
		}, error) {
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := s.q.ListTimedDistinctTagKeys(ctx, pid)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			keys := make([]string, 0, len(rows))
			for _, r := range rows {
				if r.Valid {
					keys = append(keys, r.String)
				}
			}
			return &struct {
				Body struct {
					Keys []string `json:"keys"`
				}
			}{Body: struct {
				Keys []string `json:"keys"`
			}{Keys: keys}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-timed-tag-values", Method: http.MethodGet, Path: "/api/dx/timed/tags/values"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug,omitempty"`
			TagKey string `query:"key"`
		}) (*struct {
			Body struct {
				Values []string `json:"values"`
			}
		}, error) {
			if in.TagKey == "" {
				return nil, apiErr(400, "key is required")
			}
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := s.q.ListTimedDistinctTagValues(ctx, db.ListTimedDistinctTagValuesParams{ProjectID: pid, TagKey: in.TagKey})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			values := make([]string, 0, len(rows))
			for _, r := range rows {
				if v, ok := r.(string); ok {
					values = append(values, v)
				}
			}
			return &struct {
				Body struct {
					Values []string `json:"values"`
				}
			}{Body: struct {
				Values []string `json:"values"`
			}{Values: values}}, nil
		})
}
