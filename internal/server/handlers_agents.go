package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type AgentItem struct {
	ID             string `json:"id"`
	ProjectID      int32  `json:"project_id"`
	SessionID      string `json:"session_id"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	Pid            int32  `json:"pid"`
	Status         string `json:"status"`
	TaskGroup      string `json:"task_group"`
	ComposeProject string `json:"compose_project"`
	ServerPort     int32  `json:"server_port"`
	DatabaseUrl    string `json:"database_url"`
	LastHeartbeat  string `json:"last_heartbeat"`
	CreatedAt      string `json:"created_at"`
}

type AgentTaskItem struct {
	ID             string `json:"id"`
	Text           string `json:"text"`
	Feature        string `json:"feature"`
	Status         string `json:"status"`
	Issue          string `json:"issue"`
	TaskGroup      string `json:"task_group"`
	ClaimedAt      string `json:"claimed_at"`
	LeaseExpiresAt string `json:"lease_expires_at"`
}

func agentItemFrom(a db.ZdxAgent) AgentItem {
	return AgentItem{
		ID:             a.ID,
		ProjectID:      a.ProjectID,
		SessionID:      a.SessionID,
		WorktreePath:   a.WorktreePath,
		WorktreeBranch: a.WorktreeBranch,
		Pid:            a.Pid,
		Status:         a.Status,
		TaskGroup:      a.TaskGroup,
		ComposeProject: a.ComposeProject,
		ServerPort:     a.ServerPort,
		DatabaseUrl:    a.DatabaseUrl,
		LastHeartbeat:  fmtTS(a.LastHeartbeat),
		CreatedAt:      fmtTS(a.CreatedAt),
	}
}

func (s *Server) registerAgentRoutes(api huma.API) {
	// Register / upsert agent
	huma.Register(api, huma.Operation{OperationID: "register-agent", Method: http.MethodPost, Path: "/api/agents/register"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug           string `json:"slug" required:"true"`
				ID             string `json:"id" required:"true"`
				SessionID      string `json:"session_id"`
				WorktreePath   string `json:"worktree_path"`
				WorktreeBranch string `json:"worktree_branch"`
				Pid            int32  `json:"pid"`
				Status         string `json:"status"`
				TaskGroup      string `json:"task_group"`
				ComposeProject string `json:"compose_project"`
				ServerPort     int32  `json:"server_port"`
				DatabaseUrl    string `json:"database_url"`
			}
		}) (*struct{ Body AgentItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Body.Status
			if status == "" {
				status = "active"
			}
			a, err := s.q.RegisterAgent(ctx, db.RegisterAgentParams{
				ID:             in.Body.ID,
				ProjectID:      p.ID,
				SessionID:      in.Body.SessionID,
				WorktreePath:   in.Body.WorktreePath,
				WorktreeBranch: in.Body.WorktreeBranch,
				Pid:            in.Body.Pid,
				Status:         status,
				TaskGroup:      in.Body.TaskGroup,
				ComposeProject: in.Body.ComposeProject,
				ServerPort:     in.Body.ServerPort,
				DatabaseUrl:    in.Body.DatabaseUrl,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := agentItemFrom(a)
			return &struct{ Body AgentItem }{Body: item}, nil
		})

	// List agents for project
	huma.Register(api, huma.Operation{OperationID: "list-agents", Method: http.MethodGet, Path: "/api/agents/list"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Agents []AgentItem `json:"agents"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListAgentsByProject(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentItem, len(rows))
			for i, r := range rows {
				out[i] = agentItemFrom(r)
			}
			return &struct {
				Body struct {
					Agents []AgentItem `json:"agents"`
				}
			}{Body: struct {
				Agents []AgentItem `json:"agents"`
			}{Agents: out}}, nil
		})

	// Get single agent
	huma.Register(api, huma.Operation{OperationID: "get-agent", Method: http.MethodGet, Path: "/api/agents/{id}"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{ Body AgentItem }, error) {
			a, err := s.q.GetAgent(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "agent not found")
			}
			item := agentItemFrom(a)
			return &struct{ Body AgentItem }{Body: item}, nil
		})

	// Heartbeat
	huma.Register(api, huma.Operation{OperationID: "agent-heartbeat", Method: http.MethodPost, Path: "/api/agents/{id}/heartbeat"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{}, error) {
			if err := s.q.UpdateAgentHeartbeat(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// Delete agent
	huma.Register(api, huma.Operation{OperationID: "delete-agent", Method: http.MethodDelete, Path: "/api/agents/{id}"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{}, error) {
			if err := s.q.DeleteAgent(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// Reap stale agents
	huma.Register(api, huma.Operation{OperationID: "reap-agents", Method: http.MethodPost, Path: "/api/agents/reap"},
		func(ctx context.Context, in *struct {
			Body struct {
				ThresholdMinutes int32 `json:"threshold_minutes"`
			}
		}) (*struct {
			Body struct {
				Reaped []AgentItem `json:"reaped"`
			}
		}, error) {
			mins := in.Body.ThresholdMinutes
			if mins <= 0 {
				mins = 5
			}
			interval := pgtype.Interval{Microseconds: int64(mins) * 60 * 1_000_000, Valid: true}
			rows, err := s.q.ReapStaleAgents(ctx, interval)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentItem, len(rows))
			for i, r := range rows {
				out[i] = agentItemFrom(r)
			}
			return &struct {
				Body struct {
					Reaped []AgentItem `json:"reaped"`
				}
			}{Body: struct {
				Reaped []AgentItem `json:"reaped"`
			}{Reaped: out}}, nil
		})

	// List tasks claimed by agent
	huma.Register(api, huma.Operation{OperationID: "list-agent-tasks", Method: http.MethodGet, Path: "/api/agents/{id}/tasks"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct {
			Body struct {
				Tasks []AgentTaskItem `json:"tasks"`
			}
		}, error) {
			rows, err := s.q.ListTasksByAgent(ctx, pgtype.Text{String: in.ID, Valid: true})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentTaskItem, len(rows))
			for i, r := range rows {
				out[i] = AgentTaskItem{
					ID:             r.ID,
					Text:           r.Text,
					Feature:        r.Feature,
					Status:         r.Status,
					Issue:          r.Issue,
					TaskGroup:      r.TaskGroup,
					ClaimedAt:      fmtTS(r.ClaimedAt),
					LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
				}
			}
			return &struct {
				Body struct {
					Tasks []AgentTaskItem `json:"tasks"`
				}
			}{Body: struct {
				Tasks []AgentTaskItem `json:"tasks"`
			}{Tasks: out}}, nil
		})

	// Claim a task
	huma.Register(api, huma.Operation{OperationID: "claim-task", Method: http.MethodPost, Path: "/api/tasks/claim"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug             string `json:"slug" required:"true"`
				AgentID          string `json:"agent_id" required:"true"`
				TaskGroup        string `json:"task_group"`
				Issue            string `json:"issue"`
				LeaseDurationMin int32  `json:"lease_duration_min"`
			}
		}) (*struct{ Body AgentTaskItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			dur := in.Body.LeaseDurationMin
			if dur <= 0 {
				dur = 30
			}
			interval := pgtype.Interval{Microseconds: int64(dur) * 60 * 1_000_000, Valid: true}
			t, err := s.q.ClaimTask(ctx, db.ClaimTaskParams{
				AgentID:       pgtype.Text{String: in.Body.AgentID, Valid: true},
				LeaseDuration: interval,
				ProjectID:     p.ID,
				TaskGroup:     in.Body.TaskGroup,
				Issue:         in.Body.Issue,
			})
			if err != nil {
				return nil, apiErr(404, "no tasks available to claim")
			}
			item := AgentTaskItem{
				ID:             t.ID,
				Text:           t.Text,
				Feature:        t.Feature,
				Status:         t.Status,
				Issue:          t.Issue,
				TaskGroup:      t.TaskGroup,
				ClaimedAt:      fmtTS(t.ClaimedAt),
				LeaseExpiresAt: fmtTS(t.LeaseExpiresAt),
			}
			return &struct{ Body AgentTaskItem }{Body: item}, nil
		})

	// Release a task
	huma.Register(api, huma.Operation{OperationID: "release-task", Method: http.MethodPost, Path: "/api/tasks/{id}/release"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				AgentID string `json:"agent_id" required:"true"`
			}
		}) (*struct{}, error) {
			taskID := in.ID
			if err := s.q.ReleaseTask(ctx, db.ReleaseTaskParams{
				ID:        taskID,
				ClaimedBy: pgtype.Text{String: in.Body.AgentID, Valid: true},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// Renew task lease
	huma.Register(api, huma.Operation{OperationID: "renew-task-lease", Method: http.MethodPost, Path: "/api/tasks/{id}/renew-lease"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				AgentID          string `json:"agent_id" required:"true"`
				LeaseDurationMin int32  `json:"lease_duration_min"`
			}
		}) (*struct{}, error) {
			dur := in.Body.LeaseDurationMin
			if dur <= 0 {
				dur = 30
			}
			interval := pgtype.Interval{Microseconds: int64(dur) * 60 * 1_000_000, Valid: true}
			if err := s.q.RenewTaskLease(ctx, db.RenewTaskLeaseParams{
				LeaseDuration: interval,
				ID:            in.ID,
				AgentID:       pgtype.Text{String: in.Body.AgentID, Valid: true},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// Reclaim expired tasks
	huma.Register(api, huma.Operation{OperationID: "reclaim-expired-tasks", Method: http.MethodPost, Path: "/api/tasks/reclaim-expired"},
		func(ctx context.Context, in *struct{}) (*struct {
			Body struct {
				Reclaimed int `json:"reclaimed"`
			}
		}, error) {
			rows, err := s.q.ReclaimExpiredTasks(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Reclaimed int `json:"reclaimed"`
				}
			}{Body: struct {
				Reclaimed int `json:"reclaimed"`
			}{Reclaimed: len(rows)}}, nil
		})
}
