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

func (h *Handler) registerErrorEventRoutes(api huma.API) {
	type ErrorEventItem struct {
		ID          int64           `json:"id"`
		Name        string          `json:"name"`
		Message     string          `json:"message"`
		StackTrace  string          `json:"stack_trace"`
		Source      string          `json:"source"`
		Component   string          `json:"component"`
		Environment string          `json:"environment"`
		ContextJson json.RawMessage `json:"context_json"`
		CreatedAt   string          `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "get-error-event", Method: http.MethodGet, Path: "/api/dx/error-events/{id}"},
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{ Body ErrorEventItem }, error) {
			row, err := h.Q.GetErrorEventByID(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "error event not found")
			}
			return &struct{ Body ErrorEventItem }{Body: ErrorEventItem{
				ID: row.ID, Name: row.Name, Message: row.Message, StackTrace: row.StackTrace,
				Source: row.Source, Component: row.Component, Environment: row.Environment,
				ContextJson: json.RawMessage(row.ContextJson), CreatedAt: fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-error-events", Method: http.MethodGet, Path: "/api/dx/error-events"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			Limit     int32  `query:"limit"`
			Offset    int32  `query:"offset"`
			TagFilter string `query:"tag_filter,omitempty"`
			Since     string `query:"since,omitempty"`
			Until     string `query:"until,omitempty"`
		}) (*struct {
			Body struct {
				Items []ErrorEventItem `json:"items"`
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
			b := db.WrapListErrorEvents(db.ListErrorEventsParams{ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ZdxErrorEvent](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ErrorEventItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = ErrorEventItem{
					ID: r.ID, Name: r.Name, Message: r.Message, StackTrace: r.StackTrace,
					Source: r.Source, Component: r.Component, Environment: r.Environment,
					ContextJson: json.RawMessage(r.ContextJson), CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Items []ErrorEventItem `json:"items"`
					Total int64            `json:"total"`
				}
			}{
				Body: struct {
					Items []ErrorEventItem `json:"items"`
					Total int64            `json:"total"`
				}{Items: out, Total: res.Meta.Pagination.Total},
			}, nil
		})

	type ErrorEventGroupedItem struct {
		GroupValue string `json:"group_value"`
		EntryCount int32  `json:"entry_count"`
		FirstSeen  string `json:"first_seen"`
		LastSeen   string `json:"last_seen"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-error-events-grouped", Method: http.MethodGet, Path: "/api/dx/error-events/grouped"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug,omitempty"`
			GroupBy   string `query:"group_by"`
			TagFilter string `query:"tag_filter,omitempty"`
			Since     string `query:"since,omitempty"`
			Until     string `query:"until,omitempty"`
		}) (*struct {
			Body struct {
				Items []ErrorEventGroupedItem `json:"items"`
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
			rows, err := h.Q.ListErrorEventsGrouped(ctx, db.ListErrorEventsGroupedParams{
				ProjectID: pid, TagFilter: tagFilter, Since: since, Until: until, GroupKey: in.GroupBy,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ErrorEventGroupedItem, len(rows))
			for i, r := range rows {
				gv, _ := r.GroupValue.(string)
				firstSeen := ""
				lastSeen := ""
				if ts, ok := r.FirstSeen.(pgtype.Timestamptz); ok {
					firstSeen = fmtTS(ts)
				}
				if ts, ok := r.LastSeen.(pgtype.Timestamptz); ok {
					lastSeen = fmtTS(ts)
				}
				out[i] = ErrorEventGroupedItem{
					GroupValue: gv, EntryCount: r.EntryCount,
					FirstSeen: firstSeen, LastSeen: lastSeen,
				}
			}
			return &struct {
				Body struct {
					Items []ErrorEventGroupedItem `json:"items"`
				}
			}{Body: struct {
				Items []ErrorEventGroupedItem `json:"items"`
			}{Items: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-error-events-tag-keys", Method: http.MethodGet, Path: "/api/dx/error-events/tags/keys"},
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
			rows, err := h.Q.ListErrorEventsDistinctTagKeys(ctx, pid)
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

	huma.Register(api, huma.Operation{OperationID: "list-error-events-tag-values", Method: http.MethodGet, Path: "/api/dx/error-events/tags/values"},
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
			rows, err := h.Q.ListErrorEventsDistinctTagValues(ctx, db.ListErrorEventsDistinctTagValuesParams{ProjectID: pid, TagKey: in.TagKey})
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
