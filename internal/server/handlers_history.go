package server

import (
	"context"
	"net/http"
	"sort"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func tsUnixNano(ts pgtype.Timestamptz) int64 {
	if !ts.Valid {
		return 0
	}
	return ts.Time.UnixNano()
}

type HistoryEvent struct {
	Kind       string `json:"kind"`
	ID         int64  `json:"id"`
	CreatedAt  string `json:"created_at"`
	FromStatus string `json:"from_status,omitempty"`
	ToStatus   string `json:"to_status,omitempty"`
	Field      string `json:"field,omitempty"`
	OldVal     string `json:"old_val,omitempty"`
	NewVal     string `json:"new_val,omitempty"`
	AgentID    string `json:"agent_id"`
	SessionID  string `json:"session_id"`
	UserID     string `json:"user_id"`
}

func (s *Server) registerHistoryRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-history", Method: http.MethodGet, Path: "/api/dx/history"},
		func(ctx context.Context, in *struct {
			TargetType string `query:"target_type" required:"true"`
			TargetID   string `query:"target_id" required:"true"`
		}) (*struct {
			Body struct {
				Events []HistoryEvent `json:"events"`
			}
		}, error) {
			if in.TargetType != "issue" && in.TargetType != "task" {
				return nil, apiErr(400, "target_type must be 'issue' or 'task'")
			}

			statusRows, err := s.q.ListStatusEventsByTarget(ctx, db.ListStatusEventsByTargetParams{
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			revRows, err := s.q.ListRevisionsByTarget(ctx, db.ListRevisionsByTargetParams{
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			type sortable struct {
				ts int64
				ev HistoryEvent
			}
			rows := make([]sortable, 0, len(statusRows)+len(revRows))
			for _, r := range statusRows {
				rows = append(rows, sortable{ts: tsUnixNano(r.CreatedAt), ev: HistoryEvent{
					Kind:       "status",
					ID:         r.ID,
					CreatedAt:  fmtTS(r.CreatedAt),
					FromStatus: r.FromStatus,
					ToStatus:   r.ToStatus,
					AgentID:    r.AgentID,
					SessionID:  r.SessionID,
					UserID:     r.UserID,
				}})
			}
			for _, r := range revRows {
				rows = append(rows, sortable{ts: tsUnixNano(r.CreatedAt), ev: HistoryEvent{
					Kind:      "field",
					ID:        int64(r.ID),
					CreatedAt: fmtTS(r.CreatedAt),
					Field:     r.Field,
					OldVal:    r.OldVal,
					NewVal:    r.NewVal,
					AgentID:   r.Agent,
					SessionID: r.SessionID,
					UserID:    r.UserID,
				}})
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].ts != rows[j].ts {
					return rows[i].ts > rows[j].ts
				}
				return rows[i].ev.ID > rows[j].ev.ID
			})

			events := make([]HistoryEvent, len(rows))
			for i, r := range rows {
				events[i] = r.ev
			}

			return &struct {
				Body struct {
					Events []HistoryEvent `json:"events"`
				}
			}{Body: struct {
				Events []HistoryEvent `json:"events"`
			}{Events: events}}, nil
		})
}
