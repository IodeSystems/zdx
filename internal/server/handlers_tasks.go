package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerTaskRoutes(api huma.API) {
	type TasksSlugOutput = struct {
		Body struct {
			Tasks []TaskItem `json:"tasks"`
			Total int64      `json:"total"`
		}
	}

	huma.Register(api, huma.Operation{OperationID: "list-tasks", Method: http.MethodGet, Path: "/api/tasks"},
		func(ctx context.Context, in *PaginatedSlugInput) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountTasks(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTasksPaginated(ctx, db.ListTasksPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-tasks-by-feature", Method: http.MethodGet, Path: "/api/tasks-by-feature"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			Feature string `query:"feature" required:"true"`
			Limit   int32  `query:"limit"`
			Offset  int32  `query:"offset"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountTasksByFeature(ctx, db.CountTasksByFeatureParams{ProjectID: p.ID, Feature: in.Feature})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTasksByFeaturePaginated(ctx, db.ListTasksByFeaturePaginatedParams{ProjectID: p.ID, Feature: in.Feature, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-tasks-for-issue", Method: http.MethodGet, Path: "/api/dx/todo/issue/tasks"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			IssueID string `query:"issue_id" required:"true"`
			Limit   int32  `query:"limit"`
			Offset  int32  `query:"offset"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountTasksByIssue(ctx, db.CountTasksByIssueParams{ProjectID: p.ID, Issue: in.IssueID})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTasksByIssuePaginated(ctx, db.ListTasksByIssuePaginatedParams{ProjectID: p.ID, Issue: in.IssueID, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-task", Method: http.MethodPost, Path: "/api/dx/todo/tech/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				Feature   *string `json:"feature,omitempty"`
				Text      string  `json:"text"`
				Issue     *string `json:"issue,omitempty"`
				Depends   *string `json:"depends,omitempty"`
				TaskGroup *string `json:"task_group,omitempty"`
			}
		}) (*struct{ Body TaskItem }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			id, err := s.q.NextTaskID(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			row, err := s.q.CreateTask(ctx, db.CreateTaskParams{
				ID:        id,
				ProjectID: p.ID,
				Text:      in.Body.Text,
				Feature:   ptrStr(in.Body.Feature),
				Issue:     ptrStr(in.Body.Issue),
				TaskGroup: ptrStr(in.Body.TaskGroup),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body TaskItem }{Body: toTaskItem(db.ZdxTask{ID: row.ID, ProjectID: row.ProjectID, Text: row.Text, Feature: row.Feature, Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends, TestPlan: row.TestPlan, TestRefs: row.TestRefs, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt, TaskGroup: row.TaskGroup})}, nil
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
			s.publish(fmt.Sprintf("task:%s", id), "task.done", map[string]any{"id": id})
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-task-undone", Method: http.MethodPost, Path: "/api/dx/todo/dev/undone"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
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
			Body struct {
				ID int32 `json:"id"`
			}
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
			Body struct {
				ID int32 `json:"id"`
			}
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
		}) (*struct {
			Body struct {
				CommitRefs string `json:"commit_refs"`
			}
		}, error) {
			return &struct {
				Body struct {
					CommitRefs string `json:"commit_refs"`
				}
			}{}, nil
		})
}

// ── Model → response converter ────────────────────────────────────────────

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
		TaskGroup:   r.TaskGroup,
		CreatedAt:   fmtTS(r.CreatedAt),
		CompletedAt: fmtTS(r.CompletedAt),
		UpdatedAt:   fmtTS(r.UpdatedAt),
	}
	if r.Issue != "" {
		n := issueIntID(r.Issue)
		if n > 0 {
			t.IssueID = &n
		}
	}
	return t
}
