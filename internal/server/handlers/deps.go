package handlers

import (
	"context"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
	"github.com/iodesystems/zdx-go/internal/zvec"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

// AgentCommander routes control messages to connected agent daemons via their
// live WebSocket connections (spec 179 — connection = liveness signal).
type AgentCommander interface {
	// SendAgentCommand writes raw JSON to the named agent's WS connection.
	// Returns a non-nil error if the agent is not connected.
	SendAgentCommand(ctx context.Context, agentID string, data []byte) error
}

// AgentConnRegistry provides live-connection lookups for the list-agents handler.
type AgentConnRegistry interface {
	// IsConnected reports whether the agent with the given ID has an active WS connection.
	IsConnected(agentID string) bool
	// ConnectedAt returns the wall-clock time the agent connected; zero value if not connected.
	ConnectedAt(agentID string) time.Time
}

type SchemaFeatures struct {
	HasLLMConfig        bool
	HasProjectGitConfig bool
	HasProjectStage     bool
}

type ReconcileResult struct {
	IssueID      string   `json:"issue_id"`
	ResolutionID string   `json:"resolution_id"`
	MatchedSHAs  []string `json:"matched_shas"`
	Source       string   `json:"source"`
}

type Broker interface {
	Publish(channel, eventType string, payload any)
	PublishIssue(slug, issueID, eventType string, payload any)
	PublishTask(slug, taskID, eventType string, payload any)
	PublishClaudeEvent(slug, sessionID, eventType string, payload any)
	PublishClaudeSessionLifecycle(slug, sessionID, eventType string, payload any)
	PublishAgentSessionLifecycle(slug, sessionID, eventType string, payload any)
	PublishAuditEvent(agentID string, payload any)
}

type Reconciler interface {
	ReconcileBranch(ctx context.Context, projectID int32, slug, targetBranch string) ([]ReconcileResult, error)
	RepoDir(slug string) string
}

type Embedder interface {
	UpsertIssue(ctx context.Context, projectID int32, issueID, text string)
	UpsertQuestion(ctx context.Context, projectID int32, questionID int32, text string)
	UpsertTask(ctx context.Context, projectID int32, taskID, text string)
	UpsertPattern(ctx context.Context, projectID int32, patternID int32, text string)
	UpsertProposal(ctx context.Context, projectID int32, proposalID int32, text string)
	UpsertFeature(ctx context.Context, projectID int32, featureID int32, text string)
	UpsertSpec(ctx context.Context, projectID int32, specID int32, text string)
	TopN(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNQuestions(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNTasks(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNPatterns(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNProposals(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNFeatures(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNSpecs(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	Complete(ctx context.Context, messages []llm.ChatMessage) (string, error)
	StreamComplete(ctx context.Context, messages []llm.ChatMessage, onDelta func(string)) (string, error)
	Reload(cfg *llm.Config)
}

// IngestRegistrar lets admin handlers re-register ingest routes and trigger
// embedder reloads without knowing the server internals.
type IngestRegistrar interface {
	ReindexAllIssues()
	ReloadEmbedder(ctx context.Context)
}

type Deps struct {
	Pool                    *pgxpool.Pool
	Q                       *db.Queries
	Features                SchemaFeatures
	Emb                     Embedder
	Broker                  Broker
	Reconciler              Reconciler
	IngestRegistrar         IngestRegistrar
	ErrorClient             *zdxclient.Client
	BuildSHA                string
	ZDXProjectSlug          string
	UploadsDir              string
	ReposDir                string
	Slot                    string
	WSSecret                string
	Mux                     chi.Router
	AgentCommander          AgentCommander
	AgentConnRegistry       AgentConnRegistry
	AgentDisconnectGraceSec int
}

type Handler struct {
	*Deps
}
