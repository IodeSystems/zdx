package handlers

import (
	"context"
	"net/http"
	"regexp"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// DeployRequestStore is the narrow db surface the deploy-request handler needs.
// *db.Queries satisfies it; tests pass a stub so they don't need a real Postgres.
type DeployRequestStore interface {
	GetProjectBySlug(ctx context.Context, slug string) (db.ZdxProject, error)
	GetEnvironment(ctx context.Context, arg db.GetEnvironmentParams) (db.GetEnvironmentRow, error)
	GetIssue(ctx context.Context, arg db.GetIssueParams) (db.ZdxIssue, error)
	CreateDeployRequest(ctx context.Context, arg db.CreateDeployRequestParams) (db.ZdxDeployRequest, error)
}

// DeployRequestItem is the JSON response shape for create-deploy-request.
type DeployRequestItem struct {
	ID        int32  `json:"id"`
	EnvSlug   string `json:"env_slug"`
	CommitSha string `json:"commit_sha"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// commitSHAPattern matches a hex-ish git sha (7–40 lowercase hex chars).
var commitSHAPattern = regexp.MustCompile(`^[a-f0-9]{7,40}$`)

func (h *Handler) deployRequestStore() DeployRequestStore {
	if h.Deps != nil && h.Deps.DeployRequestStore != nil {
		return h.Deps.DeployRequestStore
	}
	return h.Q
}

func (h *Handler) registerDeployRequestRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "create-deploy-request",
		Method:        http.MethodPost,
		Path:          "/api/dx/envs/{slug}/deploy-requests",
		DefaultStatus: http.StatusCreated,
	},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Body struct {
				CommitSha       string `json:"commit_sha"`
				Reason          string `json:"reason,omitempty"`
				BlockingIssueID string `json:"blocking_issue_id,omitempty"`
			}
		}) (*struct{ Body DeployRequestItem }, error) {
			store := h.deployRequestStore()

			// Auth gate. apiKeyMiddleware has already 401'd unauthenticated
			// callers and stamped role+project_scope on ctx. Viewers can't
			// kick off deploys; everyone else (user/admin) is a valid
			// requester for now.
			//
			// TODO(IS-1228): env-agent scoped tokens (scopes={deploy:apply,
			// schema:dump}) must be rejected here — they're the consumer, not
			// the requester. The middleware doesn't yet thread the token's
			// scope kind into ctx, so the rejection lands when that plumbing
			// does. Until then a plain admin token issued for the dx-envd
			// host could in theory POST here; that's acceptable in the gap
			// because admin tokens already have the keys to the kingdom.
			if keyID, _ := ctx.Value(CtxAPIKeyID).(int32); keyID == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			if role := UserRoleFromContext(ctx); role == "viewer" {
				return nil, apiErr(http.StatusForbidden, "deploy requests require a worker/operator/admin token")
			}

			// Body validation.
			if !commitSHAPattern.MatchString(in.Body.CommitSha) {
				return nil, apiErr(http.StatusUnprocessableEntity, "commit_sha must match ^[a-f0-9]{7,40}$")
			}

			// Resolve the env from the auth'd project. Slot-agent tokens carry
			// a single-project scope; admin/unscoped tokens are rejected here
			// because the URL alone can't disambiguate which project owns the
			// env name. (Tests/operators can scope a token to one project to
			// drive this endpoint.)
			scope := ProjectScopeFromContext(ctx)
			if len(scope) != 1 {
				return nil, apiErr(http.StatusForbidden, "deploy requests require a token scoped to exactly one project")
			}
			p, err := store.GetProjectBySlug(ctx, scope[0])
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+scope[0])
			}
			env, err := store.GetEnvironment(ctx, db.GetEnvironmentParams{ProjectID: p.ID, Name: in.Slug})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "environment not found: "+in.Slug)
			}

			// Optional blocking-issue ref. Must resolve in this project.
			var blocking pgtype.Text
			if in.Body.BlockingIssueID != "" {
				if _, err := store.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: in.Body.BlockingIssueID}); err != nil {
					return nil, apiErr(http.StatusUnprocessableEntity, "blocking_issue_id does not resolve in this project: "+in.Body.BlockingIssueID)
				}
				blocking = pgtype.Text{String: in.Body.BlockingIssueID, Valid: true}
			}

			userID := UserIDFromContext(ctx)
			var requester pgtype.Int4
			if userID != 0 {
				requester = pgtype.Int4{Int32: userID, Valid: true}
			}

			row, err := store.CreateDeployRequest(ctx, db.CreateDeployRequestParams{
				EnvID:             env.ID,
				CommitSha:         in.Body.CommitSha,
				RequestedByUserID: requester,
				Reason:            in.Body.Reason,
				BlockingIssueID:   blocking,
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}

			return &struct{ Body DeployRequestItem }{Body: DeployRequestItem{
				ID:        row.ID,
				EnvSlug:   env.Name,
				CommitSha: row.CommitSha,
				Status:    row.Status,
				CreatedAt: fmtTS(row.CreatedAt),
			}}, nil
		})
}
