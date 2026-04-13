package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

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

// ── Route registration ─────────────────────────────────────────────────────

func (s *Server) registerRoutes(api huma.API) {
	// Health
	huma.Register(api, huma.Operation{OperationID: "health", Method: http.MethodGet, Path: "/api/health"},
		func(ctx context.Context, _ *struct{}) (*struct{ Body map[string]string }, error) {
			return &struct{ Body map[string]string }{Body: map[string]string{"status": "ok", "build_sha": s.buildSHA}}, nil
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
				Slug      string `json:"slug"`
				Title     string `json:"title"`
				Source    string `json:"source"`
				Context   string `json:"context"`
				BlockedBy string `json:"blocked_by"`
				Component string `json:"component"`
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
			row, err := s.q.CreateIssue(ctx, db.CreateIssueParams{
				ID:        id,
				ProjectID: p.ID,
				Title:     in.Body.Title,
				Context:   in.Body.Context,
				Component: in.Body.Component,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body IssueItem }{Body: toIssueItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "triage-issue", Method: http.MethodPost, Path: "/api/dx/todo/owner/triage"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID       int32 `json:"id"`
				Priority int32 `json:"priority"`
			}
		}) (*struct{ Body OKBody }, error) {
			issueID := issueIDFromInt(in.Body.ID)
			err := s.q.SetIssuePriority(ctx, db.SetIssuePriorityParams{
				ID:        issueID,
				Priority:  strconv.Itoa(int(in.Body.Priority)),
				ProjectID: 0,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "close-issue", Method: http.MethodPost, Path: "/api/dx/todo/issue/close"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug   string `json:"slug"`
				ID     int32  `json:"id"`
				Reason string `json:"reason"`
				Notes  string `json:"notes"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := issueIDFromInt(in.Body.ID)
			if err := s.q.CloseIssue(ctx, db.CloseIssueParams{ProjectID: p.ID, ID: issueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			if in.Body.Reason != "" || in.Body.Notes != "" {
				note := in.Body.Reason
				if in.Body.Notes != "" {
					note += "\n" + in.Body.Notes
				}
				_ = s.q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: issueID, Agent: "cli", Note: note})
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
				Slug    string `json:"slug"`
				Feature string `json:"feature"`
				Text    string `json:"text"`
				Issue   string `json:"issue"`
				Depends string `json:"depends"`
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
				Feature:   in.Body.Feature,
				Issue:     in.Body.Issue,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body TaskItem }{Body: toTaskItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-task-done", Method: http.MethodPost, Path: "/api/dx/todo/dev/done"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID       int32  `json:"id"`
				TestPlan string `json:"test_plan"`
				TestRefs string `json:"test_refs"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			if err := s.q.MarkTaskDone(ctx, db.MarkTaskDoneParams{
				ID:       id,
				TestPlan: in.Body.TestPlan,
				TestRefs: in.Body.TestRefs,
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
				ID     int32  `json:"id"`
				Reason string `json:"reason"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     taskIDFromInt(in.Body.ID),
				Status: "blocked",
				Reason: in.Body.Reason,
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
				ID     int32  `json:"id"`
				Status string `json:"status"`
				Reason string `json:"reason"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     taskIDFromInt(in.Body.ID),
				Status: in.Body.Status,
				Reason: in.Body.Reason,
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
	out := make([]FeatureItem, len(rows))
	for i, f := range rows {
		specs, _ := s.q.ListSpecs(ctx, f.ID)
		out[i] = toFeatureItem(f, specs)
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
