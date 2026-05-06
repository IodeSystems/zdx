package budgetwatch

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

type fakeQ struct {
	mu sync.Mutex

	agent           db.ZdxAgent
	agentBudget     db.ZdxAgentBudget
	hasAgentBudget  bool
	projectBudget   db.ZdxAgentBudget
	hasProjBudget   bool
	usage           db.GetAgentTokenUsageRow
	activePause     db.ZdxBudgetPause
	hasActivePause  bool
	recordedReason  string
	recordedAgentID string
	recordCount     int
	statusUpdates   []string
}

func (f *fakeQ) GetAgent(ctx context.Context, id string) (db.ZdxAgent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.agent.ID == "" {
		return db.ZdxAgent{}, errors.New("not found")
	}
	return f.agent, nil
}

func (f *fakeQ) GetAgentBudget(ctx context.Context, agentID pgtype.Text) (db.ZdxAgentBudget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hasAgentBudget {
		return db.ZdxAgentBudget{}, errors.New("not found")
	}
	return f.agentBudget, nil
}

func (f *fakeQ) GetProjectBudget(ctx context.Context, projectID pgtype.Int4) (db.ZdxAgentBudget, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hasProjBudget {
		return db.ZdxAgentBudget{}, errors.New("not found")
	}
	return f.projectBudget, nil
}

func (f *fakeQ) GetAgentTokenUsage(ctx context.Context, agentID string) (db.GetAgentTokenUsageRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.usage, nil
}

func (f *fakeQ) GetActiveBudgetPause(ctx context.Context, agentID string) (db.ZdxBudgetPause, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.hasActivePause {
		return db.ZdxBudgetPause{}, errors.New("no active pause")
	}
	return f.activePause, nil
}

func (f *fakeQ) RecordBudgetPause(ctx context.Context, arg db.RecordBudgetPauseParams) (db.ZdxBudgetPause, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recordedReason = arg.Reason
	f.recordedAgentID = arg.AgentID
	f.recordCount++
	f.hasActivePause = true
	f.activePause = db.ZdxBudgetPause{AgentID: arg.AgentID, Reason: arg.Reason}
	return f.activePause, nil
}

func (f *fakeQ) UpdateAgentStatus(ctx context.Context, arg db.UpdateAgentStatusParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statusUpdates = append(f.statusUpdates, arg.Status)
	return nil
}

type fakeRegistry struct{ ids []string }

func (f *fakeRegistry) ConnectedAgentIDs() []string { return f.ids }

type fakeCommander struct {
	mu       sync.Mutex
	commands []string
	errOnce  bool
}

func (f *fakeCommander) SendAgentCommand(ctx context.Context, agentID string, data []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.errOnce {
		f.errOnce = false
		return errors.New("ws write failed")
	}
	f.commands = append(f.commands, agentID+":"+string(data))
	return nil
}

func newWatcher(q Querier, r AgentLister, c AgentCommander) *Watcher {
	return &Watcher{Q: q, Registry: r, Commander: c}
}

func TestSweep_TripsAgentTokenCeiling(t *testing.T) {
	q := &fakeQ{
		agent:          db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		hasAgentBudget: true,
		agentBudget:    db.ZdxAgentBudget{TokenCeiling: pgtype.Int8{Int64: 1000, Valid: true}},
		usage:          db.GetAgentTokenUsageRow{InputTokens: 600, OutputTokens: 600},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{}
	newWatcher(q, reg, cmd).Sweep(context.Background())

	if len(cmd.commands) != 1 || !strings.Contains(cmd.commands[0], `"type":"pause"`) {
		t.Fatalf("expected one pause command, got %#v", cmd.commands)
	}
	if q.recordCount != 1 {
		t.Fatalf("expected 1 pause record, got %d", q.recordCount)
	}
	if !strings.HasPrefix(q.recordedReason, "tokens:1200/1000") {
		t.Fatalf("unexpected reason: %q", q.recordedReason)
	}
	if len(q.statusUpdates) != 1 || q.statusUpdates[0] != "paused" {
		t.Fatalf("expected status update to paused, got %v", q.statusUpdates)
	}
}

func TestSweep_IdempotentWhilePaused(t *testing.T) {
	q := &fakeQ{
		agent:          db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		hasAgentBudget: true,
		agentBudget:    db.ZdxAgentBudget{TokenCeiling: pgtype.Int8{Int64: 1000, Valid: true}},
		usage:          db.GetAgentTokenUsageRow{InputTokens: 5000, OutputTokens: 5000},
		hasActivePause: true,
		activePause:    db.ZdxBudgetPause{AgentID: "A1"},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{}
	w := newWatcher(q, reg, cmd)

	w.Sweep(context.Background())
	w.Sweep(context.Background())

	if len(cmd.commands) != 0 {
		t.Fatalf("expected no commands while pause is active, got %#v", cmd.commands)
	}
	if q.recordCount != 0 {
		t.Fatalf("expected 0 records while pause is active, got %d", q.recordCount)
	}
}

func TestSweep_NoBudget_NoOp(t *testing.T) {
	q := &fakeQ{
		agent: db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		usage: db.GetAgentTokenUsageRow{InputTokens: 999_999_999},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{}
	newWatcher(q, reg, cmd).Sweep(context.Background())

	if len(cmd.commands) != 0 || q.recordCount != 0 {
		t.Fatalf("expected no action without a configured budget")
	}
}

func TestSweep_ProjectBudgetCoversAgent(t *testing.T) {
	q := &fakeQ{
		agent:         db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		hasProjBudget: true,
		projectBudget: db.ZdxAgentBudget{TokenCeiling: pgtype.Int8{Int64: 500, Valid: true}},
		usage:         db.GetAgentTokenUsageRow{InputTokens: 400, OutputTokens: 200},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{}
	newWatcher(q, reg, cmd).Sweep(context.Background())

	if len(cmd.commands) != 1 {
		t.Fatalf("expected pause from project-level ceiling, got %#v", cmd.commands)
	}
	if !strings.HasPrefix(q.recordedReason, "tokens:600/500") {
		t.Fatalf("unexpected reason: %q", q.recordedReason)
	}
}

func TestSweep_AgentBudgetOverridesProject(t *testing.T) {
	q := &fakeQ{
		agent:          db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		hasAgentBudget: true,
		agentBudget:    db.ZdxAgentBudget{TokenCeiling: pgtype.Int8{Int64: 100_000, Valid: true}},
		hasProjBudget:  true,
		projectBudget:  db.ZdxAgentBudget{TokenCeiling: pgtype.Int8{Int64: 500, Valid: true}},
		usage:          db.GetAgentTokenUsageRow{InputTokens: 400, OutputTokens: 200},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{}
	newWatcher(q, reg, cmd).Sweep(context.Background())

	if len(cmd.commands) != 0 {
		t.Fatalf("agent ceiling should win — project ceiling must not pause this agent")
	}
}

func TestSweep_CostCeiling(t *testing.T) {
	q := &fakeQ{
		agent:          db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		hasAgentBudget: true,
		agentBudget:    db.ZdxAgentBudget{CostCeiling: pgtype.Float8{Float64: 0.01, Valid: true}},
		// 10000 input tokens × $3/MTok = $0.03 — well past $0.01 ceiling.
		usage: db.GetAgentTokenUsageRow{InputTokens: 10_000},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{}
	newWatcher(q, reg, cmd).Sweep(context.Background())

	if q.recordCount != 1 || !strings.HasPrefix(q.recordedReason, "cost:") {
		t.Fatalf("expected cost trip, got record=%d reason=%q", q.recordCount, q.recordedReason)
	}
}

func TestSweep_WSWriteFailureDoesNotRecord(t *testing.T) {
	q := &fakeQ{
		agent:          db.ZdxAgent{ID: "A1", ProjectID: pgtype.Int4{Int32: 7, Valid: true}},
		hasAgentBudget: true,
		agentBudget:    db.ZdxAgentBudget{TokenCeiling: pgtype.Int8{Int64: 100, Valid: true}},
		usage:          db.GetAgentTokenUsageRow{InputTokens: 1000},
	}
	reg := &fakeRegistry{ids: []string{"A1"}}
	cmd := &fakeCommander{errOnce: true}
	newWatcher(q, reg, cmd).Sweep(context.Background())

	if q.recordCount != 0 {
		t.Fatalf("must not record a pause when the WS write failed — agent never received the command")
	}
}
