package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

type StatusEventItem struct {
	ID         int64  `json:"id"`
	TargetType string `json:"target_type"`
	TargetID   string `json:"target_id"`
	FromStatus string `json:"from_status"`
	ToStatus   string `json:"to_status"`
	AgentID    string `json:"agent_id"`
	SessionID  string `json:"session_id"`
	UserID     string `json:"user_id"`
	CreatedAt  string `json:"created_at"`
}

func (s *Server) registerStatusEventsRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-status-events", Method: http.MethodGet, Path: "/api/dx/status-events"},
		func(ctx context.Context, in *struct {
			TargetType string `query:"target_type" required:"true"`
			TargetID   string `query:"target_id" required:"true"`
		}) (*struct {
			Body struct {
				Events []StatusEventItem `json:"events"`
			}
		}, error) {
			if in.TargetType != "issue" && in.TargetType != "task" {
				return nil, apiErr(400, "target_type must be 'issue' or 'task'")
			}
			rows, err := s.q.ListStatusEventsByTarget(ctx, db.ListStatusEventsByTargetParams{
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]StatusEventItem, len(rows))
			for i, r := range rows {
				out[i] = StatusEventItem{
					ID:         r.ID,
					TargetType: r.TargetType,
					TargetID:   r.TargetID,
					FromStatus: r.FromStatus,
					ToStatus:   r.ToStatus,
					AgentID:    r.AgentID,
					SessionID:  r.SessionID,
					UserID:     r.UserID,
					CreatedAt:  fmtTS(r.CreatedAt),
				}
			}
			return &struct {
				Body struct {
					Events []StatusEventItem `json:"events"`
				}
			}{Body: struct {
				Events []StatusEventItem `json:"events"`
			}{Events: out}}, nil
		})
}
