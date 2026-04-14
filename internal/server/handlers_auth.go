package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerAuthRoutes(api huma.API) {
	// Health
	huma.Register(api, huma.Operation{OperationID: "health", Method: http.MethodGet, Path: "/api/health"},
		func(ctx context.Context, _ *struct{}) (*struct{ Body map[string]string }, error) {
			return &struct{ Body map[string]string }{Body: map[string]string{"status": "ok", "build_sha": s.buildSHA}}, nil
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
			}{ZdxProjectSlug: s.zdxProjectSlug}}, nil
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
			count, err := s.q.CountApiKeys(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if count > 0 {
				return nil, apiErr(409, "server already set up")
			}
			user, err := s.q.CreateUser(ctx, db.CreateUserParams{Email: in.Body.Email, Name: in.Body.Name})
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
			if _, err := s.q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: user.ID, Token: token, Name: keyName}); err != nil {
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
			inv, err := s.q.GetInviteByToken(ctx, in.Body.InviteCode)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "invalid or expired invite code")
			}
			inviter, err := s.q.GetUserByID(ctx, inv.InvitedBy)
			if err != nil {
				return nil, apiErr(500, "lookup inviter: "+err.Error())
			}
			role := inviter.Role
			hash, err := bcrypt.GenerateFromPassword([]byte(in.Body.Password), bcrypt.DefaultCost)
			if err != nil {
				return nil, apiErr(500, "hash password: "+err.Error())
			}
			user, err := s.q.CreateUserWithPassword(ctx, db.CreateUserWithPasswordParams{
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
			if _, err := s.q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: user.ID, Token: token, Name: "web"}); err != nil {
				return nil, apiErr(500, "create api key: "+err.Error())
			}
			_ = s.q.MarkInviteUsed(ctx, inv.ID)
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
			user, err := s.q.GetUserByEmail(ctx, in.Body.Email)
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
			if _, err := s.q.CreateApiKey(ctx, db.CreateApiKeyParams{UserID: user.ID, Token: token, Name: "web"}); err != nil {
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
			user, err := s.q.GetUserByID(ctx, uid)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "user not found")
			}
			return &struct{ Body MeItem }{Body: MeItem{ID: user.ID, Email: user.Email, Name: user.Name, Role: user.Role}}, nil
		})

	// ── Projects ─────────────────────────────────────────────────────────────

	type ProjectItem struct {
		ID        int32  `json:"id"`
		Slug      string `json:"slug"`
		Name      string `json:"name"`
		CreatedAt string `json:"created_at"`
		Stage     string `json:"stage"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-projects", Method: http.MethodGet, Path: "/api/projects"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Projects []ProjectItem `json:"projects"`
			}
		}, error) {
			rows, err := s.q.ListProjects(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ProjectItem, len(rows))
			for i, r := range rows {
				out[i] = ProjectItem{ID: r.ID, Slug: r.Slug, Name: r.Name, CreatedAt: fmtTS(r.CreatedAt), Stage: r.Stage}
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
				Slug string `json:"slug"`
				Name string `json:"name"`
			}
		}) (*struct{ Body ProjectItem }, error) {
			row, err := s.q.CreateProject(ctx, db.CreateProjectParams{Slug: in.Body.Slug, Name: in.Body.Name})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ProjectItem }{Body: ProjectItem{ID: row.ID, Slug: row.Slug, Name: row.Name, CreatedAt: fmtTS(row.CreatedAt)}}, nil
		})
}
