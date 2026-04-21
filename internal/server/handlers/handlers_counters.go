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

func (h *Handler) registerCounterRoutes(api huma.API) {
	type CountedItem struct {
		ID          int64           `json:"id"`
		Name        string          `json:"name"`
		Value       int32           `json:"value"`
		Count       int32           `json:"count"`
		TotalValue  int64           `json:"total_value"`
		AvgValue    int32           `json:"avg_value"`
		Source      string          `json:"source"`
		ContextJson json.RawMessage `json:"context_json"`
		CreatedAt   string          `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-counted", Method: http.MethodGet, Path: "/api/dx/counted"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			Limit     int32  `query:"limit"`
			Offset    int32  `query:"offset"`
			TagFilter string `query:"tag_filter,omitempty"`
		}) (*struct {
			Body struct {
				Items []CountedItem `json:"items"`
				Total int64         `json:"total"`
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
			b := db.WrapListCounted(db.ListCountedParams{ProjectID: pid, TagFilter: tagFilter}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListCountedRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CountedItem, len(res.Data))
			for i, r := range res.Data {
				var avg int32
				if r.Count > 0 {
					avg = int32(r.TotalValue / int64(r.Count)) //nolint:gosec
				}
				out[i] = CountedItem{
					ID: r.ID, Name: r.Name, Value: r.Value,
					Count: r.Count, TotalValue: r.TotalValue, AvgValue: avg,
					Source: r.Source, ContextJson: json.RawMessage(r.ContextJson), CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Items []CountedItem `json:"items"`
					Total int64         `json:"total"`
				}
			}{
				Body: struct {
					Items []CountedItem `json:"items"`
					Total int64         `json:"total"`
				}{Items: out, Total: res.Meta.Pagination.Total},
			}, nil
		})

	type CountedGroupedItem struct {
		GroupValue    string `json:"group_value"`
		EntryCount    int32  `json:"entry_count"`
		MaxValue      int32  `json:"max_value"`
		SumTotalValue int64  `json:"sum_total_value"`
		SumCount      int32  `json:"sum_count"`
		AvgValue      int32  `json:"avg_value"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-counted-grouped", Method: http.MethodGet, Path: "/api/dx/counted/grouped"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			GroupBy   string `query:"group_by"`
			TagFilter string `query:"tag_filter,omitempty"`
		}) (*struct {
			Body struct {
				Items []CountedGroupedItem `json:"items"`
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
			b := db.WrapListCountedGrouped(db.ListCountedParams{ProjectID: pid, TagFilter: tagFilter}, in.GroupBy)
			res, err := mqpgx.Scan[db.ListCountedGroupedRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CountedGroupedItem, len(res.Data))
			for i, r := range res.Data {
				var avg int32
				if r.SumCount > 0 {
					avg = int32(r.SumTotalValue / r.SumCount) //nolint:gosec
				}
				out[i] = CountedGroupedItem{
					GroupValue: r.GroupValue, EntryCount: int32(r.EntryCount),
					MaxValue: r.MaxValue, SumTotalValue: r.SumTotalValue, SumCount: int32(r.SumCount), AvgValue: avg,
				}
			}
			return &struct {
				Body struct {
					Items []CountedGroupedItem `json:"items"`
				}
			}{Body: struct {
				Items []CountedGroupedItem `json:"items"`
			}{Items: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-counted-tag-keys", Method: http.MethodGet, Path: "/api/dx/counted/tags/keys"},
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
			rows, err := h.Q.ListCountedDistinctTagKeys(ctx, pid)
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

	huma.Register(api, huma.Operation{OperationID: "list-counted-tag-values", Method: http.MethodGet, Path: "/api/dx/counted/tags/values"},
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
			rows, err := h.Q.ListCountedDistinctTagValues(ctx, db.ListCountedDistinctTagValuesParams{ProjectID: pid, TagKey: in.TagKey})
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

	// ── Counter events (per-sample append-only) ────────────────────────────

	type CounterEventItem struct {
		ID          int64           `json:"id"`
		Name        string          `json:"name"`
		Value       int32           `json:"value"`
		Source      string          `json:"source"`
		Component   string          `json:"component"`
		Environment string          `json:"environment"`
		ContextJson json.RawMessage `json:"context_json"`
		CreatedAt   string          `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-counter-events", Method: http.MethodGet, Path: "/api/dx/counter-events"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			Limit     int32  `query:"limit"`
			Offset    int32  `query:"offset"`
			TagFilter string `query:"tag_filter,omitempty"`
			Since     string `query:"since,omitempty"`
			Until     string `query:"until,omitempty"`
		}) (*struct {
			Body struct {
				Items []CounterEventItem `json:"items"`
				Total int64              `json:"total"`
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
			b := db.WrapListCounterEvents(db.ListCounterEventsParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until,
			}).ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxCounterEvent](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CounterEventItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = CounterEventItem{
					ID: r.ID, Name: r.Name, Value: r.Value,
					Source: r.Source, Component: r.Component, Environment: r.Environment,
					ContextJson: json.RawMessage(r.ContextJson), CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Items []CounterEventItem `json:"items"`
					Total int64              `json:"total"`
				}
			}{
				Body: struct {
					Items []CounterEventItem `json:"items"`
					Total int64              `json:"total"`
				}{Items: out, Total: res.Meta.Pagination.Total},
			}, nil
		})

	type CounterEventGroupedItem struct {
		GroupValue string `json:"group_value"`
		EntryCount int32  `json:"entry_count"`
		MaxValue   int32  `json:"max_value"`
		SumValue   int64  `json:"sum_value"`
		FirstSeen  string `json:"first_seen"`
		LastSeen   string `json:"last_seen"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-counter-events-grouped", Method: http.MethodGet, Path: "/api/dx/counter-events/grouped"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			GroupBy   string `query:"group_by"`
			TagFilter string `query:"tag_filter,omitempty"`
			Since     string `query:"since,omitempty"`
			Until     string `query:"until,omitempty"`
		}) (*struct {
			Body struct {
				Items []CounterEventGroupedItem `json:"items"`
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
			b := db.WrapListCounterEventsGrouped(db.ListCounterEventsParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until,
			}, in.GroupBy)
			res, err := mqpgx.Scan[db.ListCounterEventsGroupedRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CounterEventGroupedItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = CounterEventGroupedItem{
					GroupValue: r.GroupValue, EntryCount: int32(r.EntryCount), //nolint:gosec
					MaxValue: r.MaxValue, SumValue: r.SumValue,
					FirstSeen: fmtTS(r.FirstSeen), LastSeen: fmtTS(r.LastSeen),
				}
			}
			return &struct {
				Body struct {
					Items []CounterEventGroupedItem `json:"items"`
				}
			}{Body: struct {
				Items []CounterEventGroupedItem `json:"items"`
			}{Items: out}}, nil
		})
}
