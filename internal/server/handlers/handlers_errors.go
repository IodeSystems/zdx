package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerErrorRoutes(api huma.API) {
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.InsertErrorReport(ctx, db.InsertErrorReportParams{
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			pid := pgtype.Int4{Int32: p.ID, Valid: true}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListErrorReports(pid).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxErrorReport](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ErrorReportItem, len(res.Data))
			for i, r := range res.Data {
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
			}{Errors: out, Total: res.Meta.Pagination.Total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-error", Method: http.MethodGet, Path: "/api/dx/errors/{id}"},
		func(ctx context.Context, in *struct {
			ID   int64  `path:"id"`
			Slug string `query:"slug,omitempty"`
		}) (*struct{ Body ErrorReportItem }, error) {
			row, err := h.Q.GetErrorReportByID(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "error report not found")
			}
			item := ErrorReportItem{
				ID:         row.ID,
				Source:     row.Source,
				Endpoint:   row.Endpoint,
				ErrorName:  row.ErrorName,
				StackTrace: row.StackTrace,
				CreatedAt:  fmtTS(row.CreatedAt),
			}
			if in.Slug != "" {
				if p, pErr := getProject(ctx, h.Q, in.Slug); pErr == nil {
					if linked, lErr := h.Q.GetIssueBySourceErrorID(ctx, db.GetIssueBySourceErrorIDParams{
						ProjectID:     p.ID,
						SourceErrorID: pgtype.Int8{Int64: row.ID, Valid: true},
					}); lErr == nil {
						item.LinkedIssueID = linked.ID
					}
				}
			}
			return &struct{ Body ErrorReportItem }{Body: item}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "trigger-error", Method: http.MethodGet, Path: "/api/error"},
		func(ctx context.Context, in *struct{}) (*struct{}, error) {
			return nil, apiErr(500, "test error — this is intentional")
		})

	huma.Register(api, huma.Operation{OperationID: "clear-errors", Method: http.MethodDelete, Path: "/api/dx/errors"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.DeleteErrorReports(ctx, pgtype.Int4{Int32: p.ID, Valid: true}); err != nil {
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.InsertSlowQuery(ctx, db.InsertSlowQueryParams{
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			pid := pgtype.Int4{Int32: p.ID, Valid: true}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListSlowQueries(pid).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxSlowQuery](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SlowQueryItem, len(res.Data))
			for i, r := range res.Data {
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
			}{Queries: out, Total: res.Meta.Pagination.Total}}, nil
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
				if p, err := getProject(ctx, h.Q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListTimed(db.ListTimedParams{ProjectID: pid, TagFilter: tagFilter}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListTimedRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedItem, len(res.Data))
			for i, r := range res.Data {
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
				}{Items: out, Total: res.Meta.Pagination.Total},
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
				if p, err := getProject(ctx, h.Q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			b := db.WrapListTimedGrouped(db.ListTimedParams{ProjectID: pid, TagFilter: tagFilter}, in.GroupBy)
			res, err := mqpgx.Scan[db.ListTimedGroupedRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedGroupedItem, len(res.Data))
			for i, r := range res.Data {
				var avg int32
				if r.SumCount > 0 {
					avg = int32(r.SumTotalMs / r.SumCount) //nolint:gosec
				}
				out[i] = TimedGroupedItem{
					GroupValue: r.GroupValue, EntryCount: int32(r.EntryCount), //nolint:gosec
					MaxMs: r.MaxMs, SumTotalMs: r.SumTotalMs, SumCount: int32(r.SumCount), AvgMs: avg, //nolint:gosec
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
				if p, err := getProject(ctx, h.Q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := h.Q.ListTimedDistinctTagKeys(ctx, pid)
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
				if p, err := getProject(ctx, h.Q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := h.Q.ListTimedDistinctTagValues(ctx, db.ListTimedDistinctTagValuesParams{ProjectID: pid, TagKey: in.TagKey})
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

	// ── Timed events (per-sample append-only) ────────────────────────────

	type TimedEventItem struct {
		ID          int64           `json:"id"`
		Name        string          `json:"name"`
		DurationMs  int32           `json:"duration_ms"`
		Source      string          `json:"source"`
		Component   string          `json:"component"`
		Environment string          `json:"environment"`
		ContextJson json.RawMessage `json:"context_json"`
		CreatedAt   string          `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-timed-events", Method: http.MethodGet, Path: "/api/dx/timed-events"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			Limit     int32  `query:"limit"`
			Offset    int32  `query:"offset"`
			TagFilter string `query:"tag_filter,omitempty"`
			Since     string `query:"since,omitempty"`
			Until     string `query:"until,omitempty"`
		}) (*struct {
			Body struct {
				Items []TimedEventItem `json:"items"`
				Total int64            `json:"total"`
			}
		}, error) {
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				if p, err := getProject(ctx, h.Q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			since := pgtype.Timestamptz{}
			until := pgtype.Timestamptz{}
			if in.Since != "" {
				if t, err := time.Parse(time.RFC3339, in.Since); err == nil {
					since = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}
			if in.Until != "" {
				if t, err := time.Parse(time.RFC3339, in.Until); err == nil {
					until = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListTimedEvents(db.ListTimedEventsParams{ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxTimedEvent](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedEventItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = TimedEventItem{
					ID: r.ID, Name: r.Name, DurationMs: r.DurationMs,
					Source: r.Source, Component: r.Component, Environment: r.Environment,
					ContextJson: json.RawMessage(r.ContextJson), CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Items []TimedEventItem `json:"items"`
					Total int64            `json:"total"`
				}
			}{
				Body: struct {
					Items []TimedEventItem `json:"items"`
					Total int64            `json:"total"`
				}{Items: out, Total: res.Meta.Pagination.Total},
			}, nil
		})

	type TimedEventGroupedItem struct {
		GroupValue string `json:"group_value"`
		EntryCount int32  `json:"entry_count"`
		MaxMs      int32  `json:"max_ms"`
		SumMs      int64  `json:"sum_ms"`
		FirstSeen  string `json:"first_seen"`
		LastSeen   string `json:"last_seen"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-timed-events-grouped", Method: http.MethodGet, Path: "/api/dx/timed-events/grouped"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			GroupBy   string `query:"group_by"`
			TagFilter string `query:"tag_filter,omitempty"`
			Since     string `query:"since,omitempty"`
			Until     string `query:"until,omitempty"`
		}) (*struct {
			Body struct {
				Items []TimedEventGroupedItem `json:"items"`
			}
		}, error) {
			if in.GroupBy == "" {
				return nil, apiErr(400, "group_by is required")
			}
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				if p, err := getProject(ctx, h.Q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			since := pgtype.Timestamptz{}
			until := pgtype.Timestamptz{}
			if in.Since != "" {
				if t, err := time.Parse(time.RFC3339, in.Since); err == nil {
					since = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}
			if in.Until != "" {
				if t, err := time.Parse(time.RFC3339, in.Until); err == nil {
					until = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}
			b := db.WrapListTimedEventsGrouped(db.ListTimedEventsParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until,
			}, in.GroupBy)
			res, err := mqpgx.Scan[db.ListTimedEventsGroupedRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedEventGroupedItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = TimedEventGroupedItem{
					GroupValue: r.GroupValue, EntryCount: int32(r.EntryCount), //nolint:gosec
					MaxMs: r.MaxMs, SumMs: r.SumMs,
					FirstSeen: fmtTS(r.FirstSeen), LastSeen: fmtTS(r.LastSeen),
				}
			}
			return &struct {
				Body struct {
					Items []TimedEventGroupedItem `json:"items"`
				}
			}{Body: struct {
				Items []TimedEventGroupedItem `json:"items"`
			}{Items: out}}, nil
		})
}
