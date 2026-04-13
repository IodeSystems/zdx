package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/crypto/bcrypt"

	"github.com/iodesystems/zdx-go/internal/db"
)

// ── ID conversion helpers ─────────────────────────────────────────────────

func intFromPrefixed(s, prefix string) int32 {
	if !strings.HasPrefix(s, prefix) {
		return 0
	}
	n, _ := strconv.ParseInt(s[len(prefix):], 10, 32)
	return int32(n)
}

func issueIntID(id string) int32 { return intFromPrefixed(id, "IS-") }
func taskIntID(id string) int32  { return intFromPrefixed(id, "TK-") }

func issueIDFromInt(n int32) string { return fmt.Sprintf("IS-%d", n) }
func taskIDFromInt(n int32) string  { return fmt.Sprintf("TK-%d", n) }

func fmtTS(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339)
}

// ── Response types ─────────────────────────────────────────────────────────

type IssueItem struct {
	ID        int32  `json:"id" doc:"Server integer ID; CLI formats as IS-N"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Priority  string `json:"priority"`
	Component string `json:"component"`
	Features  string `json:"features"`
	BlockedBy string `json:"blocked_by"`
	Context   string `json:"context"`
	Source    string `json:"source"`
	IssueType string `json:"issue_type"`
	CreatedAt string `json:"created_at"`
}

type TaskItem struct {
	ID          int32  `json:"id" doc:"Server integer ID; CLI formats as TK-N"`
	Text        string `json:"text"`
	Feature     string `json:"feature"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
	IssueID     *int32 `json:"issue_id,omitempty" doc:"Linked issue integer ID; CLI formats as IS-N"`
	Depends     string `json:"depends"`
	TestPlan    string `json:"test_plan"`
	TestRefs    string `json:"test_refs"`
	CreatedAt   string `json:"created_at"`
	CompletedAt string `json:"completed_at"`
}

type FeatureItem struct {
	ID          int32       `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	What        string      `json:"what"`
	Why         string      `json:"why"`
	DoneWhen    string      `json:"done_when"`
	Component   string      `json:"component"`
	Category    string      `json:"category"`
	PlanType    string      `json:"plan_type"`
	PlanStatus  string      `json:"plan_status"`
	HasTestRefs bool        `json:"has_test_refs"`
	Specs       []SpecItem  `json:"specs"`
}

type SpecItem struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
}

type ThemeItem struct {
	ID          int32  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Priority    int32  `json:"priority"`
	Status      string `json:"status"`
	Blockers    string `json:"blockers"`
	CreatedAt   string `json:"created_at"`
}

type TodoItem struct {
	ID         int32  `json:"id"`
	Text       string `json:"text"`
	Key        string `json:"key"`
	Persona    string `json:"persona"`
	Priority   int32  `json:"priority"`
	Status     string `json:"status"`
	CreatedAt  string `json:"created_at"`
	ResolvedAt string `json:"resolved_at"`
}

type ErrorReportItem struct {
	ID         int64  `json:"id"`
	Source     string `json:"source"`
	Endpoint   string `json:"endpoint"`
	ErrorName  string `json:"error_name"`
	StackTrace string `json:"stack_trace"`
	CreatedAt  string `json:"created_at"`
}

type SlowQueryItem struct {
	ID          int64  `json:"id"`
	SqlHash     string `json:"sql_hash"`
	SqlText     string `json:"sql_text"`
	Endpoint    string `json:"endpoint"`
	DurationMs  int32  `json:"duration_ms"`
	ExplainJson string `json:"explain_json"`
	CreatedAt   string `json:"created_at"`
}

type JournalEntryItem struct {
	Date          string `json:"date"`
	Baseline      bool   `json:"baseline"`
	Tldr          string `json:"tldr"`
	Assessment    string `json:"assessment"`
	Concerns      string `json:"concerns"`
	Next          string `json:"next"`
	ChangelogJSON string `json:"changelog_json"`
	StateJSON     string `json:"state_json"`
}

type IssueWorkItem struct {
	Agent     string `json:"agent"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type OKBody struct {
	OK bool `json:"ok"`
}

type CodeRefItem struct {
	ID        int32  `json:"id"`
	FilePath  string `json:"file_path"`
	GitHash   string `json:"git_hash"`
	LineStart int32  `json:"line_start"`
	LineEnd   int32  `json:"line_end"`
	Note      string `json:"note"`
	CreatedAt string `json:"created_at"`
}

type SimilarIssueItem struct {
	ID    string  `json:"id"`
	Title string  `json:"title"`
	Score float32 `json:"score"`
}

type QuestionItem struct {
	ID        int32  `json:"id"`
	Category  string `json:"category"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type WriteTodoInput struct {
	Text     string `json:"text"`
	Key      string `json:"key"`
	Persona  string `json:"persona"`
	Priority int32  `json:"priority"`
	Status   string `json:"status"`
}

type TestResultInput struct {
	Driver     string `json:"driver"`
	TestName   string `json:"test_name"`
	Feature    string `json:"feature"`
	Status     string `json:"status"`
	DurationMS int32  `json:"duration_ms"`
}

// ptrStr dereferences a *string, returning "" for nil.
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ptrInt32 dereferences a *int32, returning 0 for nil.
func ptrInt32(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
}

// ── Route registration ─────────────────────────────────────────────────────

func (s *Server) registerRoutes(api huma.API) {
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
			role, err := s.q.GetApiKeyUserRole(ctx, in.Body.InviteCode)
			if err != nil {
				return nil, apiErr(http.StatusUnauthorized, "invalid invite code")
			}
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
	}

	huma.Register(api, huma.Operation{OperationID: "list-projects", Method: http.MethodGet, Path: "/api/projects"},
		func(ctx context.Context, _ *struct{}) (*struct{ Body struct{ Projects []ProjectItem `json:"projects"` } }, error) {
			rows, err := s.q.ListProjects(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ProjectItem, len(rows))
			for i, r := range rows {
				out[i] = ProjectItem{ID: r.ID, Slug: r.Slug, Name: r.Name, CreatedAt: fmtTS(r.CreatedAt)}
			}
			return &struct{ Body struct{ Projects []ProjectItem `json:"projects"` } }{Body: struct{ Projects []ProjectItem `json:"projects"` }{Projects: out}}, nil
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

	// ── Issues ──────────────────────────────────────────────────────────────

	type IssueSlugInput struct{ Slug string `query:"slug" required:"true"` }
	type IssueIntIDInput struct{ ID int32 `json:"id"` }

	huma.Register(api, huma.Operation{OperationID: "list-issues", Method: http.MethodGet, Path: "/api/dx/todo/issue/list"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{ Body struct{ Issues []IssueItem `json:"issues"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListIssues(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]IssueItem, len(rows))
			for i, r := range rows {
				out[i] = toIssueItem(r)
			}
			return &struct{ Body struct{ Issues []IssueItem `json:"issues"` } }{Body: struct{ Issues []IssueItem `json:"issues"` }{Issues: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "search-issues", Method: http.MethodGet, Path: "/api/dx/todo/issue/search"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Q    string `query:"q" required:"true"`
		}) (*struct{ Body struct{ Issues []IssueItem `json:"issues"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.SearchIssues(ctx, db.SearchIssuesParams{ProjectID: p.ID, Query: in.Q})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]IssueItem, len(rows))
			for i, r := range rows {
				out[i] = toIssueItem(r)
			}
			return &struct{ Body struct{ Issues []IssueItem `json:"issues"` } }{
				Body: struct{ Issues []IssueItem `json:"issues"` }{Issues: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "show-issue", Method: http.MethodGet, Path: "/api/dx/todo/issue/show"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct {
			Body struct {
				Issue IssueItem       `json:"issue"`
				Work  []IssueWorkItem `json:"work"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(intFromPrefixed(in.ID, "IS-"))
			row, err := s.q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found: "+in.ID)
			}
			work, _ := s.q.GetIssueWork(ctx, issueID)
			workItems := make([]IssueWorkItem, len(work))
			for i, w := range work {
				workItems[i] = IssueWorkItem{Agent: w.Agent, Note: w.Note, CreatedAt: fmtTS(w.CreatedAt)}
			}
			type respBody = struct {
				Issue IssueItem       `json:"issue"`
				Work  []IssueWorkItem `json:"work"`
			}
			return &struct{ Body respBody }{Body: respBody{Issue: toIssueItem(row), Work: workItems}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug          string  `json:"slug"`
				Title         *string `json:"title,omitempty"`
				Source        *string `json:"source,omitempty"`
				Context       *string `json:"context,omitempty"`
				BlockedBy     *string `json:"blocked_by,omitempty"`
				Component     *string `json:"component,omitempty"`
				IssueType     *string `json:"issue_type,omitempty"`
				ScreenshotIDs []int32 `json:"screenshot_ids,omitempty"`
			}
		}) (*struct{ Body IssueItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			id, err := s.q.NextIssueID(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			params := db.CreateIssueParams{
				ID:        id,
				ProjectID: p.ID,
				IssueType: "ops",
			}
			if in.Body.Title != nil {
				params.Title = *in.Body.Title
			}
			if in.Body.Context != nil {
				params.Context = *in.Body.Context
			}
			if in.Body.Component != nil {
				params.Component = *in.Body.Component
			}
			if in.Body.IssueType != nil {
				params.IssueType = *in.Body.IssueType
			}
			row, err := s.q.CreateIssue(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, fid := range in.Body.ScreenshotIDs {
				_ = s.q.AttachFileToIssue(ctx, db.AttachFileToIssueParams{
					IssueID: row.ID,
					FileID:  fid,
					Kind:    "screenshot",
				})
			}
			go s.emb.upsertIssue(context.Background(), p.ID, row.ID, row.Title+" "+row.Context)
			return &struct{ Body IssueItem }{Body: toIssueItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "triage-issue", Method: http.MethodPost, Path: "/api/dx/todo/owner/triage"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				ID        int32   `json:"id"`
				Priority  int32   `json:"priority"`
				Title     *string `json:"title,omitempty"`
				IssueType *string `json:"issue_type,omitempty"`
				Context   *string `json:"context,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			agent := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, uErr := s.q.GetUserByID(ctx, uid); uErr == nil {
					agent = u.Email
				}
			}
			// Capture old values for revision log
			oldIssue, _ := s.q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID})
			newPriority := strconv.Itoa(int(in.Body.Priority))
			if err := s.q.SetIssuePriority(ctx, db.SetIssuePriorityParams{
				ID:        issueID,
				Priority:  newPriority,
				ProjectID: p.ID,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			if oldIssue.Priority != newPriority {
				s.recordRevision(ctx, p.ID, "issue", issueID, "priority", oldIssue.Priority, newPriority, agent)
			}
			for field, val := range map[string]*string{
				"title":      in.Body.Title,
				"issue_type": in.Body.IssueType,
				"context":    in.Body.Context,
			} {
				if val != nil && *val != "" {
					if err := s.q.SetIssueField(ctx, db.SetIssueFieldParams{
						Field:     field,
						Value:     *val,
						ProjectID: p.ID,
						ID:        issueID,
					}); err != nil {
						return nil, apiErr(500, err.Error())
					}
					oldVal := ""
					switch field {
					case "title":
						oldVal = oldIssue.Title
					case "issue_type":
						oldVal = oldIssue.IssueType
					case "context":
						oldVal = oldIssue.Context
					}
					s.recordRevision(ctx, p.ID, "issue", issueID, field, oldVal, *val, agent)
				}
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "close-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/close"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string  `json:"slug"`
				ID     int32   `json:"id"`
				Reason *string `json:"reason,omitempty"`
				Notes  *string `json:"notes,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			agent := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, uErr := s.q.GetUserByID(ctx, uid); uErr == nil {
					agent = u.Email
				}
			}
			if err := s.q.CloseIssue(ctx, db.CloseIssueParams{ProjectID: p.ID, ID: issueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordRevision(ctx, p.ID, "issue", issueID, "status", "open", "closed", agent)
			reason := ptrStr(in.Body.Reason)
			notes := ptrStr(in.Body.Notes)
			if reason != "" || notes != "" {
				note := reason
				if notes != "" {
					note += "\n" + notes
				}
				_ = s.q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: issueID, Agent: agent, Note: note})
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reopen-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/reopen"},
		func(ctx context.Context, in *struct {
			Body IssueIntIDInput
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			_ = s.q.ReopenIssue(ctx, db.ReopenIssueParams{ID: issueID, ProjectID: 0})
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID    int32  `json:"id"`
				Field string `json:"field"`
				Value string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			err := s.q.SetIssueField(ctx, db.SetIssueFieldParams{
				Field: in.Body.Field,
				Value: in.Body.Value,
				ID:    issueID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "issue-kind", Method: http.MethodPost, Path: "/api/dx/todo/issue/kind"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID   int32  `json:"id"`
				Kind string `json:"kind"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			err := s.q.SetIssueField(ctx, db.SetIssueFieldParams{
				Field: "component",
				Value: in.Body.Kind,
				ID:    issueID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "issue-set-blocked-by", Method: http.MethodPost, Path: "/api/dx/todo/issue/set-blocked-by"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID        int32  `json:"id"`
				BlockedBy string `json:"blocked_by"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			err := s.q.SetIssueField(ctx, db.SetIssueFieldParams{
				Field: "blocked_by",
				Value: in.Body.BlockedBy,
				ID:    issueID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// set-features is a no-op on the Go side — features are linked via task.issue
	huma.Register(api, huma.Operation{OperationID: "issue-set-features", Method: http.MethodPost, Path: "/api/dx/todo/issue/set-features"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID       int32  `json:"id"`
				Features string `json:"features"`
			}
		}) (*struct{ Body OKBody }, error) {
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// Issue work
	huma.Register(api, huma.Operation{OperationID: "append-issue-work", Method: http.MethodPost, Path: "/api/issue-work"},
		func(ctx context.Context, in *struct {
			Body struct {
				IssueID   int32  `json:"issue_id"`
				EntryType string `json:"entry_type"`
				ByRole    string `json:"by_role"`
				Note      string `json:"note"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.IssueID)
			note := in.Body.Note
			if in.Body.EntryType != "" {
				note = "[" + in.Body.EntryType + "] " + note
			}
			if err := s.q.AppendIssueWork(ctx, db.AppendIssueWorkParams{
				IssueID: issueID,
				Agent:   in.Body.ByRole,
				Note:    note,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-worklog", Method: http.MethodGet, Path: "/api/dx/worklog"},
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Entries []struct {
					IssueID   string `json:"issue_id"`
					Agent     string `json:"agent"`
					Note      string `json:"note"`
					CreatedAt string `json:"created_at"`
				} `json:"entries"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListWorklogForProject(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			type entry = struct {
				IssueID   string `json:"issue_id"`
				Agent     string `json:"agent"`
				Note      string `json:"note"`
				CreatedAt string `json:"created_at"`
			}
			type respBody = struct {
				Entries []entry `json:"entries"`
			}
			out := make([]entry, len(rows))
			for i, r := range rows {
				out[i] = entry{IssueID: r.IssueID, Agent: r.Agent, Note: r.Note, CreatedAt: fmtTS(r.CreatedAt)}
			}
			return &struct{ Body respBody }{Body: respBody{Entries: out}}, nil
		})

	// ── Tasks ────────────────────────────────────────────────────────────────

	type TasksSlugOutput = struct {
		Body struct {
			Tasks []TaskItem `json:"tasks"`
		}
	}

	huma.Register(api, huma.Operation{OperationID: "list-tasks", Method: http.MethodGet, Path: "/api/tasks"},
		func(ctx context.Context, in *IssueSlugInput) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListTasks(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(r)
			}
			return &TasksSlugOutput{Body: struct{ Tasks []TaskItem `json:"tasks"` }{Tasks: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-tasks-by-feature", Method: http.MethodGet, Path: "/api/tasks-by-feature"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			Feature string `query:"feature" required:"true"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListTasksByFeature(ctx, db.ListTasksByFeatureParams{ProjectID: p.ID, Feature: in.Feature})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(r)
			}
			return &TasksSlugOutput{Body: struct{ Tasks []TaskItem `json:"tasks"` }{Tasks: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-tasks-for-issue", Method: http.MethodGet, Path: "/api/dx/todo/issue/tasks"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			IssueID string `query:"issue_id" required:"true"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListTasksByIssue(ctx, db.ListTasksByIssueParams{ProjectID: p.ID, Issue: in.IssueID})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(r)
			}
			return &TasksSlugOutput{Body: struct{ Tasks []TaskItem `json:"tasks"` }{Tasks: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-task", Method: http.MethodPost, Path: "/api/dx/todo/tech/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string  `json:"slug"`
				Feature *string `json:"feature,omitempty"`
				Text    string  `json:"text"`
				Issue   *string `json:"issue,omitempty"`
				Depends *string `json:"depends,omitempty"`
			}
		}) (*struct{ Body TaskItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			id, err := s.q.NextTaskID(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			row, err := s.q.CreateTask(ctx, db.CreateTaskParams{
				ID:        id,
				ProjectID: p.ID,
				Text:      in.Body.Text,
				Feature:   ptrStr(in.Body.Feature),
				Issue:     ptrStr(in.Body.Issue),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body TaskItem }{Body: toTaskItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-task-done", Method: http.MethodPost, Path: "/api/dx/todo/dev/done"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID       int32   `json:"id"`
				TestPlan *string `json:"test_plan,omitempty"`
				TestRefs *string `json:"test_refs,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			if err := s.q.MarkTaskDone(ctx, db.MarkTaskDoneParams{
				ID:       id,
				TestPlan: ptrStr(in.Body.TestPlan),
				TestRefs: ptrStr(in.Body.TestRefs),
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-task-undone", Method: http.MethodPost, Path: "/api/dx/todo/dev/undone"},
		func(ctx context.Context, in *struct {
			Body struct{ ID int32 `json:"id"` }
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.MarkTaskUndone(ctx, taskIDFromInt(in.Body.ID)); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "block-task", Method: http.MethodPost, Path: "/api/dx/todo/dev/block"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID     int32   `json:"id"`
				Reason *string `json:"reason,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     taskIDFromInt(in.Body.ID),
				Status: "blocked",
				Reason: ptrStr(in.Body.Reason),
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "unblock-task", Method: http.MethodPost, Path: "/api/dx/todo/dev/unblock"},
		func(ctx context.Context, in *struct {
			Body struct{ ID int32 `json:"id"` }
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     taskIDFromInt(in.Body.ID),
				Status: "pending",
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// PUT /api/task-status — generic status update fallback
	huma.Register(api, huma.Operation{OperationID: "update-task-status", Method: http.MethodPut, Path: "/api/task-status"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID     int32   `json:"id"`
				Status string  `json:"status"`
				Reason *string `json:"reason,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     taskIDFromInt(in.Body.ID),
				Status: in.Body.Status,
				Reason: ptrStr(in.Body.Reason),
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-task", Method: http.MethodDelete, Path: "/api/task"},
		func(ctx context.Context, in *struct {
			Body struct{ ID int32 `json:"id"` }
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.DeleteTask(ctx, taskIDFromInt(in.Body.ID)); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// Commit record (stub — not stored, just acknowledged)
	huma.Register(api, huma.Operation{OperationID: "add-task-commit", Method: http.MethodPost, Path: "/api/dx/todo/dev/commit"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID   int32  `json:"id"`
				SHA  string `json:"sha"`
				Note string `json:"note"`
			}
		}) (*struct{ Body OKBody }, error) {
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-task-commit-refs", Method: http.MethodGet, Path: "/api/dx/todo/dev/commit-refs"},
		func(ctx context.Context, in *struct {
			ID int32 `query:"id" required:"true"`
		}) (*struct{ Body struct{ CommitRefs string `json:"commit_refs"` } }, error) {
			return &struct{ Body struct{ CommitRefs string `json:"commit_refs"` } }{}, nil
		})

	// ── Features ─────────────────────────────────────────────────────────────

	type FeaturesOutput = struct {
		Body struct {
			Features []FeatureItem `json:"features"`
		}
	}

	// /api/dx/todo/list — feature list with specs and plan (used by CLI todo queue)
	huma.Register(api, huma.Operation{OperationID: "list-features-todo", Method: http.MethodGet, Path: "/api/dx/todo/list"},
		func(ctx context.Context, in *IssueSlugInput) (*FeaturesOutput, error) {
			return s.featuresWithSpecs(ctx, in.Slug)
		})

	// /api/features — same data, used by removeFeature lookup
	huma.Register(api, huma.Operation{OperationID: "list-features", Method: http.MethodGet, Path: "/api/features"},
		func(ctx context.Context, in *IssueSlugInput) (*FeaturesOutput, error) {
			return s.featuresWithSpecs(ctx, in.Slug)
		})

	huma.Register(api, huma.Operation{OperationID: "get-feature", Method: http.MethodGet, Path: "/api/dx/feature"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Name string `query:"name" required:"true"`
		}) (*struct{ Body FeatureItem }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			feat, err := s.q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Name})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found: "+in.Name)
			}
			specs, _ := s.q.ListSpecs(ctx, feat.ID)
			return &struct{ Body FeatureItem }{Body: toFeatureItem(feat, specs)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "upsert-feature", Method: http.MethodPost, Path: "/api/feature"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
			}
		}) (*struct{ Body FeatureItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.UpsertFeature(ctx, db.UpsertFeatureParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body FeatureItem }{Body: toFeatureItem(row, nil)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-feature", Method: http.MethodDelete, Path: "/api/feature"},
		func(ctx context.Context, in *struct {
			Body struct{ ID int32 `json:"id"` }
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.DeleteFeature(ctx, in.Body.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-feature-field", Method: http.MethodPost, Path: "/api/dx/features/field"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				Field   string `json:"field"`
				Value   string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateFeatureField(ctx, db.UpdateFeatureFieldParams{
				ProjectID: p.ID,
				Name:      in.Body.Feature,
				Field:     in.Body.Field,
				Value:     in.Body.Value,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-specs", Method: http.MethodPost, Path: "/api/dx/specs/update"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				Field   string `json:"field"`
				Value   string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			f, err := s.q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found")
			}
			// field is the spec kind (unit_test, api_test, ui_test); value is description
			_, err = s.q.AddSpec(ctx, db.AddSpecParams{
				FeatureID:   f.ID,
				Description: in.Body.Value,
				Kind:        in.Body.Field,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Plans ─────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "create-plan", Method: http.MethodPost, Path: "/api/dx/plan/create"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				Feature    string `json:"feature"`
				PlanType   string `json:"plan_type"`
				Complexity string `json:"complexity"`
				Approach   string `json:"approach"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			f, err := s.q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: in.Body.Feature})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "feature not found")
			}
			_, err = s.q.UpsertPlan(ctx, db.UpsertPlanParams{
				FeatureID:  f.ID,
				PlanType:   in.Body.PlanType,
				Complexity: in.Body.Complexity,
				Approach:   in.Body.Approach,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Themes ────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "list-themes", Method: http.MethodGet, Path: "/api/dx/themes"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{ Body struct{ Themes []ThemeItem `json:"themes"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListThemes(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ThemeItem, len(rows))
			for i, r := range rows {
				blockers, _ := r.Blockers.(string)
				out[i] = ThemeItem{
					ID:          r.ID,
					Name:        r.Name,
					Description: r.Description,
					Priority:    r.Priority,
					Status:      r.Status,
					Blockers:    blockers,
					CreatedAt:   fmtTS(r.CreatedAt),
				}
			}
			return &struct{ Body struct{ Themes []ThemeItem `json:"themes"` } }{Body: struct{ Themes []ThemeItem `json:"themes"` }{Themes: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-theme", Method: http.MethodPost, Path: "/api/dx/themes/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Priority    int32  `json:"priority"`
				Blockers    string `json:"blockers"`
			}
		}) (*struct{ Body ThemeItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.CreateTheme(ctx, db.CreateThemeParams{
				ProjectID:   p.ID,
				Name:        in.Body.Name,
				Description: in.Body.Description,
				Priority:    in.Body.Priority,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ThemeItem }{Body: ThemeItem{
				ID:          row.ID,
				Name:        row.Name,
				Description: row.Description,
				Priority:    row.Priority,
				Status:      row.Status,
				CreatedAt:   fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-theme-status", Method: http.MethodPost, Path: "/api/dx/themes/status"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				Theme  string `json:"theme"` // "TH-N" or name
				Status string `json:"status"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			theme, err := s.resolveTheme(ctx, p.ID, in.Body.Theme)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateThemeStatus(ctx, db.UpdateThemeStatusParams{
				ProjectID: p.ID,
				ID:        theme.ID,
				Status:    in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-theme-blocker", Method: http.MethodPost, Path: "/api/dx/themes/block"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Theme string `json:"theme"`
				Issue string `json:"issue"` // "IS-N"
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			theme, err := s.resolveTheme(ctx, p.ID, in.Body.Theme)
			if err != nil {
				return nil, err
			}
			if err := s.q.AddThemeBlocker(ctx, db.AddThemeBlockerParams{
				ThemeID: theme.ID,
				IssueID: in.Body.Issue,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "remove-theme-blocker", Method: http.MethodPost, Path: "/api/dx/themes/unblock"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Theme string `json:"theme"`
				Issue string `json:"issue"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			theme, err := s.resolveTheme(ctx, p.ID, in.Body.Theme)
			if err != nil {
				return nil, err
			}
			if err := s.q.RemoveThemeBlocker(ctx, db.RemoveThemeBlockerParams{
				ThemeID: theme.ID,
				IssueID: in.Body.Issue,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── State ─────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "get-state", Method: http.MethodGet, Path: "/api/dx/state"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Key  string `query:"key" required:"true"`
		}) (*struct{ Body struct{ Value string `json:"value"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			val, err := s.q.GetState(ctx, db.GetStateParams{ProjectID: p.ID, Key: in.Key})
			if err != nil {
				val = ""
			}
			return &struct{ Body struct{ Value string `json:"value"` } }{Body: struct{ Value string `json:"value"` }{Value: val}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-state", Method: http.MethodPost, Path: "/api/dx/state"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Key   string `json:"key"`
				Value string `json:"value"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.SetState(ctx, db.SetStateParams{
				ProjectID: p.ID,
				Key:       in.Body.Key,
				Value:     in.Body.Value,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Todos ─────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "list-todos", Method: http.MethodGet, Path: "/api/dx/todos"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{ Body struct{ Todos []TodoItem `json:"todos"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListTodos(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TodoItem, len(rows))
			for i, r := range rows {
				out[i] = toTodoItem(r)
			}
			return &struct{ Body struct{ Todos []TodoItem `json:"todos"` } }{Body: struct{ Todos []TodoItem `json:"todos"` }{Todos: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "write-todos", Method: http.MethodPost, Path: "/api/dx/todos"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string `json:"slug"`
				Todos []WriteTodoInput `json:"todos"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.DeleteTodosForProject(ctx, p.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			for _, t := range in.Body.Todos {
				status := t.Status
				if status == "" {
					status = "open"
				}
				_, err := s.q.CreateTodo(ctx, db.CreateTodoParams{
					ProjectID: p.ID,
					Text:      t.Text,
					Key:       t.Key,
					Persona:   t.Persona,
					Priority:  t.Priority,
					Status:    status,
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Test results ──────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "submit-test-results", Method: http.MethodPost, Path: "/api/dx/test-results/submit"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				Results []TestResultInput `json:"results"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			for _, r := range in.Body.Results {
				_ = s.q.UpsertTestResult(ctx, db.UpsertTestResultParams{
					ProjectID:  p.ID,
					Driver:     r.Driver,
					TestName:   r.TestName,
					Feature:    r.Feature,
					Status:     r.Status,
					DurationMs: r.DurationMS,
				})
				_ = s.q.InsertTestResultHistory(ctx, db.InsertTestResultHistoryParams{
					ProjectID:  p.ID,
					Driver:     r.Driver,
					TestName:   r.TestName,
					Feature:    r.Feature,
					Status:     r.Status,
					DurationMs: r.DurationMS,
				})
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Journal ───────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "journal-checkin", Method: http.MethodPost, Path: "/api/dx/journal/checkin"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				Role       string `json:"role"`
				Date       string `json:"date"`
				Tldr       string `json:"tldr"`
				Assessment string `json:"assessment"`
				Concerns   string `json:"concerns"`
				Next       string `json:"next"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_, err = s.q.InsertJournalEntry(ctx, db.InsertJournalEntryParams{
				ProjectID:  p.ID,
				Role:       in.Body.Role,
				Date:       in.Body.Date,
				Tldr:       in.Body.Tldr,
				Assessment: in.Body.Assessment,
				Concerns:   in.Body.Concerns,
				Next:       in.Body.Next,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "journal-show", Method: http.MethodGet, Path: "/api/dx/journal/show"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Role string `query:"role" required:"true"`
		}) (*struct{ Body struct{ Entries []JournalEntryItem `json:"entries"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListJournalEntries(ctx, db.ListJournalEntriesParams{ProjectID: p.ID, Role: in.Role})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]JournalEntryItem, len(rows))
			for i, r := range rows {
				out[i] = JournalEntryItem{
					Date:          r.Date,
					Baseline:      r.Baseline,
					Tldr:          r.Tldr,
					Assessment:    r.Assessment,
					Concerns:      r.Concerns,
					Next:          r.Next,
					ChangelogJSON: r.ChangelogJson,
					StateJSON:     r.StateJson,
				}
			}
			return &struct{ Body struct{ Entries []JournalEntryItem `json:"entries"` } }{Body: struct{ Entries []JournalEntryItem `json:"entries"` }{Entries: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "journal-state", Method: http.MethodGet, Path: "/api/dx/journal/state"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Role string `query:"role" required:"true"`
		}) (*struct{ Body struct{ StateJSON string `json:"state_json"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			entry, err := s.q.GetLatestJournalEntry(ctx, db.GetLatestJournalEntryParams{ProjectID: p.ID, Role: in.Role})
			if err != nil {
				return &struct{ Body struct{ StateJSON string `json:"state_json"` } }{}, nil
			}
			return &struct{ Body struct{ StateJSON string `json:"state_json"` } }{Body: struct{ StateJSON string `json:"state_json"` }{StateJSON: entry.StateJson}}, nil
		})

	// ── Errors ────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "report-error", Method: http.MethodPost, Path: "/api/dx/errors/report"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				Source     string `json:"source"`
				Endpoint   string `json:"endpoint"`
				ErrorName  string `json:"error_name"`
				StackTrace string `json:"stack_trace"`
			}
		}) (*struct{ Body ErrorReportItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.InsertErrorReport(ctx, db.InsertErrorReportParams{
				ProjectID:  pgtype.Int4{Int32: p.ID, Valid: true},
				Source:     in.Body.Source,
				Endpoint:   in.Body.Endpoint,
				ErrorName:  in.Body.ErrorName,
				StackTrace: in.Body.StackTrace,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ErrorReportItem }{Body: ErrorReportItem{
				ID:         row.ID,
				Source:     row.Source,
				Endpoint:   row.Endpoint,
				ErrorName:  row.ErrorName,
				StackTrace: row.StackTrace,
				CreatedAt:  fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-errors", Method: http.MethodGet, Path: "/api/dx/errors"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{ Body struct{ Errors []ErrorReportItem `json:"errors"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListErrorReports(ctx, pgtype.Int4{Int32: p.ID, Valid: true})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ErrorReportItem, len(rows))
			for i, r := range rows {
				out[i] = ErrorReportItem{
					ID:         r.ID,
					Source:     r.Source,
					Endpoint:   r.Endpoint,
					ErrorName:  r.ErrorName,
					StackTrace: r.StackTrace,
					CreatedAt:  fmtTS(r.CreatedAt),
				}
			}
			return &struct{ Body struct{ Errors []ErrorReportItem `json:"errors"` } }{Body: struct{ Errors []ErrorReportItem `json:"errors"` }{Errors: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "trigger-error", Method: http.MethodGet, Path: "/api/error"},
		func(ctx context.Context, in *struct{}) (*struct{}, error) {
			return nil, apiErr(500, "test error — this is intentional")
		})

	huma.Register(api, huma.Operation{OperationID: "clear-errors", Method: http.MethodDelete, Path: "/api/dx/errors"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.DeleteErrorReports(ctx, pgtype.Int4{Int32: p.ID, Valid: true}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return nil, nil
		})

	huma.Register(api, huma.Operation{OperationID: "report-slow-query", Method: http.MethodPost, Path: "/api/dx/slow-queries/report"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string `json:"slug"`
				SqlHash     string `json:"sql_hash"`
				SqlText     string `json:"sql_text"`
				Endpoint    string `json:"endpoint"`
				DurationMs  int32  `json:"duration_ms"`
				ExplainJson string `json:"explain_json"`
			}
		}) (*struct{ Body SlowQueryItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.InsertSlowQuery(ctx, db.InsertSlowQueryParams{
				ProjectID:   pgtype.Int4{Int32: p.ID, Valid: true},
				SqlHash:     in.Body.SqlHash,
				SqlText:     in.Body.SqlText,
				Endpoint:    in.Body.Endpoint,
				DurationMs:  in.Body.DurationMs,
				ExplainJson: in.Body.ExplainJson,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body SlowQueryItem }{Body: SlowQueryItem{
				ID:          row.ID,
				SqlHash:     row.SqlHash,
				SqlText:     row.SqlText,
				Endpoint:    row.Endpoint,
				DurationMs:  row.DurationMs,
				ExplainJson: row.ExplainJson,
				CreatedAt:   fmtTS(row.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-slow-queries", Method: http.MethodGet, Path: "/api/dx/slow-queries"},
		func(ctx context.Context, in *IssueSlugInput) (*struct{ Body struct{ Queries []SlowQueryItem `json:"queries"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListSlowQueries(ctx, pgtype.Int4{Int32: p.ID, Valid: true})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]SlowQueryItem, len(rows))
			for i, r := range rows {
				out[i] = SlowQueryItem{
					ID:          r.ID,
					SqlHash:     r.SqlHash,
					SqlText:     r.SqlText,
					Endpoint:    r.Endpoint,
					DurationMs:  r.DurationMs,
					ExplainJson: r.ExplainJson,
					CreatedAt:   fmtTS(r.CreatedAt),
				}
			}
			return &struct{ Body struct{ Queries []SlowQueryItem `json:"queries"` } }{Body: struct{ Queries []SlowQueryItem `json:"queries"` }{Queries: out}}, nil
		})

	// ── Timed ─────────────────────────────────────────────────────────────────

	type TimedItem struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		DurationMs  int32  `json:"duration_ms"`
		Source      string `json:"source"`
		ContextJson string `json:"context_json"`
		CreatedAt   string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-timed", Method: http.MethodGet, Path: "/api/dx/timed"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug,omitempty"`
		}) (*struct{ Body struct{ Items []TimedItem `json:"items"` } }, error) {
			var projectID pgtype.Int4
			if in.Slug != "" {
				if p, err := getProject(ctx, s.q, in.Slug); err == nil {
					projectID = pgtype.Int4{Int32: p.ID, Valid: true}
				}
			}
			rows, err := s.q.ListTimed(ctx, projectID.Int32)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TimedItem, len(rows))
			for i, r := range rows {
				out[i] = TimedItem{
					ID: r.ID, Name: r.Name, DurationMs: r.DurationMs,
					Source: r.Source, ContextJson: r.ContextJson, CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct{ Body struct{ Items []TimedItem `json:"items"` } }{
				Body: struct{ Items []TimedItem `json:"items"` }{Items: out},
			}, nil
		})

	// ── Comments ──────────────────────────────────────────────────────────────

	type CommentItem struct {
		ID         int32  `json:"id"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Author     string `json:"author"`
		Body       string `json:"body"`
		CreatedAt  string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "add-comment", Method: http.MethodPost, Path: "/api/dx/comment/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				TargetType string `json:"target_type"`
				TargetID   string `json:"target_id"`
				Body       string `json:"body"`
			}
		}) (*struct{ Body CommentItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			author := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, err := s.q.GetUserByID(ctx, uid); err == nil {
					author = u.Email
				}
			}
			c, err := s.q.AddComment(ctx, db.AddCommentParams{
				ProjectID:  p.ID,
				TargetType: in.Body.TargetType,
				TargetID:   in.Body.TargetID,
				Author:     author,
				Body:       in.Body.Body,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CommentItem }{Body: CommentItem{
				ID: c.ID, TargetType: c.TargetType, TargetID: c.TargetID,
				Author: c.Author, Body: c.Body, CreatedAt: fmtTS(c.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-comments", Method: http.MethodGet, Path: "/api/dx/comment/list"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug"`
			TargetType string `query:"target_type"`
			TargetID   string `query:"target_id"`
		}) (*struct{ Body struct{ Comments []CommentItem `json:"comments"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListComments(ctx, db.ListCommentsParams{
				ProjectID:  p.ID,
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CommentItem, len(rows))
			for i, r := range rows {
				out[i] = CommentItem{
					ID: r.ID, TargetType: r.TargetType, TargetID: r.TargetID,
					Author: r.Author, Body: r.Body, CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct{ Body struct{ Comments []CommentItem `json:"comments"` } }{
				Body: struct{ Comments []CommentItem `json:"comments"` }{Comments: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-comments-read", Method: http.MethodPost, Path: "/api/dx/comment/mark-read"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug       string `json:"slug"`
				TargetType string `json:"target_type"`
				TargetID   string `json:"target_id"`
				Role       string `json:"role"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpsertCommentRead(ctx, db.UpsertCommentReadParams{
				ProjectID:  p.ID,
				TargetType: in.Body.TargetType,
				TargetID:   in.Body.TargetID,
				Role:       in.Body.Role,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Revisions ─────────────────────────────────────────────────────────────

	type RevisionItem struct {
		ID         int32  `json:"id"`
		TargetType string `json:"target_type"`
		TargetID   string `json:"target_id"`
		Field      string `json:"field"`
		OldVal     string `json:"old_val"`
		NewVal     string `json:"new_val"`
		Agent      string `json:"agent"`
		CreatedAt  string `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-revisions", Method: http.MethodGet, Path: "/api/dx/revisions"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug"`
			TargetType string `query:"target_type"`
			TargetID   string `query:"target_id"`
		}) (*struct{ Body struct{ Revisions []RevisionItem `json:"revisions"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListRevisions(ctx, db.ListRevisionsParams{
				ProjectID:  p.ID,
				TargetType: in.TargetType,
				TargetID:   in.TargetID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]RevisionItem, len(rows))
			for i, r := range rows {
				out[i] = RevisionItem{
					ID: r.ID, TargetType: r.TargetType, TargetID: r.TargetID,
					Field: r.Field, OldVal: r.OldVal, NewVal: r.NewVal,
					Agent: r.Agent, CreatedAt: fmtTS(r.CreatedAt),
				}
			}
			return &struct{ Body struct{ Revisions []RevisionItem `json:"revisions"` } }{
				Body: struct{ Revisions []RevisionItem `json:"revisions"` }{Revisions: out},
			}, nil
		})

	// ── Admin: LLM config ─────────────────────────────────────────────────────

	type LLMConfigBody struct {
		Type   string `json:"type"`
		URL    string `json:"url"`
		Model  string `json:"model"`
		APIKey string `json:"api_key,omitempty"` // omitted/redacted in GET
	}

	huma.Register(api, huma.Operation{OperationID: "get-llm-config", Method: http.MethodGet, Path: "/api/admin/llm-config"},
		func(ctx context.Context, _ *struct{}) (*struct{ Body LLMConfigBody }, error) {
			if !s.features.HasLLMConfig {
				return &struct{ Body LLMConfigBody }{Body: LLMConfigBody{}}, nil
			}
			cfg, err := s.q.GetLLMConfig(ctx)
			if err != nil {
				// No config yet — return empty.
				return &struct{ Body LLMConfigBody }{Body: LLMConfigBody{}}, nil
			}
			return &struct{ Body LLMConfigBody }{Body: LLMConfigBody{
				Type:  cfg.Type,
				URL:   cfg.Url,
				Model: cfg.Model,
				// api_key intentionally omitted in response
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-llm-config", Method: http.MethodPut, Path: "/api/admin/llm-config"},
		func(ctx context.Context, in *struct{ Body LLMConfigBody }) (*struct{ Body LLMConfigBody }, error) {
			if !s.features.HasLLMConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "LLM config schema not yet applied — run: dx migrate up")
			}
			cfg, err := s.q.UpsertLLMConfig(ctx, db.UpsertLLMConfigParams{
				Type:   in.Body.Type,
				Url:    in.Body.URL,
				Model:  in.Body.Model,
				ApiKey: in.Body.APIKey,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.reloadEmbedder(ctx)
			// Bulk-index existing issues now that a new LLM config is set.
			go s.reindexAllIssues()
			return &struct{ Body LLMConfigBody }{Body: LLMConfigBody{
				Type:  cfg.Type,
				URL:   cfg.Url,
				Model: cfg.Model,
			}}, nil
		})

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
			if !s.features.HasProjectGitConfig {
				return &struct{ Body GitConfigBody }{Body: GitConfigBody{GitBranch: "main"}}, nil
			}
			row, err := s.q.GetProjectGitConfig(ctx, in.Slug)
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
			if !s.features.HasProjectGitConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "git config schema not yet applied — run: dx migrate up")
			}
			if err := s.q.SetProjectGitConfig(ctx, db.SetProjectGitConfigParams{
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

	// ── Issue similarity ───────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "similar-issues", Method: http.MethodPost, Path: "/api/dx/issues/similar"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				Text string `json:"text"`
				N    int    `json:"n,omitempty"`
			}
		}) (*struct {
			Body struct {
				Issues []SimilarIssueItem `json:"issues"`
			}
		}, error) {
			n := in.Body.N
			if n <= 0 {
				n = 5
			}
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			results, err := s.findSimilarIssues(ctx, p.ID, in.Body.Text, n)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Issues []SimilarIssueItem `json:"issues"`
				}
			}{Body: struct {
				Issues []SimilarIssueItem `json:"issues"`
			}{Issues: results}}, nil
		})

	// ── Git commits ───────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "list-git-commits", Method: http.MethodGet, Path: "/api/dx/git/commits"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			N    int    `query:"n"`
		}) (*struct {
			Body struct {
				Commits []GitCommit `json:"commits"`
			}
		}, error) {
			if !s.features.HasProjectGitConfig {
				return &struct {
					Body struct {
						Commits []GitCommit `json:"commits"`
					}
				}{}, nil
			}
			n := in.N
			if n <= 0 || n > 100 {
				n = 20
			}
			row, err := s.q.GetProjectGitConfig(ctx, in.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Slug)
			}
			if row.GitUrl == "" {
				return nil, apiErr(http.StatusUnprocessableEntity, "git URL not configured for project "+in.Slug)
			}
			// Embed token into URL for HTTPS auth if provided.
			gitURL := row.GitUrl
			if row.GitToken != "" && strings.HasPrefix(gitURL, "https://") {
				gitURL = "https://" + row.GitToken + "@" + strings.TrimPrefix(gitURL, "https://")
			}
			dir := s.repoDir(in.Slug)
			branch := row.GitBranch
			if branch == "" {
				branch = "main"
			}
			if err := ensureRepo(dir, gitURL, branch); err != nil {
				return nil, apiErr(500, "git: "+err.Error())
			}
			commits, err := recentCommits(dir, branch, n)
			if err != nil {
				return nil, apiErr(500, "git: "+err.Error())
			}
			return &struct {
				Body struct {
					Commits []GitCommit `json:"commits"`
				}
			}{Body: struct {
				Commits []GitCommit `json:"commits"`
			}{Commits: commits}}, nil
		})

	// ── Code refs ─────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "attach-code-ref-to-issue", Method: http.MethodPost, Path: "/api/dx/code-refs/issue/attach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				IssueID   string  `json:"issue_id"`
				FilePath  string  `json:"file_path"`
				GitHash   *string `json:"git_hash,omitempty"`
				LineStart *int32  `json:"line_start,omitempty"`
				LineEnd   *int32  `json:"line_end,omitempty"`
				Note      *string `json:"note,omitempty"`
			}
		}) (*struct{ Body CodeRefItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := in.Body.IssueID
			if _, ferr := s.q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID}); ferr != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found: "+issueID)
			}
			ref, err := s.q.CreateCodeRef(ctx, db.CreateCodeRefParams{
				ProjectID: p.ID,
				FilePath:  in.Body.FilePath,
				GitHash:   ptrStr(in.Body.GitHash),
				LineStart: ptrInt32(in.Body.LineStart),
				LineEnd:   ptrInt32(in.Body.LineEnd),
				Note:      ptrStr(in.Body.Note),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := s.q.AttachCodeRefToIssue(ctx, db.AttachCodeRefToIssueParams{IssueID: issueID, CodeRefID: ref.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CodeRefItem }{Body: toCodeRefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "detach-code-ref-from-issue", Method: http.MethodPost, Path: "/api/dx/code-refs/issue/detach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				IssueID   string `json:"issue_id"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := s.q.DetachCodeRefFromIssue(ctx, db.DetachCodeRefFromIssueParams{IssueID: in.Body.IssueID, CodeRefID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-code-refs-for-issue", Method: http.MethodGet, Path: "/api/dx/code-refs/issue"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			IssueID string `query:"issue_id" required:"true"`
		}) (*struct{ Body struct{ Refs []CodeRefItem `json:"refs"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := s.q.ListCodeRefsByIssue(ctx, in.IssueID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CodeRefItem, len(rows))
			for i, r := range rows {
				out[i] = toCodeRefItem(r)
			}
			return &struct{ Body struct{ Refs []CodeRefItem `json:"refs"` } }{
				Body: struct{ Refs []CodeRefItem `json:"refs"` }{Refs: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "attach-code-ref-to-task", Method: http.MethodPost, Path: "/api/dx/code-refs/task/attach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				TaskID    string  `json:"task_id"`
				FilePath  string  `json:"file_path"`
				GitHash   *string `json:"git_hash,omitempty"`
				LineStart *int32  `json:"line_start,omitempty"`
				LineEnd   *int32  `json:"line_end,omitempty"`
				Note      *string `json:"note,omitempty"`
			}
		}) (*struct{ Body CodeRefItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			ref, err := s.q.CreateCodeRef(ctx, db.CreateCodeRefParams{
				ProjectID: p.ID,
				FilePath:  in.Body.FilePath,
				GitHash:   ptrStr(in.Body.GitHash),
				LineStart: ptrInt32(in.Body.LineStart),
				LineEnd:   ptrInt32(in.Body.LineEnd),
				Note:      ptrStr(in.Body.Note),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := s.q.AttachCodeRefToTask(ctx, db.AttachCodeRefToTaskParams{TaskID: in.Body.TaskID, CodeRefID: ref.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CodeRefItem }{Body: toCodeRefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "detach-code-ref-from-task", Method: http.MethodPost, Path: "/api/dx/code-refs/task/detach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				TaskID    string `json:"task_id"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := s.q.DetachCodeRefFromTask(ctx, db.DetachCodeRefFromTaskParams{TaskID: in.Body.TaskID, CodeRefID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-code-refs-for-task", Method: http.MethodGet, Path: "/api/dx/code-refs/task"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			TaskID string `query:"task_id" required:"true"`
		}) (*struct{ Body struct{ Refs []CodeRefItem `json:"refs"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := s.q.ListCodeRefsByTask(ctx, in.TaskID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CodeRefItem, len(rows))
			for i, r := range rows {
				out[i] = toCodeRefItem(r)
			}
			return &struct{ Body struct{ Refs []CodeRefItem `json:"refs"` } }{
				Body: struct{ Refs []CodeRefItem `json:"refs"` }{Refs: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-code-ref", Method: http.MethodPost, Path: "/api/dx/code-refs/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.DeleteCodeRef(ctx, db.DeleteCodeRefParams{ProjectID: p.ID, ID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Q&A ───────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "add-question", Method: http.MethodPost, Path: "/api/dx/qa/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug     string `json:"slug"`
				Category string `json:"category"`
				Question string `json:"question"`
			}
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.InsertQuestion(ctx, db.InsertQuestionParams{
				ProjectID: p.ID,
				Category:  in.Body.Category,
				Question:  in.Body.Question,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "answer-question", Method: http.MethodPost, Path: "/api/dx/qa/answer"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				ID     int32  `json:"id"`
				Answer string `json:"answer"`
			}
		}) (*struct{ Body QuestionItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			row, err := s.q.AnswerQuestion(ctx, db.AnswerQuestionParams{
				ProjectID: p.ID,
				ID:        in.Body.ID,
				Answer:    pgtype.Text{String: in.Body.Answer, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body QuestionItem }{Body: toQuestionItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-questions", Method: http.MethodGet, Path: "/api/dx/qa/list"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct{ Body struct{ Questions []QuestionItem `json:"questions"` } }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := s.q.ListQuestions(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]QuestionItem, len(rows))
			for i, r := range rows {
				out[i] = toQuestionItem(r)
			}
			return &struct{ Body struct{ Questions []QuestionItem `json:"questions"` } }{
				Body: struct{ Questions []QuestionItem `json:"questions"` }{Questions: out},
			}, nil
		})

	// ── File upload (raw chi — huma doesn't handle multipart) ─────────────────
	s.mux.Post("/api/upload", s.handleUpload)
	s.mux.Get("/api/files/{id}", s.handleFileServe)
}

// handleUpload accepts multipart/form-data with a single "file" field.
// Stores to s.uploadsDir and records in zdx_files. Returns {id, url}.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	const maxSize = 10 << 20 // 10 MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"invalid multipart"}`, http.StatusBadRequest)
		return
	}
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer f.Close()

	mimeType := fh.Header.Get("Content-Type")
	allowed := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}
	ext, ok := allowed[mimeType]
	if !ok {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"unsupported file type"}`, http.StatusBadRequest)
		return
	}

	// Generate a unique filename: year/month/randomhex.ext
	now := time.Now().UTC()
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	relPath := filepath.Join(
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		hex.EncodeToString(rnd[:])+ext,
	)
	absPath := filepath.Join(s.uploadsDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(absPath)
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	n, err := io.Copy(dst, f)
	dst.Close()
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	row, err := s.q.CreateFile(r.Context(), db.CreateFileParams{
		Provider:  "fs",
		Path:      relPath,
		MimeType:  mimeType,
		SizeBytes: n,
	})
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":  row.ID,
		"url": fmt.Sprintf("/api/files/%d", row.ID),
	})
}

// handleFileServe serves an uploaded file by its zdx_files.id.
func (s *Server) handleFileServe(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := s.q.GetFile(r.Context(), int32(id))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Join(s.uploadsDir, row.Path)
	w.Header().Set("Content-Type", row.MimeType)
	http.ServeFile(w, r, absPath)
}

// ── Internal helpers ───────────────────────────────────────────────────────

func (s *Server) featuresWithSpecs(ctx context.Context, slug string) (*struct {
	Body struct {
		Features []FeatureItem `json:"features"`
	}
}, error) {
	p, err := getProject(ctx, s.q, slug)
	if err != nil {
		return nil, err
	}
	rows, err := s.q.ListFeatures(ctx, p.ID)
	if err != nil {
		return nil, apiErr(500, err.Error())
	}
	// Fetch all specs in one query and group by feature_id.
	allSpecs, _ := s.q.ListSpecsForProject(ctx, p.ID)
	specsByFeature := make(map[int32][]db.ZdxSpec, len(allSpecs))
	for _, sp := range allSpecs {
		specsByFeature[sp.FeatureID] = append(specsByFeature[sp.FeatureID], sp)
	}
	out := make([]FeatureItem, len(rows))
	for i, f := range rows {
		out[i] = toFeatureItem(f, specsByFeature[f.ID])
	}
	return &struct{ Body struct{ Features []FeatureItem `json:"features"` } }{Body: struct{ Features []FeatureItem `json:"features"` }{Features: out}}, nil
}

func (s *Server) resolveTheme(ctx context.Context, projectID int32, ref string) (db.ZdxTheme, error) {
	// "TH-N" → integer lookup
	if strings.HasPrefix(ref, "TH-") {
		id := intFromPrefixed(ref, "TH-")
		t, err := s.q.GetThemeByID(ctx, db.GetThemeByIDParams{ProjectID: projectID, ID: id})
		if err != nil {
			return db.ZdxTheme{}, apiErr(http.StatusNotFound, "theme not found: "+ref)
		}
		return t, nil
	}
	t, err := s.q.GetThemeByName(ctx, db.GetThemeByNameParams{ProjectID: projectID, Name: ref})
	if err != nil {
		return db.ZdxTheme{}, apiErr(http.StatusNotFound, "theme not found: "+ref)
	}
	return t, nil
}

// ── Revision recording ────────────────────────────────────────────────────

func (s *Server) recordRevision(ctx context.Context, projectID int32, targetType, targetID, field, oldVal, newVal, agent string) {
	_ = s.q.AddRevision(ctx, db.AddRevisionParams{
		ProjectID:  projectID,
		TargetType: targetType,
		TargetID:   targetID,
		Field:      field,
		OldVal:     oldVal,
		NewVal:     newVal,
		Agent:      agent,
	})
}

// ── Model → response converters ────────────────────────────────────────────

func toIssueItem(r db.ZdxIssue) IssueItem {
	return IssueItem{
		ID:        issueIntID(r.ID),
		Title:     r.Title,
		Status:    r.Status,
		Priority:  r.Priority,
		Component: r.Component,
		BlockedBy: r.BlockedBy,
		Context:   r.Context,
		IssueType: r.IssueType,
		CreatedAt: fmtTS(r.CreatedAt),
	}
}

func toTaskItem(r db.ZdxTask) TaskItem {
	t := TaskItem{
		ID:          taskIntID(r.ID),
		Text:        r.Text,
		Feature:     r.Feature,
		Status:      r.Status,
		Reason:      r.Reason,
		Depends:     r.Depends,
		TestPlan:    r.TestPlan,
		TestRefs:    r.TestRefs,
		CreatedAt:   fmtTS(r.CreatedAt),
		CompletedAt: fmtTS(r.CompletedAt),
	}
	if r.Issue != "" {
		n := issueIntID(r.Issue)
		if n > 0 {
			t.IssueID = &n
		}
	}
	return t
}

func toFeatureItem(f db.ZdxFeature, specs []db.ZdxSpec) FeatureItem {
	item := FeatureItem{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		What:        f.What,
		Why:         f.Why,
		DoneWhen:    f.DoneWhen,
		Component:   f.Component,
		Category:    f.Category,
		Specs:       make([]SpecItem, len(specs)),
	}
	for i, sp := range specs {
		item.Specs[i] = SpecItem{
			ID:          sp.ID,
			Description: sp.Description,
			Kind:        sp.Kind,
		}
	}
	return item
}

func toTodoItem(r db.ZdxTodo) TodoItem {
	return TodoItem{
		ID:         r.ID,
		Text:       r.Text,
		Key:        r.Key,
		Persona:    r.Persona,
		Priority:   r.Priority,
		Status:     r.Status,
		CreatedAt:  fmtTS(r.CreatedAt),
		ResolvedAt: fmtTS(r.ResolvedAt),
	}
}

func toCodeRefItem(r db.ZdxCodeRef) CodeRefItem {
	return CodeRefItem{
		ID:        r.ID,
		FilePath:  r.FilePath,
		GitHash:   r.GitHash,
		LineStart: r.LineStart,
		LineEnd:   r.LineEnd,
		Note:      r.Note,
		CreatedAt: fmtTS(r.CreatedAt),
	}
}

func toQuestionItem(r db.ZdxQuestion) QuestionItem {
	return QuestionItem{
		ID:        r.ID,
		Category:  r.Category,
		Question:  r.Question,
		Answer:    r.Answer.String,
		CreatedAt: fmtTS(r.CreatedAt),
		UpdatedAt: fmtTS(r.UpdatedAt),
	}
}
