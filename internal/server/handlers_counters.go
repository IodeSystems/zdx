package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerCounterRoutes(api huma.API) {
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
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			total, _ := s.q.CountCounted(ctx, db.CountCountedParams{ProjectID: pid, TagFilter: tagFilter})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListCountedPaginated(ctx, db.ListCountedPaginatedParams{ProjectID: pid, TagFilter: tagFilter, Lim: limit, Off: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CountedItem, len(rows))
			for i, r := range rows {
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
				}{Items: out, Total: total},
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
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			var tagFilter []byte
			if in.TagFilter != "" {
				tagFilter = []byte(in.TagFilter)
			}
			rows, err := s.q.ListCountedGrouped(ctx, db.ListCountedGroupedParams{ProjectID: pid, TagFilter: tagFilter, GroupKey: in.GroupBy})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CountedGroupedItem, len(rows))
			for i, r := range rows {
				gv, _ := r.GroupValue.(string)
				maxVal, _ := r.MaxValue.(int32)
				var avg int32
				if r.SumCount > 0 {
					avg = int32(r.SumTotalValue / int64(r.SumCount)) //nolint:gosec
				}
				out[i] = CountedGroupedItem{
					GroupValue: gv, EntryCount: r.EntryCount,
					MaxValue: maxVal, SumTotalValue: r.SumTotalValue, SumCount: r.SumCount, AvgValue: avg,
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
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := s.q.ListCountedDistinctTagKeys(ctx, pid)
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
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					pid = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := s.q.ListCountedDistinctTagValues(ctx, db.ListCountedDistinctTagValuesParams{ProjectID: pid, TagKey: in.TagKey})
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
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
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
			total, _ := s.q.CountCounterEvents(ctx, db.CountCounterEventsParams{ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListCounterEvents(ctx, db.ListCounterEventsParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until, Lim: limit, Off: offset,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CounterEventItem, len(rows))
			for i, r := range rows {
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
				}{Items: out, Total: total},
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
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
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
			rows, err := s.q.ListCounterEventsGrouped(ctx, db.ListCounterEventsGroupedParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until, GroupKey: in.GroupBy,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CounterEventGroupedItem, len(rows))
			for i, r := range rows {
				gv, _ := r.GroupValue.(string)
				maxVal, _ := r.MaxValue.(int32)
				firstSeen := ""
				lastSeen := ""
				if ts, ok := r.FirstSeen.(pgtype.Timestamptz); ok {
					firstSeen = fmtTS(ts)
				}
				if ts, ok := r.LastSeen.(pgtype.Timestamptz); ok {
					lastSeen = fmtTS(ts)
				}
				out[i] = CounterEventGroupedItem{
					GroupValue: gv, EntryCount: r.EntryCount,
					MaxValue: maxVal, SumValue: r.SumValue,
					FirstSeen: firstSeen, LastSeen: lastSeen,
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
