package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// Approximate per-token cost rates for Sonnet 4.6 in USD (per token, not per MTok).
const (
	costPerInputToken         = 3.00 / 1_000_000
	costPerOutputToken        = 15.00 / 1_000_000
	costPerCacheReadToken     = 0.30 / 1_000_000
	costPerCacheCreationToken = 3.75 / 1_000_000
)

type BudgetItem struct {
	ID           int64   `json:"id"`
	AgentID      string  `json:"agent_id"`
	ProjectID    int32   `json:"project_id"`
	TokenCeiling int64   `json:"token_ceiling"`
	CostCeiling  float64 `json:"cost_ceiling"`
	TokensUsed   int64   `json:"tokens_used"`
	CostUsed     float64 `json:"cost_used"`
	ActivePause  bool    `json:"active_pause"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

func budgetItemFrom(b db.ZdxAgentBudget) BudgetItem {
	item := BudgetItem{
		ID:        b.ID,
		CreatedAt: fmtTS(b.CreatedAt),
		UpdatedAt: fmtTS(b.UpdatedAt),
	}
	if b.AgentID.Valid {
		item.AgentID = b.AgentID.String
	}
	if b.ProjectID.Valid {
		item.ProjectID = b.ProjectID.Int32
	}
	if b.TokenCeiling.Valid {
		item.TokenCeiling = b.TokenCeiling.Int64
	}
	if b.CostCeiling.Valid {
		item.CostCeiling = b.CostCeiling.Float64
	}
	return item
}

func computeCost(usage db.GetAgentTokenUsageRow) float64 {
	return float64(usage.InputTokens)*costPerInputToken +
		float64(usage.OutputTokens)*costPerOutputToken +
		float64(usage.CacheReadInputTokens)*costPerCacheReadToken +
		float64(usage.CacheCreationInputTokens)*costPerCacheCreationToken
}

func (h *Handler) registerAgentBudgetRoutes(api huma.API) {
	// GET /api/agents/budget?agent_id=<id>  OR  ?project_id=<id>
	huma.Register(api, huma.Operation{OperationID: "get-agent-budget", Method: http.MethodGet, Path: "/api/agents/budget"},
		func(ctx context.Context, in *struct {
			AgentID   string `query:"agent_id"`
			ProjectID int32  `query:"project_id"`
		}) (*struct{ Body BudgetItem }, error) {
			if in.AgentID == "" && in.ProjectID == 0 {
				return nil, apiErr(400, "provide agent_id or project_id")
			}
			if in.AgentID != "" && in.ProjectID != 0 {
				return nil, apiErr(400, "provide only one of agent_id or project_id")
			}

			var item BudgetItem
			if in.AgentID != "" {
				b, err := h.Q.GetAgentBudget(ctx, pgtype.Text{String: in.AgentID, Valid: true})
				if err != nil {
					return nil, apiErr(404, "no budget set for agent")
				}
				item = budgetItemFrom(b)
				if usage, err := h.Q.GetAgentTokenUsage(ctx, in.AgentID); err == nil {
					item.TokensUsed = usage.InputTokens + usage.OutputTokens
					item.CostUsed = computeCost(usage)
				}
				if _, err := h.Q.GetActiveBudgetPause(ctx, in.AgentID); err == nil {
					item.ActivePause = true
				}
			} else {
				b, err := h.Q.GetProjectBudget(ctx, pgtype.Int4{Int32: in.ProjectID, Valid: true})
				if err != nil {
					return nil, apiErr(404, "no budget set for project")
				}
				item = budgetItemFrom(b)
			}
			return &struct{ Body BudgetItem }{Body: item}, nil
		})

	// GET /api/agents/budgets — list all budgets
	huma.Register(api, huma.Operation{OperationID: "list-agent-budgets", Method: http.MethodGet, Path: "/api/agents/budgets"},
		func(ctx context.Context, in *struct{}) (*struct {
			Body struct {
				Budgets []BudgetItem `json:"budgets"`
			}
		}, error) {
			rows, err := h.Q.ListBudgets(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]BudgetItem, len(rows))
			for i, r := range rows {
				out[i] = budgetItemFrom(r)
				if r.AgentID.Valid {
					if usage, uErr := h.Q.GetAgentTokenUsage(ctx, r.AgentID.String); uErr == nil {
						out[i].TokensUsed = usage.InputTokens + usage.OutputTokens
						out[i].CostUsed = computeCost(usage)
					}
					if _, pErr := h.Q.GetActiveBudgetPause(ctx, r.AgentID.String); pErr == nil {
						out[i].ActivePause = true
					}
				}
			}
			return &struct {
				Body struct {
					Budgets []BudgetItem `json:"budgets"`
				}
			}{Body: struct {
				Budgets []BudgetItem `json:"budgets"`
			}{Budgets: out}}, nil
		})

	// POST /api/agents/budget — upsert ceiling (set)
	huma.Register(api, huma.Operation{OperationID: "set-agent-budget", Method: http.MethodPost, Path: "/api/agents/budget"},
		func(ctx context.Context, in *struct {
			Body struct {
				AgentID      string  `json:"agent_id"`
				ProjectID    int32   `json:"project_id"`
				TokenCeiling int64   `json:"token_ceiling"`
				CostCeiling  float64 `json:"cost_ceiling"`
			}
		}) (*struct{ Body BudgetItem }, error) {
			if in.Body.AgentID == "" && in.Body.ProjectID == 0 {
				return nil, apiErr(400, "provide agent_id or project_id")
			}
			if in.Body.AgentID != "" && in.Body.ProjectID != 0 {
				return nil, apiErr(400, "provide only one of agent_id or project_id")
			}

			var b db.ZdxAgentBudget
			var err error
			if in.Body.AgentID != "" {
				b, err = h.Q.UpsertAgentBudget(ctx, db.UpsertAgentBudgetParams{
					AgentID:      pgtype.Text{String: in.Body.AgentID, Valid: true},
					TokenCeiling: pgtype.Int8{Int64: in.Body.TokenCeiling, Valid: in.Body.TokenCeiling > 0},
					CostCeiling:  pgtype.Float8{Float64: in.Body.CostCeiling, Valid: in.Body.CostCeiling > 0},
				})
			} else {
				b, err = h.Q.UpsertProjectBudget(ctx, db.UpsertProjectBudgetParams{
					ProjectID:    pgtype.Int4{Int32: in.Body.ProjectID, Valid: true},
					TokenCeiling: pgtype.Int8{Int64: in.Body.TokenCeiling, Valid: in.Body.TokenCeiling > 0},
					CostCeiling:  pgtype.Float8{Float64: in.Body.CostCeiling, Valid: in.Body.CostCeiling > 0},
				})
			}
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body BudgetItem }{Body: budgetItemFrom(b)}, nil
		})

	// POST /api/agents/{id}/budget/lift — lift active pause + resume
	huma.Register(api, huma.Operation{OperationID: "lift-agent-budget-pause", Method: http.MethodPost, Path: "/api/agents/{id}/budget/lift"},
		func(ctx context.Context, in *struct {
			ID   string `path:"id" required:"true"`
			Body struct {
				LiftedBy string `json:"lifted_by"`
			}
		}) (*struct {
			Body struct {
				Lifted  bool   `json:"lifted"`
				Message string `json:"message"`
			}
		}, error) {
			// Verify an active pause exists before sending resume.
			if _, err := h.Q.GetActiveBudgetPause(ctx, in.ID); err != nil {
				return nil, apiErr(404, "no active budget pause for agent "+in.ID)
			}

			liftedBy := in.Body.LiftedBy
			if liftedBy == "" {
				liftedBy = "operator"
			}
			if err := h.Q.LiftBudgetPause(ctx, db.LiftBudgetPauseParams{
				AgentID:  in.ID,
				LiftedBy: pgtype.Text{String: liftedBy, Valid: true},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}

			// Send resume command if agent is connected.
			msg := "budget pause lifted"
			if h.AgentCommander != nil {
				data, _ := json.Marshal(map[string]string{"type": "resume"})
				if err := h.AgentCommander.SendAgentCommand(ctx, in.ID, data); err == nil {
					msg = "budget pause lifted; resume sent"
					if updater := h.statusUpdater(); updater != nil {
						_ = updater.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{ID: in.ID, Status: "active"})
					}
				}
			}
			return &struct {
				Body struct {
					Lifted  bool   `json:"lifted"`
					Message string `json:"message"`
				}
			}{Body: struct {
				Lifted  bool   `json:"lifted"`
				Message string `json:"message"`
			}{Lifted: true, Message: msg}}, nil
		})
}
