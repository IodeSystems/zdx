package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type TodoSessionItem struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	IssueID   string `json:"issue_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Alias     string `json:"alias,omitempty"`
	Header    string `json:"header,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at,omitempty"`
}

type TodoDetailBody struct {
	Todo         TodoItem          `json:"todo"`
	Reservations []ReservationItem `json:"reservations"`
	Sessions     []TodoSessionItem `json:"sessions"`
}

func (h *Handler) registerTodoRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "get-todo-detail", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/todos/{key}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Key  string `path:"key"`
		}) (*struct{ Body TodoDetailBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			t, err := h.Q.GetTodoByKey(ctx, db.GetTodoByKeyParams{ProjectID: p.ID, Key: in.Key})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "todo not found: "+in.Key)
			}
			todo := TodoItem{
				ID:         t.ID,
				Text:       t.Text,
				Key:        t.Key,
				Persona:    t.Persona,
				Priority:   t.Priority,
				Status:     t.Status,
				TargetType: t.TargetType,
				TargetID:   t.TargetID,
				Kind:       t.Kind,
				IssueRef:   t.IssueRef,
				Blocked:    t.Blocked,
				ClaimedBy:  t.ClaimedBy,
				ClaimedAt:  fmtTS(t.ClaimedAt),
				CreatedAt:  fmtTS(t.CreatedAt),
				ResolvedAt: fmtTS(t.ResolvedAt),
			}

			resRows, err := h.Q.ListReservationsByTodoKey(ctx, db.ListReservationsByTodoKeyParams{
				ProjectID: p.ID,
				Key:       in.Key,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			reservations := make([]ReservationItem, len(resRows))
			for i, r := range resRows {
				item := ReservationItem{
					ID:             r.ID,
					TargetType:     r.TargetType,
					TargetID:       r.TargetID,
					ClaimedBy:      r.ClaimedBy,
					ClaimedAt:      fmtTS(r.ClaimedAt),
					ReleasedAt:     fmtTS(r.ReleasedAt),
					LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
				}
				if r.SessionID.Valid {
					item.SessionID = r.SessionID.Int64
					item.SessionStatus = r.SessionStatus.String
					item.SessionClosedAt = fmtTS(r.SessionClosedAt)
					item.SessionHeader = r.SessionHeader.String
					item.SessionAlias = r.SessionAlias.String
				}
				reservations[i] = item
			}

			sessRows, err := h.Q.ListClaudeSessionsByTodoID(ctx, db.ListClaudeSessionsByTodoIDParams{
				ProjectID: p.ID,
				TodoID:    pgtype.Int4{Int32: t.ID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			sessions := make([]TodoSessionItem, len(sessRows))
			for i, s := range sessRows {
				sessions[i] = TodoSessionItem{
					ID:        s.ID,
					SessionID: s.SessionID,
					IssueID:   s.IssueID,
					Title:     s.Title,
					Alias:     s.Alias,
					Header:    s.Header,
					Summary:   s.Summary,
					Status:    s.Status,
					CreatedAt: fmtTS(s.CreatedAt),
					UpdatedAt: fmtTS(s.UpdatedAt),
					ClosedAt:  fmtTS(s.ClosedAt),
				}
			}

			return &struct{ Body TodoDetailBody }{Body: TodoDetailBody{
				Todo:         todo,
				Reservations: reservations,
				Sessions:     sessions,
			}}, nil
		})
}
