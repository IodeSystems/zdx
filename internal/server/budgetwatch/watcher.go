// Package budgetwatch enforces per-agent and per-project token+cost ceilings
// configured in zdx_agent_budgets. Every poll interval the watcher rolls up
// each connected agent's live usage; if it has crossed its ceiling and no
// active pause exists for it, the watcher sends a "pause" control message
// over the agent's WS connection (reusing IS-602 pause semantics so the
// session+lease are preserved) and records the trip in zdx_budget_pauses for
// audit. Lifting the pause is operator-driven (handlers_agent_budgets.go
// /budget/lift) — the watcher never auto-resumes.
package budgetwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// Default cost rates approximate Sonnet 4.6 USD/token (see
// handlers_agent_budgets.go for the operator-facing copy of the same numbers).
// A future task will move these to per-model config.
const (
	costPerInputToken         = 3.00 / 1_000_000
	costPerOutputToken        = 15.00 / 1_000_000
	costPerCacheReadToken     = 0.30 / 1_000_000
	costPerCacheCreationToken = 3.75 / 1_000_000
)

// DefaultPollInterval is the cadence the prod server runs the loop at.
const DefaultPollInterval = 30 * time.Second

// AgentLister returns the IDs of agents currently holding a live WS connection.
type AgentLister interface {
	ConnectedAgentIDs() []string
}

// AgentCommander dispatches a control message to a connected agent.
type AgentCommander interface {
	SendAgentCommand(ctx context.Context, agentID string, data []byte) error
}

// Querier is the narrow slice of *db.Queries the watcher actually uses; tests
// supply an in-memory fake.
type Querier interface {
	GetAgent(ctx context.Context, id string) (db.ZdxAgent, error)
	GetAgentBudget(ctx context.Context, agentID pgtype.Text) (db.ZdxAgentBudget, error)
	GetProjectBudget(ctx context.Context, projectID pgtype.Int4) (db.ZdxAgentBudget, error)
	GetAgentTokenUsage(ctx context.Context, agentID string) (db.GetAgentTokenUsageRow, error)
	GetActiveBudgetPause(ctx context.Context, agentID string) (db.ZdxBudgetPause, error)
	RecordBudgetPause(ctx context.Context, arg db.RecordBudgetPauseParams) (db.ZdxBudgetPause, error)
	UpdateAgentStatus(ctx context.Context, arg db.UpdateAgentStatusParams) error
}

// Watcher polls connected agents and trips budget pauses.
type Watcher struct {
	Q         Querier
	Registry  AgentLister
	Commander AgentCommander
	Interval  time.Duration
}

// Start runs Sweep on Interval until ctx is done. Returns immediately; runs in
// a goroutine. A first sweep fires before the ticker so a recent restart does
// not leave a runaway agent unobserved for a full interval.
func (w *Watcher) Start(ctx context.Context) {
	interval := w.Interval
	if interval <= 0 {
		interval = DefaultPollInterval
	}
	go func() {
		w.Sweep(ctx)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				w.Sweep(ctx)
			}
		}
	}()
}

// Sweep runs one pass over the connected-agent set.
func (w *Watcher) Sweep(ctx context.Context) {
	for _, agentID := range w.Registry.ConnectedAgentIDs() {
		if err := w.checkAgent(ctx, agentID); err != nil {
			log.Printf("budgetwatch: %s: %v", agentID, err)
		}
	}
}

// checkAgent evaluates one agent against its applicable ceiling and trips a
// pause if needed. Returns nil when no action is required.
func (w *Watcher) checkAgent(ctx context.Context, agentID string) error {
	// Skip if already paused — pause is idempotent and the lift handler is the
	// only path that may resume.
	if _, err := w.Q.GetActiveBudgetPause(ctx, agentID); err == nil {
		return nil
	}

	agent, err := w.Q.GetAgent(ctx, agentID)
	if err != nil {
		// Agent may have been reaped between Registry.List and this lookup; ignore.
		return nil
	}

	budget, ok := w.resolveBudget(ctx, agentID, agent.ProjectID)
	if !ok {
		return nil
	}

	usage, err := w.Q.GetAgentTokenUsage(ctx, agentID)
	if err != nil {
		return fmt.Errorf("rollup tokens: %w", err)
	}
	totalTokens := usage.InputTokens + usage.OutputTokens
	cost := computeCost(usage)

	reason, exceeded := budgetExceeded(budget, totalTokens, cost)
	if !exceeded {
		return nil
	}

	msg, _ := json.Marshal(map[string]string{"type": "pause"})
	if err := w.Commander.SendAgentCommand(ctx, agentID, msg); err != nil {
		// Agent is in registry but WS write failed — leave the trip unrecorded
		// so a later pass retries when the connection is healthy again. Without
		// the daemon actually receiving the pause, recording one would lie
		// about the runtime state.
		return fmt.Errorf("send pause: %w", err)
	}

	if _, err := w.Q.RecordBudgetPause(ctx, db.RecordBudgetPauseParams{
		AgentID:   agentID,
		ProjectID: pgtype.Int4{Int32: agent.ProjectID, Valid: true},
		Reason:    reason,
	}); err != nil {
		return fmt.Errorf("record pause: %w", err)
	}

	if err := w.Q.UpdateAgentStatus(ctx, db.UpdateAgentStatusParams{ID: agentID, Status: "paused"}); err != nil {
		log.Printf("budgetwatch: %s: update status to paused: %v", agentID, err)
	}

	log.Printf("budgetwatch: paused agent %s — %s", agentID, reason)
	return nil
}

// resolveBudget returns the per-agent ceiling if set, otherwise the
// per-project ceiling. ok=false when neither applies.
func (w *Watcher) resolveBudget(ctx context.Context, agentID string, projectID int32) (db.ZdxAgentBudget, bool) {
	if b, err := w.Q.GetAgentBudget(ctx, pgtype.Text{String: agentID, Valid: true}); err == nil {
		return b, true
	}
	if b, err := w.Q.GetProjectBudget(ctx, pgtype.Int4{Int32: projectID, Valid: true}); err == nil {
		return b, true
	}
	return db.ZdxAgentBudget{}, false
}

// budgetExceeded returns the canonical reason string and true when usage has
// crossed either ceiling. The format ("tokens:N/M" or "cost:N/M") matches what
// `dx agent budget list` already renders so operators see one vocabulary
// across CLI + audit table.
func budgetExceeded(b db.ZdxAgentBudget, tokens int64, cost float64) (string, bool) {
	if b.TokenCeiling.Valid && b.TokenCeiling.Int64 > 0 && tokens >= b.TokenCeiling.Int64 {
		return fmt.Sprintf("tokens:%d/%d", tokens, b.TokenCeiling.Int64), true
	}
	if b.CostCeiling.Valid && b.CostCeiling.Float64 > 0 && cost >= b.CostCeiling.Float64 {
		return fmt.Sprintf("cost:%.4f/%.4f", cost, b.CostCeiling.Float64), true
	}
	return "", false
}

func computeCost(u db.GetAgentTokenUsageRow) float64 {
	return float64(u.InputTokens)*costPerInputToken +
		float64(u.OutputTokens)*costPerOutputToken +
		float64(u.CacheReadInputTokens)*costPerCacheReadToken +
		float64(u.CacheCreationInputTokens)*costPerCacheCreationToken
}
