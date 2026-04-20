package handlers

import (
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
	"github.com/iodesystems/zdx-go/internal/zvec"
	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

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
	TopN(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNQuestions(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNTasks(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	TopNPatterns(ctx context.Context, projectID int32, query string, n int) ([]zvec.SearchResult, error)
	Complete(ctx context.Context, messages []llm.ChatMessage) (string, error)
	Reload(cfg *llm.Config)
}

// IngestRegistrar lets admin handlers re-register ingest routes and trigger
// embedder reloads without knowing the server internals.
type IngestRegistrar interface {
	ReindexAllIssues()
	ReloadEmbedder(ctx context.Context)
}

type Deps struct {
	Pool            *pgxpool.Pool
	Q               *db.Queries
	Features        SchemaFeatures
	Emb             Embedder
	Broker          Broker
	Reconciler      Reconciler
	IngestRegistrar IngestRegistrar
	ErrorClient     *zdxclient.Client
	BuildSHA        string
	ZDXProjectSlug  string
	UploadsDir      string
	ReposDir        string
	Slot            string
	WSSecret        string
	Mux             chi.Router
}

type Handler struct {
	*Deps
}
