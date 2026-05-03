package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/doctor"
)

// AdminTokenStore narrows db.Queries to the calls used by admin token routes,
// so tests can inject a stub without spinning up a real database.
type AdminTokenStore interface {
	GetProjectBySlug(ctx context.Context, slug string) (db.ZdxProject, error)
	CreateApiKey(ctx context.Context, arg db.CreateApiKeyParams) (db.CreateApiKeyRow, error)
	AdminListApiKeys(ctx context.Context) ([]db.AdminListApiKeysRow, error)
	AdminDeleteApiKey(ctx context.Context, id int32) (int64, error)
	GetApiKeyProjectScope(ctx context.Context, id int32) ([]string, error)
	ListAdminScopedApiKeys(ctx context.Context) ([]db.ListAdminScopedApiKeysRow, error)
}

// AdminTokenItem is the JSON representation of an API key in admin views.
// Token is only set on POST responses; never returned by list.
type AdminTokenItem struct {
	ID           int32    `json:"id"`
	Name         string   `json:"name"`
	UserEmail    string   `json:"user_email,omitempty"`
	ProjectScope []string `json:"project_scope,omitempty"`
	LastUsedAt   string   `json:"last_used_at,omitempty"`
	CreatedAt    string   `json:"created_at"`
	Token        string   `json:"token,omitempty"`
}

func (h *Handler) adminTokenStore() AdminTokenStore {
	if h.Deps != nil && h.Deps.AdminTokenStore != nil {
		return h.Deps.AdminTokenStore
	}
	return h.Q
}

// requireUnscopedCallerKey enforces that the caller's API key is admin-unrestricted
// (project_scope IS NULL). Even an admin-role caller must hold an unscoped key —
// otherwise a leaked scoped token tied to an admin user could mint fresh admin tokens.
func requireUnscopedCallerKey(ctx context.Context, store AdminTokenStore) error {
	keyID, _ := ctx.Value(CtxAPIKeyID).(int32)
	if keyID == 0 {
		return apiErr(http.StatusUnauthorized, "not authenticated")
	}
	scope, err := store.GetApiKeyProjectScope(ctx, keyID)
	if err != nil {
		return apiErr(http.StatusInternalServerError, "lookup caller key: "+err.Error())
	}
	if scope != nil {
		return apiErr(http.StatusForbidden, "requires admin token (project_scope must be NULL)")
	}
	return nil
}

func (h *Handler) registerAdminTokenRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "create-admin-token", Method: http.MethodPost, Path: "/api/admin/tokens"},
		func(ctx context.Context, in *struct {
			Body struct {
				ProjectSlug string `json:"project_slug"`
				Name        string `json:"name,omitempty"`
			}
		}) (*struct{ Body AdminTokenItem }, error) {
			store := h.adminTokenStore()
			if err := requireUnscopedCallerKey(ctx, store); err != nil {
				return nil, err
			}
			if in.Body.ProjectSlug == "" {
				return nil, apiErr(http.StatusBadRequest, "project_slug is required")
			}
			p, err := store.GetProjectBySlug(ctx, in.Body.ProjectSlug)
			if err != nil {
				return nil, apiErr(http.StatusBadRequest, "project not found: "+in.Body.ProjectSlug)
			}
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			var raw [32]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return nil, apiErr(http.StatusInternalServerError, "generate token: "+err.Error())
			}
			token := hex.EncodeToString(raw[:])
			name := in.Body.Name
			if name == "" {
				name = "admin-token"
			}
			row, err := store.CreateApiKey(ctx, db.CreateApiKeyParams{
				UserID:       uid,
				Token:        token,
				Name:         name,
				ProjectScope: []string{p.Slug},
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body AdminTokenItem }{Body: AdminTokenItem{
				ID:           row.ID,
				Name:         row.Name,
				ProjectScope: row.ProjectScope,
				CreatedAt:    fmtTS(row.CreatedAt),
				Token:        row.Token,
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-admin-tokens", Method: http.MethodGet, Path: "/api/admin/tokens"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Tokens []AdminTokenItem `json:"tokens"`
			}
		}, error) {
			store := h.adminTokenStore()
			rows, err := store.AdminListApiKeys(ctx)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			out := make([]AdminTokenItem, len(rows))
			for i, r := range rows {
				item := AdminTokenItem{
					ID:           r.ID,
					Name:         r.Name,
					UserEmail:    r.UserEmail,
					ProjectScope: r.ProjectScope,
					CreatedAt:    fmtTS(r.CreatedAt),
				}
				if r.LastUsedAt.Valid {
					item.LastUsedAt = fmtTS(r.LastUsedAt)
				}
				out[i] = item
			}
			return &struct {
				Body struct {
					Tokens []AdminTokenItem `json:"tokens"`
				}
			}{Body: struct {
				Tokens []AdminTokenItem `json:"tokens"`
			}{Tokens: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-admin-token", Method: http.MethodDelete, Path: "/api/admin/tokens/{id}"},
		func(ctx context.Context, in *struct {
			ID int32 `path:"id"`
		}) (*struct{}, error) {
			store := h.adminTokenStore()
			n, err := store.AdminDeleteApiKey(ctx, in.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			if n == 0 {
				return nil, apiErr(http.StatusNotFound, "no such token")
			}
			return &struct{}{}, nil
		})

	// IS-841: doctor token-hygiene rung. Returns the names of legacy admin-
	// scoped agent-* tokens (rotate to project-scoped) and non-agent admin
	// tokens used in the trailing 7 days (still hot).
	huma.Register(api, huma.Operation{OperationID: "doctor-token-hygiene", Method: http.MethodGet, Path: "/api/admin/doctor/token-hygiene"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				LegacyAgentTokens []string `json:"legacy_agent_tokens"`
				RecentAdminTokens []string `json:"recent_admin_tokens"`
			}
		}, error) {
			store := h.adminTokenStore()
			rows, err := store.ListAdminScopedApiKeys(ctx)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			tokens := make([]doctor.AdminScopedToken, 0, len(rows))
			for _, r := range rows {
				tok := doctor.AdminScopedToken{ID: r.ID, Name: r.Name}
				if r.LastUsedAt.Valid {
					tok.LastUsedAt = r.LastUsedAt.Time
				}
				tokens = append(tokens, tok)
			}
			legacy, recent := doctor.PartitionAdminScopedTokens(tokens, time.Now())
			out := struct {
				Body struct {
					LegacyAgentTokens []string `json:"legacy_agent_tokens"`
					RecentAdminTokens []string `json:"recent_admin_tokens"`
				}
			}{}
			for _, t := range legacy {
				out.Body.LegacyAgentTokens = append(out.Body.LegacyAgentTokens, t.Name)
			}
			for _, t := range recent {
				out.Body.RecentAdminTokens = append(out.Body.RecentAdminTokens, t.Name)
			}
			return &out, nil
		})
}
