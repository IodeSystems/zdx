package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// MaturityItem is the wire shape of zdx_maturity_items. Fields mirror the
// SQL table; timestamps are RFC3339 strings; snooze_until is empty when unset.
type MaturityItem struct {
	ID             int32  `json:"id"`
	ProjectID      int32  `json:"project_id"`
	Kind           string `json:"kind"`
	TargetType     string `json:"target_type"`
	TargetID       int32  `json:"target_id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Status         string `json:"status"`
	Justification  string `json:"justification"`
	SnoozeUntil    string `json:"snooze_until"`
	SourceQuestion string `json:"source_question"`
	PriorityHint   int32  `json:"priority_hint"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

func maturityItemFrom(r db.ZdxMaturityItem) MaturityItem {
	targetID := int32(0)
	if r.TargetID.Valid {
		targetID = r.TargetID.Int32
	}
	return MaturityItem{
		ID:             r.ID,
		ProjectID:      r.ProjectID,
		Kind:           r.Kind,
		TargetType:     r.TargetType,
		TargetID:       targetID,
		Title:          r.Title,
		Description:    r.Description,
		Status:         r.Status,
		Justification:  r.Justification,
		SnoozeUntil:    fmtTS(r.SnoozeUntil),
		SourceQuestion: r.SourceQuestion,
		PriorityHint:   r.PriorityHint,
		CreatedAt:      fmtTS(r.CreatedAt),
		UpdatedAt:      fmtTS(r.UpdatedAt),
	}
}

func (h *Handler) registerMaturityRoutes(api huma.API) {
	// GET /api/dx/maturity/items?slug=...&status=open
	huma.Register(api, huma.Operation{OperationID: "list-maturity-items", Method: http.MethodGet, Path: "/api/dx/maturity/items"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			Status string `query:"status"`
		}) (*struct {
			Body struct {
				Items []MaturityItem `json:"items"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListMaturityItems(ctx, db.ListMaturityItemsParams{
				ProjectID:    p.ID,
				StatusFilter: in.Status,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]MaturityItem, len(rows))
			for i, r := range rows {
				out[i] = maturityItemFrom(r)
			}
			return &struct {
				Body struct {
					Items []MaturityItem `json:"items"`
				}
			}{Body: struct {
				Items []MaturityItem `json:"items"`
			}{Items: out}}, nil
		})

	// GET /api/dx/maturity/items/{id}
	huma.Register(api, huma.Operation{OperationID: "get-maturity-item", Method: http.MethodGet, Path: "/api/dx/maturity/items/{id}"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{ Body MaturityItem }, error) {
			id, err := parseMaturityID(in.ID)
			if err != nil {
				return nil, err
			}
			r, err := h.Q.GetMaturityItem(ctx, id)
			if err != nil {
				return nil, apiErr(404, "maturity item not found")
			}
			return &struct{ Body MaturityItem }{Body: maturityItemFrom(r)}, nil
		})

	// PATCH /api/dx/maturity/items/{id}
	huma.Register(api, huma.Operation{OperationID: "update-maturity-item", Method: http.MethodPatch, Path: "/api/dx/maturity/items/{id}"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				Status        string `json:"status"`
				Justification string `json:"justification" required:"false"`
				SnoozeUntil   string `json:"snooze_until" required:"false"`
			}
		}) (*struct{ Body MaturityItem }, error) {
			id, err := parseMaturityID(in.ID)
			if err != nil {
				return nil, err
			}
			params, err := buildMaturityStatusParams(id, in.Body.Status, in.Body.Justification, in.Body.SnoozeUntil)
			if err != nil {
				return nil, err
			}
			r, err := h.Q.UpdateMaturityItemStatus(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body MaturityItem }{Body: maturityItemFrom(r)}, nil
		})
}

func parseMaturityID(raw string) (int32, error) {
	n, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || n <= 0 {
		return 0, apiErr(400, "invalid maturity item id")
	}
	return int32(n), nil
}

func buildMaturityStatusParams(id int32, status, justification, snoozeUntil string) (db.UpdateMaturityItemStatusParams, error) {
	switch status {
	case "open", "done", "snoozed", "dismissed":
	default:
		return db.UpdateMaturityItemStatusParams{}, apiErr(400, "status must be one of: open, done, snoozed, dismissed")
	}
	if (status == "done" || status == "dismissed") && justification == "" {
		return db.UpdateMaturityItemStatusParams{}, apiErr(400, "justification required when status is "+status)
	}
	var snooze pgtype.Timestamptz
	if status == "snoozed" {
		if snoozeUntil == "" {
			return db.UpdateMaturityItemStatusParams{}, apiErr(400, "snooze_until required when status is snoozed")
		}
		t, err := time.Parse(time.RFC3339, snoozeUntil)
		if err != nil {
			return db.UpdateMaturityItemStatusParams{}, apiErr(400, "snooze_until must be RFC3339")
		}
		snooze = pgtype.Timestamptz{Time: t, Valid: true}
	}
	// open/done/dismissed always clear snooze_until (snooze is snoozed-only state).
	return db.UpdateMaturityItemStatusParams{
		ID:            id,
		Status:        status,
		Justification: justification,
		SnoozeUntil:   snooze,
	}, nil
}
