package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

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

			revRows, err := s.q.ListRevisionsByTarget(ctx, db.ListRevisionsByTargetParams{
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			events := make([]HistoryEvent, len(revRows))
			for i, r := range revRows {
				ev := HistoryEvent{
					ID:        int64(r.ID),
					CreatedAt: fmtTS(r.CreatedAt),
					AgentID:   r.Agent,
					SessionID: r.SessionID,
					UserID:    r.UserID,
				}
				if r.Field == "status" {
					ev.Kind = "status"
					ev.FromStatus = r.OldVal
					ev.ToStatus = r.NewVal
				} else {
					ev.Kind = "field"
					ev.Field = r.Field
					ev.OldVal = r.OldVal
					ev.NewVal = r.NewVal
				}
				events[i] = ev
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
