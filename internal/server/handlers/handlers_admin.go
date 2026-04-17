package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerAdminRoutes(api huma.API) {
	h.registerAdminLLMConfigRoutes(api)

	// ── Admin: project git config ─────────────────────────────────────────────

	type GitConfigBody struct {
		GitURL    string `json:"git_url"`
		GitBranch string `json:"git_branch"`
		GitToken  string `json:"git_token,omitempty"` // omitted in GET response
	}

	huma.Register(api, huma.Operation{OperationID: "get-project-git-config", Method: http.MethodGet, Path: "/api/admin/project-git-config"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct{ Body GitConfigBody }, error) {
			if !h.Features.HasProjectGitConfig {
				return &struct{ Body GitConfigBody }{Body: GitConfigBody{GitBranch: "main"}}, nil
			}
			row, err := h.Q.GetProjectGitConfig(ctx, in.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Slug)
			}
			return &struct{ Body GitConfigBody }{Body: GitConfigBody{
				GitURL:    row.GitUrl,
				GitBranch: row.GitBranch,
				// git_token intentionally omitted
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-project-git-config", Method: http.MethodPut, Path: "/api/admin/project-git-config"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				GitURL    string `json:"git_url"`
				GitBranch string `json:"git_branch"`
				GitToken  string `json:"git_token,omitempty"`
			}
		}) (*struct{ Body GitConfigBody }, error) {
			if !h.Features.HasProjectGitConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "git config schema not yet applied — run: dx migrate up")
			}
			if err := h.Q.SetProjectGitConfig(ctx, db.SetProjectGitConfigParams{
				Slug:      in.Body.Slug,
				GitUrl:    in.Body.GitURL,
				GitBranch: in.Body.GitBranch,
				GitToken:  in.Body.GitToken,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body GitConfigBody }{Body: GitConfigBody{
				GitURL:    in.Body.GitURL,
				GitBranch: in.Body.GitBranch,
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "test-project-git-config", Method: http.MethodPost, Path: "/api/admin/project-git-config/test"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				GitURL    string `json:"git_url"`
				GitBranch string `json:"git_branch"`
				GitToken  string `json:"git_token,omitempty"`
			}
		}) (*struct {
			Body struct {
				OK      bool   `json:"ok"`
				Message string `json:"message"`
			}
		}, error) {
			gitURL := in.Body.GitURL
			branch := in.Body.GitBranch
			token := in.Body.GitToken
			if token == "" && h.Features.HasProjectGitConfig && in.Body.Slug != "" {
				if row, err := h.Q.GetProjectGitConfig(ctx, in.Body.Slug); err == nil {
					token = row.GitToken
				}
			}
			if token != "" && strings.HasPrefix(gitURL, "https://") {
				gitURL = "https://" + token + "@" + strings.TrimPrefix(gitURL, "https://")
			}
			resp := &struct {
				Body struct {
					OK      bool   `json:"ok"`
					Message string `json:"message"`
				}
			}{}
			if err := GitLsRemote(gitURL, branch); err != nil {
				resp.Body.OK = false
				resp.Body.Message = err.Error()
				return resp, nil
			}
			resp.Body.OK = true
			resp.Body.Message = "ok"
			return resp, nil
		})

	// ── Admin: project stage ──────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "set-project-stage", Method: http.MethodPut, Path: "/api/admin/project-stage"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Stage string `json:"stage"`
			}
		}) (*struct {
			Body struct {
				Stage string `json:"stage"`
			}
		}, error) {
			if !h.Features.HasProjectStage {
				return nil, apiErr(http.StatusServiceUnavailable, "stage schema not yet applied — run: dx migrate up")
			}
			if err := h.Q.SetProjectStage(ctx, db.SetProjectStageParams{
				Stage: in.Body.Stage,
				Slug:  in.Body.Slug,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Stage string `json:"stage"`
				}
			}{Body: struct {
				Stage string `json:"stage"`
			}{Stage: in.Body.Stage}}, nil
		})

	// ── Admin: dashboard stats ──────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "admin-stats", Method: http.MethodGet, Path: "/api/admin/stats"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				UserCount    int `json:"user_count"`
				InviteCount  int `json:"invite_count"`
				ProjectCount int `json:"project_count"`
			}
		}, error) {
			users, _ := h.Q.ListUsers(ctx)
			invites, _ := h.Q.ListInvites(ctx)
			projects, _ := h.Q.ListProjects(ctx)
			return &struct {
				Body struct {
					UserCount    int `json:"user_count"`
					InviteCount  int `json:"invite_count"`
					ProjectCount int `json:"project_count"`
				}
			}{Body: struct {
				UserCount    int `json:"user_count"`
				InviteCount  int `json:"invite_count"`
				ProjectCount int `json:"project_count"`
			}{
				UserCount:    len(users),
				InviteCount:  len(invites),
				ProjectCount: len(projects),
			}}, nil
		})

	// ── Admin: users ─────────────────────────────────────────────────────────

	type AdminUserItem struct {
		ID        int32  `json:"id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-admin-users", Method: http.MethodGet, Path: "/api/admin/users"},
		func(ctx context.Context, in *struct {
			Q string `query:"q"`
		}) (*struct {
			Body struct {
				Users []AdminUserItem `json:"users"`
			}
		}, error) {
			var rows []db.ListUsersRow
			var err error
			if in.Q != "" {
				sRows, sErr := h.Q.SearchUsers(ctx, in.Q)
				if sErr != nil {
					return nil, apiErr(500, sErr.Error())
				}
				for _, r := range sRows {
					rows = append(rows, db.ListUsersRow(r))
				}
				err = nil
			} else {
				rows, err = h.Q.ListUsers(ctx)
			}
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]AdminUserItem, len(rows))
			for i, r := range rows {
				items[i] = AdminUserItem{
					ID:        r.ID,
					Email:     r.Email,
					Name:      r.Name,
					Role:      r.Role,
					CreatedAt: r.CreatedAt.Time.Format(time.RFC3339),
				}
			}
			return &struct {
				Body struct {
					Users []AdminUserItem `json:"users"`
				}
			}{Body: struct {
				Users []AdminUserItem `json:"users"`
			}{Users: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-user-role", Method: http.MethodPut, Path: "/api/admin/users/{id}/role"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Role string `json:"role"`
			}
		}) (*struct{}, error) {
			if in.Body.Role != "admin" && in.Body.Role != "user" && in.Body.Role != "viewer" {
				return nil, apiErr(http.StatusBadRequest, "role must be admin, user, or viewer")
			}
			if err := h.Q.UpdateUserRole(ctx, db.UpdateUserRoleParams{ID: in.ID, Role: in.Body.Role}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// ── Admin: invites ────────────────────────────────────────────────────────

	type InviteItem struct {
		ID        int32  `json:"id"`
		Email     string `json:"email"`
		Token     string `json:"token"`
		InvitedBy int32  `json:"invited_by"`
		ExpiresAt string `json:"expires_at"`
		UsedAt    string `json:"used_at,omitempty"`
		CreatedAt string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-invites", Method: http.MethodGet, Path: "/api/admin/invites"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Invites []InviteItem `json:"invites"`
			}
		}, error) {
			rows, err := h.Q.ListInvites(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]InviteItem, len(rows))
			for i, r := range rows {
				items[i] = InviteItem{
					ID:        r.ID,
					Email:     r.Email,
					Token:     r.Token,
					InvitedBy: r.InvitedBy,
					ExpiresAt: r.ExpiresAt.Time.Format(time.RFC3339),
					CreatedAt: r.CreatedAt.Time.Format(time.RFC3339),
				}
				if r.UsedAt.Valid {
					items[i].UsedAt = r.UsedAt.Time.Format(time.RFC3339)
				}
			}
			return &struct {
				Body struct {
					Invites []InviteItem `json:"invites"`
				}
			}{Body: struct {
				Invites []InviteItem `json:"invites"`
			}{Invites: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-invite", Method: http.MethodPost, Path: "/api/admin/invites"},
		func(ctx context.Context, in *struct {
			Body struct {
				Email string `json:"email"`
			}
		}) (*struct{ Body InviteItem }, error) {
			var raw [16]byte
			if _, err := rand.Read(raw[:]); err != nil {
				return nil, apiErr(500, "generate token: "+err.Error())
			}
			token := hex.EncodeToString(raw[:])
			inv, err := h.Q.CreateInvite(ctx, db.CreateInviteParams{
				Email:     in.Body.Email,
				Token:     token,
				InvitedBy: ctxUserIDVal(ctx),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := InviteItem{
				ID:        inv.ID,
				Email:     inv.Email,
				Token:     inv.Token,
				InvitedBy: inv.InvitedBy,
				ExpiresAt: inv.ExpiresAt.Time.Format(time.RFC3339),
				CreatedAt: inv.CreatedAt.Time.Format(time.RFC3339),
			}
			return &struct{ Body InviteItem }{Body: item}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-invite", Method: http.MethodDelete, Path: "/api/admin/invites/{id}"},
		func(ctx context.Context, in *struct {
			ID int32 `path:"id"`
		}) (*struct{}, error) {
			if err := h.Q.DeleteInvite(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	// ── Admin: integration tokens ─────────────────────────────────────────────

	type IntegrationTokenItem struct {
		ID          int32  `json:"id"`
		ProjectID   int32  `json:"project_id"`
		Component   string `json:"component,omitempty"`
		Name        string `json:"name"`
		TokenPrefix string `json:"token_prefix"`
		CreatedAt   string `json:"created_at"`
		RevokedAt   string `json:"revoked_at,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-integration-tokens", Method: http.MethodGet, Path: "/api/admin/integration-tokens"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug,omitempty"`
		}) (*struct {
			Body struct {
				Items []IntegrationTokenItem `json:"items"`
			}
		}, error) {
			pid := pgtype.Int4{Valid: false}
			if in.Slug != "" {
				p, err := getProject(ctx, h.Q, in.Slug)
				if err != nil {
					return nil, err
				}
				pid = pgtype.Int4{Int32: p.ID, Valid: true}
			}
			rows, err := h.Q.ListIntegrationTokens(ctx, pid)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]IntegrationTokenItem, len(rows))
			for i, r := range rows {
				it := IntegrationTokenItem{
					ID:          r.ID,
					ProjectID:   r.ProjectID,
					Name:        r.Name,
					TokenPrefix: r.TokenPrefix,
					CreatedAt:   r.CreatedAt.Time.Format(time.RFC3339),
				}
				if r.Component.Valid {
					it.Component = r.Component.String
				}
				if r.RevokedAt.Valid {
					it.RevokedAt = r.RevokedAt.Time.Format(time.RFC3339)
				}
				items[i] = it
			}
			return &struct {
				Body struct {
					Items []IntegrationTokenItem `json:"items"`
				}
			}{Body: struct {
				Items []IntegrationTokenItem `json:"items"`
			}{Items: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-integration-token", Method: http.MethodPost, Path: "/api/admin/integration-tokens"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				Component string `json:"component,omitempty"`
				Name      string `json:"name"`
			}
		}) (*struct {
			Body struct {
				IntegrationTokenItem
				Token string `json:"token"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			token, err := GenerateIntegrationToken()
			if err != nil {
				return nil, apiErr(500, "generate token: "+err.Error())
			}
			comp := pgtype.Text{Valid: false}
			if in.Body.Component != "" {
				comp = pgtype.Text{String: in.Body.Component, Valid: true}
			}
			row, err := h.Q.CreateIntegrationToken(ctx, db.CreateIntegrationTokenParams{
				ProjectID:   p.ID,
				Component:   comp,
				Name:        in.Body.Name,
				TokenHash:   HashIntegrationToken(token),
				TokenPrefix: TokenPrefix(token),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			item := IntegrationTokenItem{
				ID:          row.ID,
				ProjectID:   row.ProjectID,
				Name:        row.Name,
				TokenPrefix: row.TokenPrefix,
				CreatedAt:   row.CreatedAt.Time.Format(time.RFC3339),
			}
			if row.Component.Valid {
				item.Component = row.Component.String
			}
			return &struct {
				Body struct {
					IntegrationTokenItem
					Token string `json:"token"`
				}
			}{Body: struct {
				IntegrationTokenItem
				Token string `json:"token"`
			}{IntegrationTokenItem: item, Token: token}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "revoke-integration-token", Method: http.MethodPost, Path: "/api/admin/integration-tokens/{id}/revoke"},
		func(ctx context.Context, in *struct {
			ID int32 `path:"id"`
		}) (*struct{}, error) {
			if err := h.Q.RevokeIntegrationToken(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-integration-token", Method: http.MethodDelete, Path: "/api/admin/integration-tokens/{id}"},
		func(ctx context.Context, in *struct {
			ID int32 `path:"id"`
		}) (*struct{}, error) {
			if err := h.Q.DeleteIntegrationToken(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

}
