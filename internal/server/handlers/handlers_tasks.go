package handlers

import (
	"context"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery"
	"github.com/iodesystems/sqlc-go-codegen-metaquery/metaquery/mqpgx"

	"github.com/iodesystems/zdx-go/internal/db"
)

// templatedTitleStems are case-insensitive prefixes emitted by workflow nudges
// (see internal/workflowhints) where the discriminator before the colon
// identifies the work's *target*. Two open tasks with the same stem are
// duplicates regardless of body text — the test-ref nudge re-fires per
// session and the agent writes a different "test approach" each time.
var templatedTitleStems = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^Test spec \d+:`),
}

// titleStem returns the canonical templated-prefix stem for a title (lowercase,
// trimmed) or "" if no templated prefix matches.
func titleStem(title string) string {
	for _, re := range templatedTitleStems {
		if m := re.FindString(title); m != "" {
			return strings.ToLower(strings.TrimSpace(m))
		}
	}
	return ""
}

// deriveTitle extracts a short title from task text when no explicit title is
// provided. Uses the first line, stripped of markdown heading prefixes, and
// truncates to 120 characters.
func deriveTitle(text string) string {
	line, _, _ := strings.Cut(text, "\n")
	line = strings.TrimSpace(line)
	line = strings.TrimLeft(line, "#")
	line = strings.TrimSpace(line)
	if len(line) > 120 {
		line = line[:117] + "..."
	}
	return line
}

func (h *Handler) publishTaskByID(ctx context.Context, id, eventType string, payload any) {
	task, err := h.Q.GetTask(ctx, id)
	if err != nil {
		return
	}
	p, err := h.Q.GetProjectByID(ctx, task.ProjectID)
	if err != nil {
		return
	}
	h.Broker.PublishTask(p.Slug, id, eventType, payload)
}

func (h *Handler) registerTaskRoutes(api huma.API) {
	type TasksSlugOutput = struct {
		Body struct {
			Tasks []TaskItem `json:"tasks"`
			Total int64      `json:"total"`
		}
	}

	huma.Register(api, huma.Operation{OperationID: "list-tasks", Method: http.MethodGet, Path: "/api/tasks"},
		func(ctx context.Context, in *PaginatedSlugInput) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListTasks(db.ListTasksParams{ProjectID: p.ID, StatusFilter: in.Status, Search: in.Search}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListTasksRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, Spec: r.Spec, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: res.Meta.Pagination.Total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-task", Method: http.MethodGet, Path: "/api/task"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			ID   string `query:"id" required:"true"`
		}) (*struct{ Body TaskItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			id := taskIDFromInt(taskIntID(in.ID))
			row, err := h.Q.GetTask(ctx, id)
			if err != nil {
				return nil, apiErr(404, "task not found")
			}
			if row.ProjectID != p.ID {
				return nil, apiErr(404, "task not found")
			}
			return &struct{ Body TaskItem }{Body: toTaskItem(db.ZdxTask{
				ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, Text: row.Text, Feature: row.Feature,
				Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends,
				TestPlan: row.TestPlan, TestRefs: row.TestRefs, Spec: row.Spec, TaskGroup: row.TaskGroup,
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListTasksByFeature(db.ListTasksByFeatureParams{ProjectID: p.ID, Feature: in.Feature, StatusFilter: in.Status, Search: in.Search}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListTasksByFeatureRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, Spec: r.Spec, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: res.Meta.Pagination.Total}}, nil
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			limit, offset := parsePage(in.Limit, in.Offset)
			b := db.WrapListTasksByIssue(db.ListTasksByIssueParams{ProjectID: p.ID, Issue: in.IssueID, StatusFilter: in.Status, Search: in.Search}).
				ApplyPagination(metaquery.PageRequest{Page: int(offset / limit), Size: int(limit), Total: true})
			res, err := mqpgx.Scan[db.ListTasksByIssueRow](ctx, h.Pool, b)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(res.Data))
			for i, r := range res.Data {
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, Spec: r.Spec, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: res.Meta.Pagination.Total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-task", Method: http.MethodPost, Path: "/api/dx/todo/tech/add"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				Feature   *string `json:"feature,omitempty"`
				Title     *string `json:"title,omitempty"`
				Text      string  `json:"text"`
				Reason    *string `json:"reason,omitempty"`
				TestPlan  *string `json:"test_plan,omitempty"`
				Issue     *string `json:"issue,omitempty"`
				Spec      *string `json:"spec,omitempty"`
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			issueFilter := ""
			if in.Body.Issue != nil {
				issueFilter = *in.Body.Issue
			}

			if issueFilter != "" {
				iss, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueFilter})
				if err != nil {
					return nil, apiErr(404, "issue not found: "+issueFilter)
				}
				if iss.Status != "open" && iss.Status != "wip" {
					return nil, apiErr(400, issueFilter+" is "+iss.Status+" — pick an open issue or omit --issue to link by feature")
				}
			}

			if in.Body.Feature != nil && *in.Body.Feature != "" {
				if _, err := h.Q.GetFeature(ctx, db.GetFeatureParams{ProjectID: p.ID, Name: *in.Body.Feature}); err != nil {
					return nil, apiErr(404, "feature not found: "+*in.Body.Feature)
				}
			}

			specFilter := ""
			if in.Body.Spec != nil {
				specFilter = *in.Body.Spec
			}
			if specFilter != "" {
				specID, err := strconv.Atoi(specFilter)
				if err != nil {
					return nil, apiErr(400, "invalid spec id: "+specFilter)
				}
				if _, err := h.Q.GetSpecForProject(ctx, db.GetSpecForProjectParams{ID: int32(specID), ProjectID: p.ID}); err != nil {
					return nil, apiErr(404, "spec not found: "+specFilter)
				}
			}

			if !in.Body.Force {
				exactMatches, err := h.Q.GetTaskByExactText(ctx, db.GetTaskByExactTextParams{
					ProjectID: p.ID,
					Text:      in.Body.Text,
					Issue:     issueFilter,
				})
				if err == nil && len(exactMatches) > 0 {
					return nil, apiErr(409, "exact duplicate task already exists: "+exactMatches[0].ID+" ("+exactMatches[0].Status+")")
				}
			}

			// Templated-title dedupe (IS-495). Workflow nudges like
			// `[tech:test-ref] target=spec:N` instruct agents to file a task
			// titled "Test spec N: ...". Different agent sessions write
			// different bodies, so the embedding-similarity check below
			// misses them. Match by templated stem and block on any open
			// same-stem task. Honors --force only; --auto-ready does NOT
			// bypass — these are sharp signals worth respecting.
			var stemMatches []SimilarTaskItem
			if !in.Body.Force {
				newTitle := ptrStr(in.Body.Title)
				if newTitle == "" {
					newTitle = deriveTitle(in.Body.Text)
				}
				if stem := titleStem(newTitle); stem != "" {
					rows, err := h.Q.ListOpenTasksByTitlePrefix(ctx, db.ListOpenTasksByTitlePrefixParams{
						ProjectID: p.ID,
						Prefix:    stem,
					})
					if err == nil && len(rows) > 0 {
						for _, r := range rows {
							stemMatches = append(stemMatches, SimilarTaskItem{
								ID: r.ID, Title: r.Title, Text: r.Text,
								Status: r.Status, Reason: r.Reason, Issue: r.Issue,
								Score: 1.0,
							})
						}
					}
				}
			}

			id, err := h.Q.NextTaskID(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			status := "wip"
			if in.Body.AutoReady {
				status = "ready"
			}

			var similar []SimilarTaskItem
			var duplicateBlocked bool
			if len(stemMatches) > 0 {
				similar = stemMatches
				duplicateBlocked = true
			} else if !in.Body.AutoReady && !in.Body.Force {
				similar, _ = h.findSimilarTasks(ctx, p.ID, in.Body.Text, 5)
				for _, s := range similar {
					if s.Score < 0.90 {
						continue
					}
					// Only open tasks block — closed dupes are informational
					// (they often explain why the work re-fires) and can't
					// be picked up anyway.
					if s.Status != "wip" && s.Status != "ready" && s.Status != "active" {
						continue
					}
					// When the new task is scoped to an issue, only same-issue
					// dupes block; cross-issue overlap may be coincidental.
					// When unscoped (e.g. test-ref candidates), any open
					// high-similarity match blocks.
					if issueFilter == "" || s.Issue == issueFilter {
						duplicateBlocked = true
						break
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

			title := ptrStr(in.Body.Title)
			if title == "" {
				title = deriveTitle(in.Body.Text)
			}

			row, err := h.Q.CreateTask(ctx, db.CreateTaskParams{
				ID:        id,
				ProjectID: p.ID,
				Title:     title,
				Text:      in.Body.Text,
				Feature:   ptrStr(in.Body.Feature),
				Issue:     ptrStr(in.Body.Issue),
				TaskGroup: ptrStr(in.Body.TaskGroup),
				Status:    status,
				Reason:    ptrStr(in.Body.Reason),
				TestPlan:  ptrStr(in.Body.TestPlan),
				Spec:      specFilter,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertTask(context.Background(), p.ID, row.ID, row.Text)
			item := toTaskItem(db.ZdxTask{ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, Text: row.Text, Feature: row.Feature, Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends, TestPlan: row.TestPlan, TestRefs: row.TestRefs, Spec: row.Spec, CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt, TaskGroup: row.TaskGroup})
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

	huma.Register(api, huma.Operation{OperationID: "start-task", Method: http.MethodPost, Path: "/api/dx/todo/dev/start"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID               int32  `json:"id"`
				ClaimedBy        string `json:"claimed_by,omitempty"`
				LeaseDurationMin int32  `json:"lease_duration_min,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			t, err := h.Q.GetTask(ctx, id)
			if err != nil {
				return nil, apiErr(404, "task not found")
			}
			prev := t.Status
			if err := h.Q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: "active",
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			dur := in.Body.LeaseDurationMin
			if dur <= 0 {
				dur = 30
			}
			leaseExpiry := pgtype.Timestamptz{Time: time.Now().Add(time.Duration(dur) * time.Minute), Valid: true}
			_, _ = h.Q.InsertReservation(ctx, db.InsertReservationParams{
				ProjectID:      t.ProjectID,
				TargetType:     "task",
				TargetID:       id,
				ClaimedBy:      in.Body.ClaimedBy,
				LeaseExpiresAt: leaseExpiry,
			})
			h.recordTaskStatusChange(ctx, id, prev, "active", in.Body.ClaimedBy)
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
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
			var oldTask db.GetTaskRow
			var haveOld bool
			if t, gErr := h.Q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
				oldTask = t
				haveOld = true
			}
			newTestPlan := ptrStr(in.Body.TestPlan)
			newTestRefs := ptrStr(in.Body.TestRefs)
			if err := h.Q.MarkTaskDone(ctx, db.MarkTaskDoneParams{
				ID:       id,
				TestPlan: newTestPlan,
				TestRefs: newTestRefs,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			if haveOld && oldTask.ProjectID != 0 {
				_ = h.Q.ReleaseReservation(ctx, db.ReleaseReservationParams{
					ProjectID:  oldTask.ProjectID,
					TargetType: "task",
					TargetID:   id,
				})
			}
			h.recordTaskStatusChange(ctx, id, prev, "done", "")
			if haveOld {
				h.recordTaskFieldRevisions(ctx, oldTask, map[string]string{
					"test_plan": newTestPlan,
					"test_refs": newTestRefs,
				})
			}
			h.publishTaskByID(ctx, id, "task.done", map[string]any{"id": id})
			if haveOld && oldTask.ProjectID != 0 {
				h.refreshQueueAsync(oldTask.ProjectID)
			}
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
			var taskProjectID int32
			if t, gErr := h.Q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
				taskProjectID = t.ProjectID
			}
			if err := h.Q.MarkTaskUndone(ctx, id); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordTaskStatusChange(ctx, id, prev, "ready", "")
			if taskProjectID != 0 {
				h.refreshQueueAsync(taskProjectID)
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
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			var oldTask db.GetTaskRow
			var haveOld bool
			if t, gErr := h.Q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
				oldTask = t
				haveOld = true
			}
			newReason := ptrStr(in.Body.Reason)
			if err := h.Q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: "blocked",
				Reason: newReason,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordTaskStatusChange(ctx, id, prev, "blocked", "")
			if haveOld {
				h.recordTaskFieldRevisions(ctx, oldTask, map[string]string{"reason": newReason})
			}
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
			var taskProjectID int32
			if t, gErr := h.Q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
				taskProjectID = t.ProjectID
			}
			if err := h.Q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: "ready",
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordTaskStatusChange(ctx, id, prev, "ready", "")
			if taskProjectID != 0 {
				h.refreshQueueAsync(taskProjectID)
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
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			var oldTask db.GetTaskRow
			var haveOld bool
			if t, gErr := h.Q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
				oldTask = t
				haveOld = true
			}
			newReason := ptrStr(in.Body.Reason)
			if err := h.Q.UpdateTaskStatus(ctx, db.UpdateTaskStatusParams{
				ID:     id,
				Status: in.Body.Status,
				Reason: newReason,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordTaskStatusChange(ctx, id, prev, in.Body.Status, "")
			if haveOld {
				h.recordTaskFieldRevisions(ctx, oldTask, map[string]string{"reason": newReason})
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-task", Method: http.MethodDelete, Path: "/api/task"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			if err := h.Q.DeleteTask(ctx, taskIDFromInt(in.Body.ID)); err != nil {
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			var rows []db.ListStaleTasksByIssueRow
			if in.Issue != "" {
				rows, err = h.Q.ListStaleTasksByIssue(ctx, db.ListStaleTasksByIssueParams{ProjectID: p.ID, Issue: in.Issue})
			} else {
				staleRows, err2 := h.Q.ListStaleTasks(ctx, p.ID)
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
				out[i] = toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, Spec: r.Spec, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup, StaleSince: r.StaleSince})
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
				p, err := getProject(ctx, h.Q, in.Body.Slug)
				if err != nil {
					return nil, err
				}
				projectID = p.ID
			}
			flagged, err := h.Q.FlagStaleTasks(ctx, db.FlagStaleTasksParams{StaleDays: days, ProjectID: projectID})
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

	// ── Task ready (wip → ready) ─────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "ready-task", Method: http.MethodPost, Path: "/api/dx/todo/task/ready"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			prev := ""
			if t, gErr := h.Q.GetTask(ctx, id); gErr == nil {
				prev = t.Status
			}
			if err := h.Q.ReadyTask(ctx, id); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.recordTaskStatusChange(ctx, id, prev, "ready", "")
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Task adopt (link orphan task to issue) ───────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "adopt-task", Method: http.MethodPost, Path: "/api/dx/tasks/adopt"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug    string `json:"slug"`
				TaskID  string `json:"task_id"`
				IssueID string `json:"issue_id"`
			}
		}) (*struct{ Body TaskItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if !strings.HasPrefix(in.Body.TaskID, "TK-") {
				return nil, apiErr(400, "expected task_id like TK-N, got "+in.Body.TaskID)
			}
			if !strings.HasPrefix(in.Body.IssueID, "IS-") {
				return nil, apiErr(400, "expected issue_id like IS-N, got "+in.Body.IssueID)
			}
			task, err := h.Q.GetTask(ctx, in.Body.TaskID)
			if err != nil {
				return nil, apiErr(404, "task not found: "+in.Body.TaskID)
			}
			if task.ProjectID != p.ID {
				return nil, apiErr(404, "task not found: "+in.Body.TaskID)
			}
			switch task.Status {
			case "ready", "wip", "active", "blocked":
			default:
				return nil, apiErr(409, in.Body.TaskID+" is "+task.Status+" — adopt is only allowed for ready/wip/active/blocked tasks")
			}
			if task.Issue != "" {
				return nil, apiErr(409, in.Body.TaskID+" is already linked to "+task.Issue+" — release the link before re-adopting")
			}
			iss, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: in.Body.IssueID})
			if err != nil {
				return nil, apiErr(404, "issue not found: "+in.Body.IssueID)
			}
			if iss.Status != "open" && iss.Status != "wip" {
				return nil, apiErr(400, in.Body.IssueID+" is "+iss.Status+" — pick an open issue")
			}
			if err := h.Q.AdoptTaskToIssue(ctx, db.AdoptTaskToIssueParams{ID: in.Body.TaskID, Issue: in.Body.IssueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.refreshQueueAsync(p.ID)
			row, err := h.Q.GetTask(ctx, in.Body.TaskID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body TaskItem }{Body: toTaskItem(db.ZdxTask{
				ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, Text: row.Text, Feature: row.Feature,
				Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends,
				TestPlan: row.TestPlan, TestRefs: row.TestRefs, Spec: row.Spec, TaskGroup: row.TaskGroup,
				CreatedAt: row.CreatedAt, CompletedAt: row.CompletedAt, UpdatedAt: row.UpdatedAt,
			})}, nil
		})

	// ── Task delete (wip drafts only) ────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "delete-draft-task", Method: http.MethodPost, Path: "/api/dx/todo/task/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID int32 `json:"id"`
			}
		}) (*struct{ Body OKBody }, error) {
			id := taskIDFromInt(in.Body.ID)
			t, gErr := h.Q.GetTask(ctx, id)
			if gErr != nil {
				return nil, apiErr(404, "task not found: "+id)
			}
			if t.Status != "wip" {
				return nil, apiErr(409, "cannot delete "+id+": status is "+t.Status+" (delete is restricted to wip drafts; use 'dx todo dev done' or close the parent issue instead)")
			}
			n, err := h.Q.DeleteDraftTask(ctx, id)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if n == 0 {
				return nil, apiErr(409, "cannot delete "+id+": no wip task deleted (possible race)")
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	// ── Task convert-to-blocker (wip drafts only) ────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "convert-task-to-blocker", Method: http.MethodPost, Path: "/api/dx/tasks/convert-to-blocker"},
		func(ctx context.Context, in *struct {
			Body struct {
				ID               int32   `json:"id"`
				BlockedByIssueID string  `json:"blocked_by_issue_id"`
				Reason           *string `json:"reason,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			taskID := taskIDFromInt(in.Body.ID)
			if !strings.HasPrefix(in.Body.BlockedByIssueID, "IS-") {
				return nil, apiErr(400, "blocked_by_issue_id must be IS-N format, got "+in.Body.BlockedByIssueID)
			}
			t, gErr := h.Q.GetTask(ctx, taskID)
			if gErr != nil {
				return nil, apiErr(404, "task not found: "+taskID)
			}
			if t.Status != "wip" {
				return nil, apiErr(409, taskID+" is "+t.Status+" — convert-to-blocker requires wip status")
			}
			if t.Issue == "" {
				return nil, apiErr(400, taskID+" has no parent issue — cannot add a blocker without a parent issue to block")
			}
			blockerIssue, bErr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: t.ProjectID, ID: in.Body.BlockedByIssueID})
			if bErr != nil {
				return nil, apiErr(404, "blocked-by issue not found: "+in.Body.BlockedByIssueID)
			}
			_ = blockerIssue
			tx, tErr := h.Pool.Begin(ctx)
			if tErr != nil {
				return nil, apiErr(500, tErr.Error())
			}
			defer tx.Rollback(ctx)
			q := h.Q.WithTx(tx)
			if err := q.AddIssueBlock(ctx, db.AddIssueBlockParams{IssueID: t.Issue, BlockedByID: in.Body.BlockedByIssueID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			n, err := q.DeleteDraftTask(ctx, taskID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if n == 0 {
				return nil, apiErr(409, "cannot delete "+taskID+": possible race")
			}
			reason := ptrStr(in.Body.Reason)
			note := "[blocker added] " + in.Body.BlockedByIssueID + " (from " + taskID + ")"
			if reason != "" {
				note += ": " + reason
			}
			agent := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, uErr := h.Q.GetUserByID(ctx, uid); uErr == nil {
					agent = u.Email
				}
			}
			_ = q.AppendIssueWork(ctx, db.AppendIssueWorkParams{IssueID: t.Issue, Agent: agent, Note: note})
			if err := tx.Commit(ctx); err != nil {
				return nil, apiErr(500, err.Error())
			}
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
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			results, err := h.findSimilarTasks(ctx, p.ID, in.Body.Text, n)
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
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			id := taskIDFromInt(in.ID)
			row, err := h.Q.GetTaskWithReview(ctx, id)
			if err != nil {
				return nil, apiErr(404, "task not found")
			}
			if row.ProjectID != p.ID {
				return nil, apiErr(404, "task not found")
			}
			issueType := ""
			if row.Issue != "" {
				issueRow, err := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: row.Issue})
				if err == nil {
					issueType = issueRow.IssueType
				}
			}
			task := toTaskItem(db.ZdxTask{
				ID: row.ID, ProjectID: row.ProjectID, Title: row.Title, Text: row.Text, Feature: row.Feature,
				Status: row.Status, Reason: row.Reason, Issue: row.Issue, Depends: row.Depends,
				TestPlan: row.TestPlan, TestRefs: row.TestRefs, Spec: row.Spec, TaskGroup: row.TaskGroup,
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
				Slug         string `json:"slug"`
				ID           int32  `json:"id"`
				Verdict      string `json:"verdict"`
				Body         string `json:"body,omitempty" required:"false"`
				ReviewerRole string `json:"reviewer_role,omitempty" required:"false"`
			}
		}) (*struct {
			Body struct {
				OK     bool  `json:"ok"`
				Review int32 `json:"review_id"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			id := taskIDFromInt(in.Body.ID)
			if err := h.Q.MarkTaskReviewed(ctx, id); err != nil {
				return nil, apiErr(500, err.Error())
			}
			role := in.Body.ReviewerRole
			if role == "" {
				role = "reviewer"
			}
			var reviewerUID pgtype.Int4
			if uid := ctxUserIDVal(ctx); uid != 0 {
				reviewerUID = pgtype.Int4{Int32: uid, Valid: true}
			}
			rev, err := h.Q.UpsertTaskReview(ctx, db.UpsertTaskReviewParams{
				ProjectID:      p.ID,
				TaskID:         id,
				ReviewerRole:   role,
				ReviewerUserID: reviewerUID,
				Verdict:        in.Body.Verdict,
				Body:           in.Body.Body,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if in.Body.Verdict == "approve" {
				_ = h.Q.UpsertCommentRead(ctx, db.UpsertCommentReadParams{
					ProjectID:  p.ID,
					TargetType: "review",
					TargetID:   reviewIDStr(rev.ID),
					Role:       "llm",
				})
			}
			h.Broker.PublishTask(p.Slug, id, "task.reviewed", map[string]any{"id": id, "verdict": in.Body.Verdict, "review_id": rev.ID})
			return &struct {
				Body struct {
					OK     bool  `json:"ok"`
					Review int32 `json:"review_id"`
				}
			}{Body: struct {
				OK     bool  `json:"ok"`
				Review int32 `json:"review_id"`
			}{OK: true, Review: rev.ID}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-task-reviews", Method: http.MethodGet, Path: "/api/dx/task/review"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			TaskID string `query:"task_id" required:"true"`
		}) (*struct {
			Body struct {
				Reviews []TaskReviewItem `json:"reviews"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListTaskReviews(ctx, db.ListTaskReviewsParams{ProjectID: p.ID, TaskID: in.TaskID})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskReviewItem, 0, len(rows))
			for _, r := range rows {
				item := TaskReviewItem{
					ID:           r.ID,
					TaskID:       r.TaskID,
					ReviewerRole: r.ReviewerRole,
					Verdict:      r.Verdict,
					Body:         r.Body,
					CreatedAt:    fmtTS(r.CreatedAt),
					UpdatedAt:    fmtTS(r.UpdatedAt),
				}
				if r.ReviewerEmail.Valid {
					item.ReviewerEmail = r.ReviewerEmail.String
				}
				out = append(out, item)
			}
			return &struct {
				Body struct {
					Reviews []TaskReviewItem `json:"reviews"`
				}
			}{Body: struct {
				Reviews []TaskReviewItem `json:"reviews"`
			}{Reviews: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-unreviewed-tasks", Method: http.MethodGet, Path: "/api/dx/todo/dev/unreviewed"},
		func(ctx context.Context, in *struct {
			Slug  string `query:"slug" required:"true"`
			Issue string `query:"issue"`
		}) (*TasksSlugOutput, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Issue != "" {
				rows, err := h.Q.ListUnreviewedDoneTasksByIssue(ctx, db.ListUnreviewedDoneTasksByIssueParams{ProjectID: p.ID, Issue: in.Issue})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				out := make([]TaskItem, len(rows))
				for i, r := range rows {
					t := toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, Spec: r.Spec, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
					t.ReviewedAt = fmtTS(r.ReviewedAt)
					out[i] = t
				}
				return &TasksSlugOutput{Body: struct {
					Tasks []TaskItem `json:"tasks"`
					Total int64      `json:"total"`
				}{Tasks: out, Total: int64(len(out))}}, nil
			}
			rows, err := h.Q.ListUnreviewedDoneTasks(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TaskItem, len(rows))
			for i, r := range rows {
				t := toTaskItem(db.ZdxTask{ID: r.ID, ProjectID: r.ProjectID, Title: r.Title, Text: r.Text, Feature: r.Feature, Status: r.Status, Reason: r.Reason, Issue: r.Issue, Depends: r.Depends, TestPlan: r.TestPlan, TestRefs: r.TestRefs, Spec: r.Spec, CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt, UpdatedAt: r.UpdatedAt, TaskGroup: r.TaskGroup})
				t.ReviewedAt = fmtTS(r.ReviewedAt)
				out[i] = t
			}
			return &TasksSlugOutput{Body: struct {
				Tasks []TaskItem `json:"tasks"`
				Total int64      `json:"total"`
			}{Tasks: out, Total: int64(len(out))}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-task-reservations", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/tasks/{id}/reservations"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			ID   string `path:"id" required:"true"`
		}) (*struct {
			Body struct {
				Reservations []ReservationItem `json:"reservations"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListReservationsByTaskID(ctx, db.ListReservationsByTaskIDParams{
				ProjectID: p.ID,
				TaskID:    in.ID,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ReservationItem, len(rows))
			for i, r := range rows {
				out[i] = ReservationItem{
					ID:             r.ID,
					TargetType:     r.TargetType,
					TargetID:       r.TargetID,
					ClaimedBy:      r.ClaimedBy,
					ClaimedAt:      fmtTS(r.ClaimedAt),
					ReleasedAt:     fmtTS(r.ReleasedAt),
					LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
				}
			}
			return &struct {
				Body struct {
					Reservations []ReservationItem `json:"reservations"`
				}
			}{Body: struct {
				Reservations []ReservationItem `json:"reservations"`
			}{Reservations: out}}, nil
		})
}

// ── Model → response converter ────────────────────────────────────────────

func toTaskItem(r db.ZdxTask) TaskItem {
	t := TaskItem{
		ID:          taskIntID(r.ID),
		Title:       r.Title,
		Text:        r.Text,
		Feature:     r.Feature,
		Status:      r.Status,
		Reason:      r.Reason,
		Spec:        r.Spec,
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
	return t
}
