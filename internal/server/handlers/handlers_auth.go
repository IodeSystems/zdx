package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerAuthRoutes(api huma.API) {
	// Health
	huma.Register(api, huma.Operation{OperationID: "health", Method: http.MethodGet, Path: "/api/health"},
		func(ctx context.Context, _ *struct{}) (*struct{ Body map[string]string }, error) {
			return &struct{ Body map[string]string }{Body: map[string]string{"status": "ok", "build_sha": h.BuildSHA}}, nil
		})

	// Config (authenticated)
	huma.Register(api, huma.Operation{OperationID: "get-config", Method: http.MethodGet, Path: "/api/config"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				ZdxProjectSlug string `json:"zdx_project_slug"`
			}
		}, error) {
			return &struct {
				Body struct {
					ZdxProjectSlug string `json:"zdx_project_slug"`
				}
			}{Body: struct {
				ZdxProjectSlug string `json:"zdx_project_slug"`
			}{ZdxProjectSlug: h.ZDXProjectSlug}}, nil
		})

	// ── Setup (unauthenticated, one-time bootstrap) ───────────────────────────

	huma.Register(api, huma.Operation{OperationID: "setup-bootstrap", Method: http.MethodPost, Path: "/api/setup/bootstrap"},
		func(ctx context.Context, in *struct {
			Body struct {
				Email   string  `json:"email"`
				Name    string  `json:"name"`
				KeyName *string `json:"key_name,omitempty"`
			}
		}) (*struct {
			Body struct {
				Token string `json:"token"`
				Email string `json:"email"`
			}
		}, error) {
			count, err := h.Q.CountApiKeys(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if count > 0 {
				return nil, apiErr(409, "server already set up")
			}
			user, err := h.Q.CreateUser(ctx, db.CreateUserParams{Email: in.Body.Email, Name: in.Body.Name})
			if err != nil {
				return nil, apiErr(500, "create user: "+err.Error())
			}
			var raw [32]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return nil, apiErr(500, "generate token: "+err.Error())
			}
			token := hex.EncodeToString(raw[:])
			keyName := "default"
			if in.Body.KeyName != nil && *in.Body.KeyName != "" {
				keyName = *in.Body.KeyName
			}
			if _, err := h.Q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: user.ID, Token: token, Name: keyName}); err != nil {
				return nil, apiErr(500, "create api key: "+err.Error())
			}
			return &struct {
				Body struct {
					Token string `json:"token"`
					Email string `json:"email"`
				}
			}{Body: struct {
				Token string `json:"token"`
				Email string `json:"email"`
			}{Token: token, Email: user.Email}}, nil
		})

	// ── Auth (unauthenticated) ────────────────────────────────────────────────

	type MeItem struct {
		ID    int32  `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}

	huma.Register(api, huma.Operation{OperationID: "auth-register", Method: http.MethodPost, Path: "/api/auth/register"},
		func(ctx context.Context, in *struct {
			Body struct {
				Email      string `json:"email"`
				Name       string `json:"name"`
				Password   string `json:"password"`
				InviteCode string `json:"invite_code"`
			}
		}) (*struct {
			Body struct {
				Token string `json:"token"`
				Email string `json:"email"`
				Role  string `json:"role"`
			}
		}, error) {
			inv, err := h.Q.GetInviteByToken(ctx, in.Body.InviteCode)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "invalid or expired invite code")
			}
			inviter, err := h.Q.GetUserByID(ctx, inv.InvitedBy)
			if err != nil {
				return nil, apiErr(500, "lookup inviter: "+err.Error())
			}
			role := inviter.Role
			hash, err := bcrypt.GenerateFromPassword([]byte(in.Body.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, apiErr(500, "hash password: "+err.Error())
			}
			user, err := h.Q.CreateUserWithPassword(ctx, db.CreateUserWithPasswordParams{
				Email:        in.Body.Email,
				Name:         in.Body.Name,
				PasswordHash: string(hash),
				Role:         role,
			})
			if err != nil {
				return nil, apiErr(409, "email already registered")
			}
			var raw [32]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return nil, apiErr(500, "generate token: "+err.Error())
			}
			token := hex.EncodeToString(raw[:])
			if _, err := h.Q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: user.ID, Token: token, Name: "web"}); err != nil {
				return nil, apiErr(500, "create api key: "+err.Error())
			}
			_ = h.Q.MarkInviteUsed(ctx, inv.ID)
			return &struct {
				Body struct {
					Token string `json:"token"`
					Email string `json:"email"`
					Role  string `json:"role"`
				}
			}{Body: struct {
				Token string `json:"token"`
				Email string `json:"email"`
				Role  string `json:"role"`
			}{Token: token, Email: user.Email, Role: user.Role}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "auth-login", Method: http.MethodPost, Path: "/api/auth/login"},
		func(ctx context.Context, in *struct {
			Body struct {
				Email    string `json:"email"`
				Password string `json:"password"`
			}
		}) (*struct {
			Body struct {
				Token string `json:"token"`
				Email string `json:"email"`
				Role  string `json:"role"`
			}
		}, error) {
			user, err := h.Q.GetUserByEmail(ctx, in.Body.Email)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "invalid credentials")
			}
			if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(in.Body.Password)); err != nil {
				return nil, apiErr(http.StatusUnauthorized, "invalid credentials")
			}
			var raw [32]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return nil, apiErr(500, "generate token: "+err.Error())
			}
			token := hex.EncodeToString(raw[:])
			if _, err := h.Q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: user.ID, Token: token, Name: "web"}); err != nil {
				return nil, apiErr(500, "create api key: "+err.Error())
			}
			return &struct {
				Body struct {
					Token string `json:"token"`
					Email string `json:"email"`
					Role  string `json:"role"`
				}
			}{Body: struct {
				Token string `json:"token"`
				Email string `json:"email"`
				Role  string `json:"role"`
			}{Token: token, Email: user.Email, Role: user.Role}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-me", Method: http.MethodGet, Path: "/api/me"},
		func(ctx context.Context, _ *struct{}) (*struct{ Body MeItem }, error) {
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			user, err := h.Q.GetUserByID(ctx, uid)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "user not found")
			}
			return &struct{ Body MeItem }{Body: MeItem{ID: user.ID, Email: user.Email, Name: user.Name, Role: user.Role}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "auth-cli-token", Method: http.MethodPost, Path: "/api/auth/cli-token"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Token string `json:"token"`
				Email string `json:"email"`
			}
		}, error) {
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			user, err := h.Q.GetUserByID(ctx, uid)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "user not found")
			}
			var raw [32]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return nil, apiErr(500, "generate token: "+err.Error())
			}
			token := hex.EncodeToString(raw[:])
			if _, err := h.Q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: uid, Token: token, Name: "cli"}); err != nil {
				return nil, apiErr(500, "create api key: "+err.Error())
			}
			return &struct {
				Body struct {
					Token string `json:"token"`
					Email string `json:"email"`
				}
			}{Body: struct {
				Token string `json:"token"`
				Email string `json:"email"`
			}{Token: token, Email: user.Email}}, nil
		})

	type ApiKeyItem struct {
		ID         int32  `json:"id"`
		Name       string `json:"name"`
		LastUsedAt string `json:"last_used_at,omitempty"`
		CreatedAt  string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-my-api-keys", Method: http.MethodGet, Path: "/api/me/api-keys"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Keys []ApiKeyItem `json:"keys"`
			}
		}, error) {
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			rows, err := h.Q.ListApiKeysByUser(ctx, uid)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ApiKeyItem, len(rows))
			for i, r := range rows {
				item := ApiKeyItem{ID: r.ID, Name: r.Name, CreatedAt: fmtTS(r.CreatedAt)}
				if r.LastUsedAt.Valid {
					item.LastUsedAt = fmtTS(r.LastUsedAt)
				}
				out[i] = item
			}
			return &struct {
				Body struct {
					Keys []ApiKeyItem `json:"keys"`
				}
			}{Body: struct {
				Keys []ApiKeyItem `json:"keys"`
			}{Keys: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-my-api-key", Method: http.MethodDelete, Path: "/api/me/api-keys/{id}"},
		func(ctx context.Context, in *struct {
			ID int32 `path:"id"`
		}) (*struct{}, error) {
			uid := ctxUserIDVal(ctx)
			if uid == 0 {
				return nil, apiErr(http.StatusUnauthorized, "not authenticated")
			}
			if err := h.Q.DeleteApiKey(ctx, db.DeleteApiKeyParams{ID: in.ID, UserID: uid}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return nil, nil
		})

	// ── Projects ─────────────────────────────────────────────────────────────

	type ProjectItem struct {
		ID             int32  `json:"id"`
		Slug           string `json:"slug"`
		Name           string `json:"name"`
		CreatedAt      string `json:"created_at"`
		Stage          string `json:"stage"`
		Classification string `json:"classification,omitempty"`
		GitEnabled     bool   `json:"git_enabled,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-projects", Method: http.MethodGet, Path: "/api/projects"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Projects []ProjectItem `json:"projects"`
			}
		}, error) {
			rows, err := h.Q.ListProjects(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ProjectItem, len(rows))
			for i, r := range rows {
				out[i] = ProjectItem{
					ID: r.ID, Slug: r.Slug, Name: r.Name, CreatedAt: fmtTS(r.CreatedAt),
					Stage: r.Stage, Classification: r.Classification, GitEnabled: r.GitEnabled,
				}
			}
			return &struct {
				Body struct {
					Projects []ProjectItem `json:"projects"`
				}
			}{Body: struct {
				Projects []ProjectItem `json:"projects"`
			}{Projects: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-project", Method: http.MethodPost, Path: "/api/project"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug                string `json:"slug"`
				Name                string `json:"name"`
				Classification      string `json:"classification,omitempty"`
				UpstreamURL         string `json:"upstream_url,omitempty"`
				UpstreamCredentials string `json:"upstream_credentials,omitempty"`
				LocalGit            bool   `json:"local_git,omitempty"`
			}
		}) (*struct{ Body ProjectItem }, error) {
			if in.Body.Classification != "" {
				switch in.Body.Classification {
				case "library", "tool", "service", "saas", "site":
				default:
					return nil, apiErr(http.StatusBadRequest, "classification must be one of: library, tool, service, saas, site")
				}
			}
			row, err := h.Q.CreateProject(ctx, db.CreateProjectParams{Slug: in.Body.Slug, Name: in.Body.Name})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if in.Body.Classification != "" {
				if err := h.Q.SetProjectClassification(ctx, db.SetProjectClassificationParams{
					ID:             row.ID,
					Classification: in.Body.Classification,
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			gitEnabled := false
			if in.Body.UpstreamURL != "" || in.Body.LocalGit {
				if err := h.Q.SetProjectProxyConfig(ctx, db.SetProjectProxyConfigParams{
					Slug:                row.Slug,
					UpstreamUrl:         in.Body.UpstreamURL,
					UpstreamCredentials: in.Body.UpstreamCredentials,
					GitEnabled:          true,
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
				gitEnabled = true
			}
			return &struct{ Body ProjectItem }{Body: ProjectItem{
				ID: row.ID, Slug: row.Slug, Name: row.Name, CreatedAt: fmtTS(row.CreatedAt),
				Classification: in.Body.Classification, GitEnabled: gitEnabled,
			}}, nil
		})
}
