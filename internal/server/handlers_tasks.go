package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) publishTaskByID(ctx context.Context, id, eventType string, payload any) {
	task, err := s.q.GetTask(ctx, id)
	if err != nil {
		return
	}
	p, err := s.q.GetProjectByID(ctx, task.ProjectID)
	if err != nil {
		return
	}
	s.publishTask(p.Slug, id, eventType, payload)
}

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
			total, _ := s.q.CountTasks(ctx, db.CountTasksParams{ProjectID: p.ID, StatusFilter: in.Status, Search: in.Search})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTasksPaginated(ctx, db.ListTasksPaginatedParams{ProjectID: p.ID, StatusFilter: in.Status, Search: in.Search, PageLimit: limit, PageOffset: offset})
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

	huma.Register(api, huma.Operation{OperationID: "get-task", Method: http.MethodGet, Path: "/api/task"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct{ Body TaskItem }, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			id := taskIDFromInt(taskIntID(in.ID))
			row, err := s.q.GetTask(ctx, id)
			if err != nil {
				return nil, apiErr(404, "task not found")
			}
			if row.ProjectID != p.ID {
				return nil, apiErr(404, "task not found")
			}
			return &struct{ Body TaskItem }{Body: toTaskItem(db.ZdxTask{
				ID: row.ID, ProjectID: row.ProjectID, Text: row.Text, Feature: row.Feature,
				Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends,
				TestPlan: row.TestPlan, TestRefs: row.TestRefs, TaskGroup: row.TaskGroup,
				ClaimedBy: row.ClaimedBy, ClaimedAt: row.ClaimedAt, LeaseExpiresAt: row.LeaseExpiresAt,
				CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt,
			})}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-tasks-by-feature", Method: http.MethodGet, Path: "/api/tasks-by-feature"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			Feature string `query:"feature" required:"true"`
			Limit   int32  `query:"limit"`
			Offset  int32  `query:"offset"`
			Status  string `query:"status"`
			Search  string `query:"search"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountTasksByFeature(ctx, db.CountTasksByFeatureParams{ProjectID: p.ID, Feature: in.Feature, StatusFilter: in.Status, Search: in.Search})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTasksByFeaturePaginated(ctx, db.ListTasksByFeaturePaginatedParams{ProjectID: p.ID, Feature: in.Feature, StatusFilter: in.Status, Search: in.Search, PageLimit: limit, PageOffset: offset})
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
			Status  string `query:"status"`
			Search  string `query:"search"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountTasksByIssue(ctx, db.CountTasksByIssueParams{ProjectID: p.ID, Issue: in.IssueID, StatusFilter: in.Status, Search: in.Search})
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTasksByIssuePaginated(ctx, db.ListTasksByIssuePaginatedParams{ProjectID: p.ID, Issue: in.IssueID, StatusFilter: in.Status, Search: in.Search, PageLimit: limit, PageOffset: offset})
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
				AutoReady bool    `json:"auto_ready,omitempty"`
				Force     bool    `json:"force,omitempty"`
			}
		}) (*struct {
			Body struct {
				TaskItem
				Similar          []SimilarTaskItem `json:"similar,omitempty"`
				DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			issueFilter := ""
			if in.Body.Issue != nil {
				issueFilter = *in.Body.Issue
			}

			if !in.Body.Force {
				exactMatches, err := s.q.GetTaskByExactText(ctx, db.GetTaskByExactTextParams{
					ProjectID: p.ID,
					Text:      in.Body.Text,
					Issue:     issueFilter,
				})
				if err == nil && len(exactMatches) > 0 {
					return nil, apiErr(409, "exact duplicate task already exists: "+exactMatches[0].ID+" ("+exactMatches[0].Status+")")
				}
			}

			id, err := s.q.NextTaskID(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			status := "wip"
			if in.Body.AutoReady {
				status = "pending"
			}

			var similar []SimilarTaskItem
			var duplicateBlocked bool
			if !in.Body.AutoReady && !in.Body.Force {
				similar, _ = s.findSimilarTasks(ctx, p.ID, in.Body.Text, 5)
				if issueFilter != "" {
					for _, s := range similar {
						if s.Issue == issueFilter && s.Score > 0.85 {
							duplicateBlocked = true
							break
						}
					}
				}
			}

			if duplicateBlocked {
				return &struct {
					Body struct {
						TaskItem
						Similar          []SimilarTaskItem `json:"similar,omitempty"`
						DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
					}
				}{Body: struct {
					TaskItem
					Similar          []SimilarTaskItem `json:"similar,omitempty"`
					DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
				}{Similar: similar, DuplicateBlocked: true}}, nil
			}

			row, err := s.q.CreateTask(ctx, db.CreateTaskParams{
				ID:        id,
				ProjectID: p.ID,
				Text:      in.Body.Text,
				Feature:   ptrStr(in.Body.Feature),
				Issue:     ptrStr(in.Body.Issue),
				TaskGroup: ptrStr(in.Body.TaskGroup),
				Status:    status,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go s.emb.upsertTask(context.Background(), p.ID, row.ID, row.Text)
			item := toTaskItem(db.ZdxTask{ID: row.ID, ProjectID: row.ProjectID, Text: row.Text, Feature: row.Feature, Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends, TestPlan: row.TestPlan, TestRefs: row.TestRefs, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt, TaskGroup: row.TaskGroup})
			return &struct {
				Body struct {
					TaskItem
					Similar          []SimilarTaskItem `json:"similar,omitempty"`
					DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
				}
			}{Body: struct {
				TaskItem
				Similar          []SimilarTaskItem `json:"similar,omitempty"`
				DuplicateBlocked bool              `json:"duplicate_blocked,omitempty"`
			}{TaskItem: item, Similar: similar}}, nil
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
			prev := ""
			if t, gErr := s.q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := s.q.MarkTaskDone(ctx, db.MarkTaskDoneParams{
				ID:       id,
				TestPlan: ptrStr(in.Body.TestPlan),
				TestRefs: ptrStr(in.Body.TestRefs),
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordTaskStatusChange(ctx, id, prev, "done", "")
			s.publishTaskByID(ctx, id, "task.done", map[string]any{"id": id})
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-task-undone", Method: http.MethodPost, Path: "/api/dx/todo/dev/undone"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			if t, gErr := s.q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := s.q.MarkTaskUndone(ctx, id); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordTaskStatusChange(ctx, id, prev, "pending", "")
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "block-task", Method: http.MethodPost, Path: "/api/dx/todo/dev/block"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID     int32   `json:"id"`
				Reason *string `json:"reason,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			if t, gErr := s.q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: "blocked",
				Reason: ptrStr(in.Body.Reason),
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordTaskStatusChange(ctx, id, prev, "blocked", "")
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "unblock-task", Method: http.MethodPost, Path: "/api/dx/todo/dev/unblock"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			if t, gErr := s.q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: "pending",
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordTaskStatusChange(ctx, id, prev, "pending", "")
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
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			if t, gErr := s.q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := s.q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: in.Body.Status,
				Reason: ptrStr(in.Body.Reason),
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordTaskStatusChange(ctx, id, prev, in.Body.Status, "")
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

	huma.Register(api, huma.Operation{OperationID: "list-stale-tasks", Method: http.MethodGet, Path: "/api/dx/tasks/stale"},
		func(ctx context.Context, in *struct {
			Slug  string `query:"slug" required:"true"`
			Issue string `query:"issue"`
		}) (*struct {
			Body struct {
				Tasks []TaskItem `json:"tasks"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			var rows []db.ListStaleTasksByIssueRow
			if in.Issue != "" {
				rows, err = s.q.ListStaleTasksByIssue(ctx, db.ListStaleTasksByIssueParams{ProjectID: p.ID, Issue: in.Issue})
			} else {
				staleRows, err2 := s.q.ListStaleTasks(ctx, p.ID)
				if err2 != nil {
					return nil, apiErr(500, err2.Error())
				}
				for _, r := range staleRows {
					rows = append(rows, db.ListStaleTasksByIssueRow(r))
				}
			}
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup, StaleSince: r.StaleSince})
			}
			return &struct {
				Body struct {
					Tasks []TaskItem `json:"tasks"`
				}
			}{Body: struct {
				Tasks []TaskItem `json:"tasks"`
			}{Tasks: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "sweep-stale-tasks", Method: http.MethodPost, Path: "/api/dx/tasks/sweep-stale"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				StaleDays int32  `json:"stale_days"`
			}
		}) (*struct {
			Body struct {
				Flagged int `json:"flagged"`
			}
		}, error) {
			days := in.Body.StaleDays
			if days < 0 {
				days = 3
			}
			var projectID int32
			if in.Body.Slug != "" {
				p, err := getProject(ctx, s.q, in.Body.Slug)
				if err != nil {
					return nil, err
				}
				projectID = p.ID
			}
			flagged, err := s.q.FlagStaleTasks(ctx, db.FlagStaleTasksParams{StaleDays: days, ProjectID: projectID})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Flagged int `json:"flagged"`
				}
			}{Body: struct {
				Flagged int `json:"flagged"`
			}{Flagged: len(flagged)}}, nil
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

	// ── Task ready (wip → pending) ───────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "ready-task", Method: http.MethodPost, Path: "/api/dx/todo/task/ready"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			if t, gErr := s.q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := s.q.ReadyTask(ctx, id); err != nil {
				return nil, apiErr(500, err.Error())
			}
			s.recordTaskStatusChange(ctx, id, prev, "pending", "")
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Task similarity ──────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "similar-tasks", Method: http.MethodPost, Path: "/api/dx/tasks/similar"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				Text string `json:"text"`
				N    int    `json:"n,omitempty"`
			}
		}) (*struct {
			Body struct {
				Tasks []SimilarTaskItem `json:"tasks"`
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
			results, err := s.findSimilarTasks(ctx, p.ID, in.Body.Text, n)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					Tasks []SimilarTaskItem `json:"tasks"`
				}
			}{Body: struct {
				Tasks []SimilarTaskItem `json:"tasks"`
			}{Tasks: results}}, nil
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

	// ── Review endpoints ────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "get-review-data", Method: http.MethodGet, Path: "/api/dx/todo/dev/review-data"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   int32  `query:"id" required:"true"`
		}) (*struct {
			Body struct {
				Task      TaskItem `json:"task"`
				IssueType string   `json:"issue_type"`
				TestPlan  string   `json:"test_plan"`
				TestRefs  string   `json:"test_refs"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			id := taskIDFromInt(in.ID)
			row, err := s.q.GetTaskWithReview(ctx, id)
			if err != nil {
				return nil, apiErr(404, "task not found")
			}
			if row.ProjectID != p.ID {
				return nil, apiErr(404, "task not found")
			}
			issueType := ""
			if row.Issue != "" {
				issueRow, err := s.q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: row.Issue})
				if err == nil {
					issueType = issueRow.IssueType
				}
			}
			task := toTaskItem(db.ZdxTask{
				ID: row.ID, ProjectID: row.ProjectID, Text: row.Text, Feature: row.Feature,
				Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends,
				TestPlan: row.TestPlan, TestRefs: row.TestRefs, TaskGroup: row.TaskGroup,
				CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt,
			})
			task.ReviewedAt = fmtTS(row.ReviewedAt)
			return &struct {
				Body struct {
					Task      TaskItem `json:"task"`
					IssueType string   `json:"issue_type"`
					TestPlan  string   `json:"test_plan"`
					TestRefs  string   `json:"test_refs"`
				}
			}{Body: struct {
				Task      TaskItem `json:"task"`
				IssueType string   `json:"issue_type"`
				TestPlan  string   `json:"test_plan"`
				TestRefs  string   `json:"test_refs"`
			}{Task: task, IssueType: issueType, TestPlan: row.TestPlan, TestRefs: row.TestRefs}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-task-reviewed", Method: http.MethodPost, Path: "/api/dx/todo/dev/review"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				ID      int32  `json:"id"`
				Verdict string `json:"verdict"`
				Comment string `json:"comment"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			id := taskIDFromInt(in.Body.ID)
			if err := s.q.MarkTaskReviewed(ctx, id); err != nil {
				return nil, apiErr(500, err.Error())
			}
			if in.Body.Comment != "" {
				body := fmt.Sprintf("## Review [%s]\n\n%s", in.Body.Verdict, in.Body.Comment)
				s.q.AddComment(ctx, db.AddCommentParams{
					ProjectID:  p.ID,
					TargetType: "task",
					TargetID:   fmt.Sprintf("TK-%d", in.Body.ID),
					Body:       body,
					Author:     "reviewer",
				})
			}
			s.publishTask(p.Slug, id, "task.reviewed", map[string]any{"id": id, "verdict": in.Body.Verdict})
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-unreviewed-tasks", Method: http.MethodGet, Path: "/api/dx/todo/dev/unreviewed"},
		func(ctx context.Context, in *struct {
			Slug  string `query:"slug" required:"true"`
			Issue string `query:"issue"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Issue != "" {
				rows, err := s.q.ListUnreviewedDoneTasksByIssue(ctx, db.ListUnreviewedDoneTasksByIssueParams{ProjectID: p.ID, Issue: in.Issue})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				out := make([]TaskItem, len(rows))
				for i, r := range rows {
					t := toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
					t.ReviewedAt = fmtTS(r.ReviewedAt)
					out[i] = t
				}
				return &TasksSlugOutput{Body: struct {
					Tasks []TaskItem `json:"tasks"`
					Total int64      `json:"total"`
				}{Tasks: out, Total: int64(len(out))}}, nil
			}
			rows, err := s.q.ListUnreviewedDoneTasks(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				t := toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
				t.ReviewedAt = fmtTS(r.ReviewedAt)
				out[i] = t
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: int64(len(out))}}, nil
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
	if r.ReviewedAt.Valid {
		s := fmtTS(r.ReviewedAt)
		t.ReviewedAt = s
	}
	if r.StaleSince.Valid {
		t.StaleSince = fmtTS(r.StaleSince)
	}
	if r.ClaimedBy.Valid {
		t.ClaimedBy = r.ClaimedBy.String
	}
	if r.ClaimedAt.Valid {
		t.ClaimedAt = fmtTS(r.ClaimedAt)
	}
	if r.LeaseExpiresAt.Valid {
		t.LeaseExpiresAt = fmtTS(r.LeaseExpiresAt)
	}
	return t
}
