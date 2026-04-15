package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/techmetrics"
)

func (s *Server) registerDxRoutes(api huma.API) {
	// ── State ─────────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "get-state", Method: http.MethodGet, Path: "/api/dx/state"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Key  string `query:"key" required:"true"`
		}) (*struct {
			Body struct {
				Value string `json:"value"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			val, err := s.q.GetState(ctx, db.GetStateParams{ProjectID: p.ID, Key: in.Key})
			if err != nil {
				val = ""
			}
			return &struct {
				Body struct {
					Value string `json:"value"`
				}
			}{Body: struct {
				Value string `json:"value"`
			}{Value: val}}, nil
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
		func(ctx context.Context, in *IssueSlugInput) (*struct {
			Body struct {
				Todos []TodoItem `json:"todos"`
			}
		}, error) {
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
			return &struct {
				Body struct {
					Todos []TodoItem `json:"todos"`
				}
			}{Body: struct {
				Todos []TodoItem `json:"todos"`
			}{Todos: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "write-todos", Method: http.MethodPost, Path: "/api/dx/todos"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug  string           `json:"slug"`
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
				Slug    string            `json:"slug"`
				Results []TestResultInput `json:"results"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			for _, r := range in.Body.Results {
				test, _ := s.q.UpsertTest(ctx, db.UpsertTestParams{
					ProjectID:  p.ID,
					Component:  r.Driver,
					Name:       r.TestName,
					Layer:      "integration",
					Status:     r.Status,
					DurationMs: r.DurationMS,
				})
				_ = s.q.UpsertTestResult(ctx, db.UpsertTestResultParams{
					ProjectID:  p.ID,
					Driver:     r.Driver,
					TestName:   r.TestName,
					Feature:    r.Feature,
					Status:     r.Status,
					DurationMs: r.DurationMS,
					Branch:     r.Branch,
					GitSha:     r.GitSHA,
				})
				_ = s.q.InsertTestResultHistory(ctx, db.InsertTestResultHistoryParams{
					ProjectID:  p.ID,
					Driver:     r.Driver,
					TestName:   r.TestName,
					Feature:    r.Feature,
					Status:     r.Status,
					DurationMs: r.DurationMS,
					Branch:     r.Branch,
					GitSha:     r.GitSHA,
				})
				if test.ID != 0 {
					for _, d := range r.DemoArtifacts {
						_, _ = s.q.UpsertTestDemo(ctx, db.UpsertTestDemoParams{
							TestID:       test.ID,
							DemoType:     d.DemoType,
							ArtifactPath: d.ArtifactPath,
							FileID:       pgtype.Int4{},
						})
					}
				}
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	type TestItem struct {
		ID         int32   `json:"id"`
		Component  string  `json:"component"`
		Name       string  `json:"name"`
		Layer      string  `json:"layer"`
		Status     string  `json:"status"`
		DurationMs int32   `json:"duration_ms"`
		LastRunAt  *string `json:"last_run_at,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-tests", Method: http.MethodGet, Path: "/api/dx/tests"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Tests []TestItem `json:"tests"`
				Total int64      `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountTests(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListTestsPaginated(ctx, db.ListTestsPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TestItem, len(rows))
			for i, r := range rows {
				var lastRunAt *string
				if r.LastRunAt.Valid {
					s := r.LastRunAt.Time.Format("2006-01-02T15:04:05Z07:00")
					lastRunAt = &s
				}
				out[i] = TestItem{ID: r.ID, Component: r.Component, Name: r.Name, Layer: r.Layer, Status: r.Status, DurationMs: r.DurationMs, LastRunAt: lastRunAt}
			}
			return &struct {
				Body struct {
					Tests []TestItem `json:"tests"`
					Total int64      `json:"total"`
				}
			}{Body: struct {
				Tests []TestItem `json:"tests"`
				Total int64      `json:"total"`
			}{Tests: out, Total: total}}, nil
		})

	type TestHistoryItem struct {
		ID         int32  `json:"id"`
		Driver     string `json:"driver"`
		TestName   string `json:"test_name"`
		Status     string `json:"status"`
		DurationMs int32  `json:"duration_ms"`
		RunAt      string `json:"run_at"`
		Branch     string `json:"branch"`
		GitSHA     string `json:"git_sha"`
	}

	huma.Register(api, huma.Operation{OperationID: "get-test-history", Method: http.MethodGet, Path: "/api/dx/tests/history"},
		func(ctx context.Context, in *struct {
			Slug     string `query:"slug"`
			TestName string `query:"test_name"`
			Limit    int32  `query:"limit"`
			Branch   string `query:"branch"`
		}) (*struct {
			Body struct {
				History []TestHistoryItem `json:"history"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			maxResults := int32(50)
			if in.Limit > 0 && in.Limit <= 200 {
				maxResults = in.Limit
			}
			rows, err := s.q.ListTestResultHistory(ctx, db.ListTestResultHistoryParams{
				ProjectID:    p.ID,
				TestName:     in.TestName,
				MaxResults:   maxResults,
				BranchFilter: in.Branch,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]TestHistoryItem, len(rows))
			for i, r := range rows {
				out[i] = TestHistoryItem{
					ID:         r.ID,
					Driver:     r.Driver,
					TestName:   r.TestName,
					Status:     r.Status,
					DurationMs: r.DurationMs,
					RunAt:      r.RunAt.Time.Format("2006-01-02T15:04:05Z07:00"),
					Branch:     r.Branch,
					GitSHA:     r.GitSha,
				}
			}
			return &struct {
				Body struct {
					History []TestHistoryItem `json:"history"`
				}
			}{Body: struct {
				History []TestHistoryItem `json:"history"`
			}{History: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-test", Method: http.MethodGet, Path: "/api/dx/tests/detail"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			TestID int32  `query:"test_id" required:"true"`
		}) (*struct {
			Body TestItem
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			t, err := s.q.GetTestByID(ctx, db.GetTestByIDParams{ProjectID: p.ID, ID: in.TestID})
			if err != nil {
				return nil, apiErr(404, "test not found")
			}
			var lastRunAt *string
			if t.LastRunAt.Valid {
				s := t.LastRunAt.Time.Format("2006-01-02T15:04:05Z07:00")
				lastRunAt = &s
			}
			return &struct{ Body TestItem }{Body: TestItem{
				ID: t.ID, Component: t.Component, Name: t.Name, Layer: t.Layer,
				Status: t.Status, DurationMs: t.DurationMs, LastRunAt: lastRunAt,
			}}, nil
		})

	// ── Journal ───────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "journal-checkin", Method: http.MethodPost, Path: "/api/dx/journal/checkin"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug          string `json:"slug"`
				Role          string `json:"role"`
				Date          string `json:"date"`
				Tldr          string `json:"tldr"`
				Assessment    string `json:"assessment"`
				Concerns      string `json:"concerns"`
				Next          string `json:"next"`
				StateJSON     string `json:"state_json,omitempty"`
				ChangelogJSON string `json:"changelog_json,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			stateJSON := in.Body.StateJSON
			if stateJSON == "" {
				stateJSON = "{}"
			}
			changelogJSON := in.Body.ChangelogJSON
			if changelogJSON == "" {
				changelogJSON = "{}"
			}
			_, err = s.q.InsertJournalEntry(ctx, db.InsertJournalEntryParams{
				ProjectID:     p.ID,
				Role:          in.Body.Role,
				Date:          in.Body.Date,
				Tldr:          in.Body.Tldr,
				Assessment:    in.Body.Assessment,
				Concerns:      in.Body.Concerns,
				Next:          in.Body.Next,
				StateJson:     stateJSON,
				ChangelogJson: changelogJSON,
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
		}) (*struct {
			Body struct {
				Entries []JournalEntryItem `json:"entries"`
			}
		}, error) {
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
			return &struct {
				Body struct {
					Entries []JournalEntryItem `json:"entries"`
				}
			}{Body: struct {
				Entries []JournalEntryItem `json:"entries"`
			}{Entries: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "journal-state", Method: http.MethodGet, Path: "/api/dx/journal/state"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
			Role string `query:"role" required:"true"`
		}) (*struct {
			Body struct {
				StateJSON string `json:"state_json"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			entry, err := s.q.GetLatestJournalEntry(ctx, db.GetLatestJournalEntryParams{ProjectID: p.ID, Role: in.Role})
			if err != nil {
				return &struct {
					Body struct {
						StateJSON string `json:"state_json"`
					}
				}{}, nil
			}
			return &struct {
				Body struct {
					StateJSON string `json:"state_json"`
				}
			}{Body: struct {
				StateJSON string `json:"state_json"`
			}{StateJSON: entry.StateJson}}, nil
		})

	// ── Journal Generate ─────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "journal-generate", Method: http.MethodPost, Path: "/api/dx/journal/generate"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
				Role string `json:"role"`
			}
		}) (*struct {
			Body struct {
				Entry JournalEntryItem `json:"entry"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			role := in.Body.Role
			if role == "" {
				role = "tech"
			}

			today := time.Now().Format("2006-01-02")
			stateJSON := "{}"
			changelogJSON := "{}"

			if role == "tech" {
				row, err := s.q.GetProjectGitConfig(ctx, in.Body.Slug)
				if err != nil || row.GitUrl == "" {
					return nil, apiErr(400, "project has no git config — configure repo URL in admin settings")
				}
				gitURL := row.GitUrl
				if row.GitToken != "" && strings.HasPrefix(gitURL, "https://") {
					gitURL = "https://" + row.GitToken + "@" + strings.TrimPrefix(gitURL, "https://")
				}
				dir := s.repoDir(in.Body.Slug)
				branch := row.GitBranch
				if branch == "" {
					branch = "main"
				}
				if err := ensureRepo(dir, gitURL, branch); err != nil {
					return nil, apiErr(500, "git: "+err.Error())
				}

				metrics := techmetrics.Collect(dir)

				var prevDate string
				var prevStateJSON string
				entries, _ := s.q.ListJournalEntries(ctx, db.ListJournalEntriesParams{ProjectID: p.ID, Role: role})
				if len(entries) > 0 {
					prevDate = entries[0].Date
					prevStateJSON = entries[0].StateJson
				}

				commits, filesChanged := techmetrics.CollectGitChurn(dir, prevDate)
				metrics.GitCommits = commits
				metrics.GitFilesChanged = filesChanged

				stateJSON = techmetrics.ToJSON(metrics)

				if prevMetrics, ok := techmetrics.Parse(prevStateJSON); ok {
					changelogJSON = techmetrics.DeltasToJSON(techmetrics.ComputeDeltas(prevMetrics, metrics))
				} else {
					changelogJSON = techmetrics.DeltasToJSON(techmetrics.ComputeDeltas(techmetrics.TechMetrics{}, metrics))
				}
			}

			_, err = s.q.InsertJournalEntry(ctx, db.InsertJournalEntryParams{
				ProjectID:     p.ID,
				Role:          role,
				Date:          today,
				Tldr:          "Auto-generated " + role + " check-in",
				Assessment:    "",
				Concerns:      "",
				Next:          "",
				StateJson:     stateJSON,
				ChangelogJson: changelogJSON,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			entry := JournalEntryItem{
				Date:          today,
				Tldr:          "Auto-generated " + role + " check-in",
				StateJSON:     stateJSON,
				ChangelogJSON: changelogJSON,
			}
			return &struct {
				Body struct {
					Entry JournalEntryItem `json:"entry"`
				}
			}{Body: struct {
				Entry JournalEntryItem `json:"entry"`
			}{Entry: entry}}, nil
		})

	// ── Solo Health ──────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "solo-health", Method: http.MethodGet, Path: "/api/dx/solo/health"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				GoalCount        int64  `json:"goal_count"`
				ConstraintCount  int64  `json:"constraint_count"`
				OwnerJournalDate string `json:"owner_journal_date"`
				TechJournalDate  string `json:"tech_journal_date"`
				ClosedTaskCount  int64  `json:"closed_task_count"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			goalCount, _ := s.q.CountProjectGoals(ctx, p.ID)
			constraintCount, _ := s.q.CountProjectConstraints(ctx, p.ID)
			closedTaskCount, _ := s.q.CountClosedTasks(ctx, p.ID)

			var ownerDate, techDate string
			if oe, err := s.q.GetLatestJournalEntry(ctx, db.GetLatestJournalEntryParams{ProjectID: p.ID, Role: "owner"}); err == nil {
				ownerDate = oe.Date
			}
			if te, err := s.q.GetLatestJournalEntry(ctx, db.GetLatestJournalEntryParams{ProjectID: p.ID, Role: "tech"}); err == nil {
				techDate = te.Date
			}

			return &struct {
				Body struct {
					GoalCount        int64  `json:"goal_count"`
					ConstraintCount  int64  `json:"constraint_count"`
					OwnerJournalDate string `json:"owner_journal_date"`
					TechJournalDate  string `json:"tech_journal_date"`
					ClosedTaskCount  int64  `json:"closed_task_count"`
				}
			}{Body: struct {
				GoalCount        int64  `json:"goal_count"`
				ConstraintCount  int64  `json:"constraint_count"`
				OwnerJournalDate string `json:"owner_journal_date"`
				TechJournalDate  string `json:"tech_journal_date"`
				ClosedTaskCount  int64  `json:"closed_task_count"`
			}{
				GoalCount:        goalCount,
				ConstraintCount:  constraintCount,
				OwnerJournalDate: ownerDate,
				TechJournalDate:  techDate,
				ClosedTaskCount:  closedTaskCount,
			}}, nil
		})
}

// ── Model → response converter ────────────────────────────────────────────

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
