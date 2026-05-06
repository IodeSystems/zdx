package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type AgentItem struct {
	ID string `json:"id"`
	// ProjectID is null for global-pool agents (no project assignment).
	// Project-scoped agents carry their project's id.
	ProjectID *int32 `json:"project_id"`
	// ProjectSlug + ProjectName are populated by the server-wide list
	// endpoint (/api/agents) so the UI can render scope without a
	// secondary lookup. Empty when the agent is global or when called
	// from the project-scoped list (which already knows the project).
	ProjectSlug    string `json:"project_slug,omitempty"`
	ProjectName    string `json:"project_name,omitempty"`
	SessionID      string `json:"session_id"`
	WorktreePath   string `json:"worktree_path"`
	WorktreeBranch string `json:"worktree_branch"`
	Pid            int32  `json:"pid"`
	Status         string `json:"status"`
	TaskGroup      string `json:"task_group"`
	ComposeProject string `json:"compose_project"`
	ServerPort     int32  `json:"server_port"`
	DatabaseUrl    string `json:"database_url"`
	ValkeyUrl      string `json:"valkey_url"`
	Idle           bool   `json:"idle"`
	// OriginallyGlobal is true when the agent was first registered into
	// the global pool. Drives the assign/unassign rule: only originally-
	// global agents can be pinned/unpinned.
	OriginallyGlobal bool   `json:"originally_global"`
	LastHeartbeat    string `json:"last_heartbeat"`
	CreatedAt        string `json:"created_at"`
	ConnectionState  string `json:"connection_state"` // connected | disconnected | paused | draining
	ConnectedAt      string `json:"connected_at"`     // non-empty when connection_state=connected
}

type AgentTaskItem struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Text           string `json:"text"`
	Feature        string `json:"feature"`
	Status         string `json:"status"`
	Issue          string `json:"issue"`
	TaskGroup      string `json:"task_group"`
	CreatedAt      string `json:"created_at"`
	ClaimedAt      string `json:"claimed_at"`
	LeaseExpiresAt string `json:"lease_expires_at"`
}

// statusUpdater returns the configured AgentStatusUpdater, falling back to h.Q
// (which satisfies the interface) so prod wiring keeps working without an
// explicit Deps.AgentStatusUpdater. Returns nil when neither is set so unit
// tests that don't care about status updates can omit both.
func (h *Handler) statusUpdater() AgentStatusUpdater {
	if h.AgentStatusUpdater != nil {
		return h.AgentStatusUpdater
	}
	if h.Q != nil {
		return h.Q
	}
	return nil
}

// agentItemFrom converts a DB row to an API item. reg may be nil (no live state).
// Connection-state precedence: if status is paused/draining it takes priority over
// registry presence, since those states persist across reconnects.
func agentItemFrom(a db.ZdxAgent, reg AgentConnRegistry) AgentItem {
	connState := "disconnected"
	connectedAt := ""
	if reg != nil && reg.IsConnected(a.ID) {
		connState = "connected"
		if t := reg.ConnectedAt(a.ID); !t.IsZero() {
			connectedAt = t.UTC().Format(time.RFC3339)
		}
	}
	if a.Status == "paused" || a.Status == "draining" {
		connState = a.Status
	}
	var pid *int32
	if a.ProjectID.Valid {
		v := a.ProjectID.Int32
		pid = &v
	}
	return AgentItem{
		ID:               a.ID,
		ProjectID:        pid,
		SessionID:        a.SessionID,
		Idle:             a.Idle,
		OriginallyGlobal: a.OriginallyGlobal,
		WorktreePath:     a.WorktreePath,
		WorktreeBranch:   a.WorktreeBranch,
		Pid:              a.Pid,
		Status:           a.Status,
		TaskGroup:        a.TaskGroup,
		ComposeProject:   a.ComposeProject,
		ServerPort:       a.ServerPort,
		DatabaseUrl:      a.DatabaseUrl,
		ValkeyUrl:        a.ValkeyUrl,
		LastHeartbeat:    fmtTS(a.LastHeartbeat),
		CreatedAt:        fmtTS(a.CreatedAt),
		ConnectionState:  connState,
		ConnectedAt:      connectedAt,
	}
}

func (h *Handler) registerAgentRoutes(api huma.API) {
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
				ValkeyUrl      string `json:"valkey_url"`
			}
		}) (*struct{ Body AgentItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Body.Status
			if status == "" {
				status = "active"
			}
			a, err := h.Q.RegisterAgent(ctx, db.RegisterAgentParams{
				ID:             in.Body.ID,
				ProjectID:      pgtype.Int4{Int32: p.ID, Valid: true},
				SessionID:      in.Body.SessionID,
				WorktreePath:   in.Body.WorktreePath,
				WorktreeBranch: in.Body.WorktreeBranch,
				Pid:            in.Body.Pid,
				Status:         status,
				TaskGroup:      in.Body.TaskGroup,
				ComposeProject: in.Body.ComposeProject,
				ServerPort:     in.Body.ServerPort,
				DatabaseUrl:    in.Body.DatabaseUrl,
				ValkeyUrl:      in.Body.ValkeyUrl,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := agentItemFrom(a, nil)
			return &struct{ Body AgentItem }{Body: item}, nil
		})

	// List agents for project
	huma.Register(api, huma.Operation{OperationID: "list-agents", Method: http.MethodGet, Path: "/api/agents/list"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Agents []AgentItem `json:"agents"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListAgentsByProject(ctx, pgtype.Int4{Int32: p.ID, Valid: true})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentItem, len(rows))
			for i, r := range rows {
				out[i] = agentItemFrom(r, h.AgentConnRegistry)
			}
			return &struct {
				Body struct {
					Agents []AgentItem `json:"agents"`
				}
			}{Body: struct {
				Agents []AgentItem `json:"agents"`
			}{Agents: out}}, nil
		})

	// Server-wide list (across every project + the global pool). Used by
	// the top-level /agents nav so operators see every connected and
	// registered agent in one panel. Returns AgentItem augmented with
	// project_slug + project_name so the UI can render scope without a
	// secondary lookup.
	huma.Register(api, huma.Operation{OperationID: "list-all-agents", Method: http.MethodGet, Path: "/api/agents"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Agents []AgentItem `json:"agents"`
			}
		}, error) {
			rows, err := h.Q.ListAllAgents(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentItem, len(rows))
			for i, r := range rows {
				// Promote ListAllAgentsRow → ZdxAgent fields on agentItemFrom
				// by reusing the existing converter, then layer on the joined
				// project columns the converter doesn't know about.
				agent := db.ZdxAgent{
					ID: r.ID, ProjectID: r.ProjectID, SessionID: r.SessionID,
					WorktreePath: r.WorktreePath, WorktreeBranch: r.WorktreeBranch,
					Pid: r.Pid, Status: r.Status, TaskGroup: r.TaskGroup,
					ComposeProject: r.ComposeProject, ServerPort: r.ServerPort,
					DatabaseUrl: r.DatabaseUrl, ValkeyUrl: r.ValkeyUrl,
					Idle: r.Idle, OriginallyGlobal: r.OriginallyGlobal,
					LastHeartbeat: r.LastHeartbeat,
					CreatedAt:     r.CreatedAt, DisconnectAt: r.DisconnectAt,
				}
				out[i] = agentItemFrom(agent, h.AgentConnRegistry)
				if r.ProjectSlug.Valid {
					out[i].ProjectSlug = r.ProjectSlug.String
				}
				if r.ProjectName.Valid {
					out[i].ProjectName = r.ProjectName.String
				}
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
			a, err := h.Q.GetAgent(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "agent not found")
			}
			item := agentItemFrom(a, h.AgentConnRegistry)
			return &struct{ Body AgentItem }{Body: item}, nil
		})

	// Assign an originally-global agent to a project (pin).
	// 400 when the agent was originally registered project-scoped — those
	// are scope-immutable per design (re-register if the operator wants a
	// different project).
	huma.Register(api, huma.Operation{OperationID: "assign-agent", Method: http.MethodPost, Path: "/api/agents/{id}/assign"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				ProjectSlug string `json:"project_slug" required:"true"`
			}
		}) (*struct{ Body AgentItem }, error) {
			a, err := h.Q.GetAgent(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "agent not found")
			}
			if !a.OriginallyGlobal {
				return nil, apiErr(400, "agent was originally registered project-scoped; re-register to change scope")
			}
			p, err := h.Q.GetProjectBySlug(ctx, in.Body.ProjectSlug)
			if err != nil {
				return nil, apiErr(404, "project not found: "+in.Body.ProjectSlug)
			}
			updated, err := h.Q.AssignAgentToProject(ctx, db.AssignAgentToProjectParams{
				ID:        in.ID,
				ProjectID: pgtype.Int4{Int32: p.ID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := agentItemFrom(updated, h.AgentConnRegistry)
			item.ProjectSlug = p.Slug
			item.ProjectName = p.Name
			return &struct{ Body AgentItem }{Body: item}, nil
		})

	// Unassign an originally-global agent from its current project (unpin).
	// 400 when the agent was originally registered project-scoped.
	huma.Register(api, huma.Operation{OperationID: "unassign-agent", Method: http.MethodDelete, Path: "/api/agents/{id}/assign"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{ Body AgentItem }, error) {
			a, err := h.Q.GetAgent(ctx, in.ID)
			if err != nil {
				return nil, apiErr(404, "agent not found")
			}
			if !a.OriginallyGlobal {
				return nil, apiErr(400, "agent was originally registered project-scoped; cannot unassign")
			}
			updated, err := h.Q.UnassignAgent(ctx, in.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := agentItemFrom(updated, h.AgentConnRegistry)
			return &struct{ Body AgentItem }{Body: item}, nil
		})

	// Heartbeat
	huma.Register(api, huma.Operation{OperationID: "agent-heartbeat", Method: http.MethodPost, Path: "/api/agents/{id}/heartbeat"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{}, error) {
			if err := h.Q.UpdateAgentHeartbeat(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// Delete agent
	huma.Register(api, huma.Operation{OperationID: "delete-agent", Method: http.MethodDelete, Path: "/api/agents/{id}"},
		func(ctx context.Context, in *struct {
			ID string `path:"id" required:"true"`
		}) (*struct{}, error) {
			if err := h.Q.DeleteAgent(ctx, in.ID); err != nil {
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
			rows, err := h.Q.ReapStaleAgents(ctx, interval)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentItem, len(rows))
			for i, r := range rows {
				out[i] = agentItemFrom(r, nil)
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
			rows, err := h.Q.ListTasksByAgent(ctx, in.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AgentTaskItem, len(rows))
			for i, r := range rows {
				out[i] = AgentTaskItem{
					ID:             r.ID,
					Title:          r.Title,
					Text:           r.Text,
					Feature:        r.Feature,
					Status:         r.Status,
					Issue:          r.Issue,
					TaskGroup:      r.TaskGroup,
					CreatedAt:      fmtTS(r.CreatedAt),
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			dur := in.Body.LeaseDurationMin
			if dur <= 0 {
				dur = 30
			}
			leaseExpiry := pgtype.Timestamptz{Time: time.Now().Add(time.Duration(dur) * time.Minute), Valid: true}
			t, err := h.Q.ClaimTask(ctx, db.ClaimTaskParams{
				ProjectID: p.ID,
				TaskGroup: in.Body.TaskGroup,
				Issue:     in.Body.Issue,
			})
			if err != nil {
				return nil, apiErr(404, "no tasks available to claim")
			}
			res, _ := h.Q.InsertReservation(ctx, db.InsertReservationParams{
				ProjectID:      t.ProjectID,
				TargetType:     "task",
				TargetID:       t.ID,
				ClaimedBy:      in.Body.AgentID,
				LeaseExpiresAt: leaseExpiry,
			})
			h.recordStatusChange(ctx, p.ID, "task", t.ID, "ready", "active", in.Body.AgentID)
			item := AgentTaskItem{
				ID:             t.ID,
				Title:          t.Title,
				Text:           t.Text,
				Feature:        t.Feature,
				Status:         t.Status,
				Issue:          t.Issue,
				TaskGroup:      t.TaskGroup,
				CreatedAt:      fmtTS(t.CreatedAt),
				ClaimedAt:      fmtTS(res.ClaimedAt),
				LeaseExpiresAt: fmtTS(res.LeaseExpiresAt),
			}
			return &struct{ Body AgentTaskItem }{Body: item}, nil
		})

	// Release a task. If agent_id is empty, performs an admin release
	// (clears the claim unconditionally); otherwise only the claiming agent
	// can release.
	huma.Register(api, huma.Operation{OperationID: "release-task", Method: http.MethodPost, Path: "/api/tasks/{id}/release"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				AgentID string `json:"agent_id"`
			}
		}) (*struct{}, error) {
			taskID := in.ID
			prev := ""
			var projectID int32
			if t, gErr := h.Q.GetTask(ctx, taskID); gErr == nil {
				prev = t.Status
				projectID = t.ProjectID
			}
			if err := h.Q.ReleaseTask(ctx, taskID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordTaskStatusChange(ctx, taskID, prev, "ready", in.Body.AgentID)
			if projectID != 0 {
				_ = h.Q.ReleaseReservation(ctx, db.ReleaseReservationParams{
					ProjectID:  projectID,
					TargetType: "task",
					TargetID:   taskID,
				})
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
			if err := h.Q.RenewTaskReservation(ctx, db.RenewTaskReservationParams{
				LeaseDuration: interval,
				TargetID:      in.ID,
				ClaimedBy:     in.Body.AgentID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// Send control command to a connected agent daemon over its WS channel.
	// Valid commands: "pause", "resume", "drain", "kill".
	// Returns 404 if the agent is not currently connected.
	huma.Register(api, huma.Operation{OperationID: "send-agent-command", Method: http.MethodPost, Path: "/api/agents/{id}/command"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				Command string `json:"command" required:"true"`
			}
		}) (*struct{}, error) {
			if h.AgentCommander == nil {
				return nil, apiErr(503, "agent commander not available")
			}
			msg, _ := json.Marshal(map[string]string{"type": in.Body.Command})
			if err := h.AgentCommander.SendAgentCommand(ctx, in.ID, msg); err != nil {
				return nil, apiErr(404, "agent not connected")
			}
			// Update DB status so reclaim-expired correctly exempts paused agents
			// and dx agent list (IS-819) reflects the new state.
			if updater := h.statusUpdater(); updater != nil {
				statusFor := map[string]string{"pause": "paused", "resume": "active"}
				if newStatus, ok := statusFor[in.Body.Command]; ok {
					_ = updater.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{ID: in.ID, Status: newStatus})
				}
			}
			// Operator-driven resume implicitly approves more spend, so any
			// budget pause held against this agent is lifted in the same
			// transaction. Without this, a watcher that tripped a budget
			// pause would immediately re-pause the agent on the next sweep.
			if in.Body.Command == "resume" && h.Q != nil {
				_ = h.Q.LiftBudgetPause(ctx, db.LiftBudgetPauseParams{
					AgentID:  in.ID,
					LiftedBy: pgtype.Text{String: "operator", Valid: true},
				})
			}
			return &struct{}{}, nil
		})

	// Expire task lease (test-mode: backdates lease so reclaim-expired can be triggered without direct DB access)
	huma.Register(api, huma.Operation{OperationID: "expire-task-lease-for-test", Method: http.MethodPost, Path: "/api/tasks/test/expire-lease"},
		func(ctx context.Context, in *struct {
			Body struct {
				TaskID string `json:"task_id"`
			}
		}) (*struct{}, error) {
			if err := h.Q.ExpireTaskLeaseForTest(ctx, in.Body.TaskID); err != nil {
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
			grace := pgtype.Interval{
				Microseconds: int64(h.AgentDisconnectGraceSec) * 1_000_000,
				Valid:        true,
			}
			rows, err := h.Q.ReclaimExpiredTasks(ctx, grace)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, r := range rows {
				h.recordStatusChange(ctx, r.ProjectID, "task", r.ID, "active", "ready", "")
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
