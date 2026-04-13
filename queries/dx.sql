-- Projects

-- name: ListProjects :many
SELECT id, slug, name, created_at FROM zdx_projects ORDER BY name;

-- name: GetProjectBySlug :one
SELECT id, slug, name, created_at FROM zdx_projects WHERE slug = $1;

-- name: GetApiKeyByToken :one
SELECT id, user_id, token, name, last_used_at, created_at
FROM zdx_api_keys WHERE token = $1;

-- name: TouchApiKey :exec
UPDATE zdx_api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: CountApiKeys :one
SELECT COUNT(*)::int FROM zdx_api_keys;

-- name: CreateUser :one
INSERT INTO zdx_users (email, name, password_hash, role)
VALUES ($1, $2, '', 'admin')
RETURNING id, email, name, role, created_at;

-- name: CreateUserWithPassword :one
INSERT INTO zdx_users (email, name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, role, created_at;

-- name: GetUserByEmail :one
SELECT id, email, name, password_hash, role, created_at FROM zdx_users WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, name, role, created_at FROM zdx_users WHERE id = $1;

-- name: GetApiKeyUserRole :one
SELECT u.role FROM zdx_api_keys k JOIN zdx_users u ON u.id = k.user_id WHERE k.token = $1;

-- name: CreateApiKey :one
INSERT INTO zdx_api_keys (user_id, token, name)
VALUES ($1, $2, $3)
RETURNING id, user_id, token, name, last_used_at, created_at;

-- name: CreateProject :one
INSERT INTO zdx_projects (slug, name) VALUES ($1, $2)
RETURNING id, slug, name, created_at;

-- name: NextID :one
INSERT INTO zdx_id_seq (project_id, kind, next_val) VALUES ($1, $2, 2)
ON CONFLICT (project_id, kind) DO UPDATE SET next_val = zdx_id_seq.next_val + 1
RETURNING next_val - 1 AS val;

-- Issues

-- name: ListIssues :many
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at
FROM zdx_issues WHERE project_id = $1 ORDER BY priority NULLS LAST, created_at;

-- name: ListOpenIssues :many
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at
FROM zdx_issues WHERE project_id = $1 AND status = 'open' ORDER BY priority NULLS LAST, created_at;

-- name: GetIssue :one
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at
FROM zdx_issues WHERE project_id = $1 AND id = $2;

-- name: CreateIssue :one
INSERT INTO zdx_issues (id, project_id, title, context, priority, component)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, title, status, priority, component, context, blocked_by, created_at;

-- name: UpdateIssue :exec
UPDATE zdx_issues
SET title     = COALESCE(NULLIF(@title, ''),     title),
    context   = COALESCE(NULLIF(@context, ''),   context),
    priority  = COALESCE(NULLIF(@priority, ''),  priority),
    blocked_by= COALESCE(NULLIF(@blocked_by,''), blocked_by)
WHERE project_id = @project_id AND id = @id;

-- name: CloseIssue :exec
UPDATE zdx_issues SET status = 'closed' WHERE project_id = $1 AND id = $2;

-- name: AppendIssueWork :exec
INSERT INTO zdx_issue_work (issue_id, agent, note) VALUES ($1, $2, $3);

-- name: GetIssueWork :many
SELECT id, issue_id, agent, note, created_at FROM zdx_issue_work WHERE issue_id = $1 ORDER BY created_at;

-- Tasks

-- name: ListTasks :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, created_at, completed_at
FROM zdx_tasks WHERE project_id = $1 ORDER BY created_at;

-- name: ListTasksByFeature :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, created_at, completed_at
FROM zdx_tasks WHERE project_id = $1 AND feature = $2 ORDER BY created_at;

-- name: ListTasksByIssue :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, created_at, completed_at
FROM zdx_tasks WHERE project_id = $1 AND issue = $2 ORDER BY created_at;

-- name: CreateTask :one
INSERT INTO zdx_tasks (id, project_id, text, feature, issue)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, created_at, completed_at;

-- name: UpdateTaskStatus :exec
UPDATE zdx_tasks SET status = $2, reason = $3 WHERE id = $1;

-- name: MarkTaskDone :exec
UPDATE zdx_tasks
SET status = 'done', test_plan = $2, test_refs = $3, completed_at = NOW()
WHERE id = $1;

-- name: MarkTaskUndone :exec
UPDATE zdx_tasks SET status = 'pending', completed_at = NULL WHERE id = $1;

-- Features

-- name: ListFeatures :many
SELECT id, project_id, name, description, what, why, done_when, component
FROM zdx_features WHERE project_id = $1 ORDER BY name;

-- name: GetFeature :one
SELECT id, project_id, name, description, what, why, done_when, component
FROM zdx_features WHERE project_id = $1 AND name = $2;

-- name: UpsertFeature :one
INSERT INTO zdx_features (project_id, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, name) DO UPDATE SET description = EXCLUDED.description
RETURNING id, project_id, name, description, what, why, done_when, component;

-- name: UpdateFeatureField :exec
UPDATE zdx_features
SET description = CASE WHEN @field::text = 'description' THEN @value::text ELSE description END,
    what        = CASE WHEN @field::text = 'what'        THEN @value::text ELSE what        END,
    why         = CASE WHEN @field::text = 'why'         THEN @value::text ELSE why         END,
    done_when   = CASE WHEN @field::text = 'done_when'   THEN @value::text ELSE done_when   END,
    component   = CASE WHEN @field::text = 'component'   THEN @value::text ELSE component   END
WHERE project_id = @project_id AND name = @name;

-- Specs

-- name: ListSpecs :many
SELECT id, feature_id, description, kind FROM zdx_specs WHERE feature_id = $1 ORDER BY id;

-- name: AddSpec :one
INSERT INTO zdx_specs (feature_id, description, kind) VALUES ($1, $2, $3)
RETURNING id, feature_id, description, kind;

-- name: ReopenIssue :exec
UPDATE zdx_issues SET status = 'open' WHERE project_id = $1 AND id = $2;

-- name: SetIssueField :exec
UPDATE zdx_issues
SET title     = CASE WHEN @field::text = 'title'     THEN @value::text ELSE title     END,
    context   = CASE WHEN @field::text = 'context'   THEN @value::text ELSE context   END,
    component = CASE WHEN @field::text = 'component' THEN @value::text ELSE component END,
    blocked_by= CASE WHEN @field::text = 'blocked_by'THEN @value::text ELSE blocked_by END
WHERE project_id = @project_id AND id = @id;

-- name: SetIssuePriority :exec
UPDATE zdx_issues SET priority = @priority WHERE project_id = @project_id AND id = @id;

-- name: DeleteTask :exec
DELETE FROM zdx_tasks WHERE id = $1;

-- name: DeleteFeature :exec
DELETE FROM zdx_features WHERE id = $1;

-- name: UpdateTaskFields :exec
UPDATE zdx_tasks
SET text      = CASE WHEN @field::text = 'text'      THEN @value::text ELSE text      END,
    feature   = CASE WHEN @field::text = 'feature'   THEN @value::text ELSE feature   END,
    issue     = CASE WHEN @field::text = 'issue'     THEN @value::text ELSE issue     END,
    depends   = CASE WHEN @field::text = 'depends'   THEN @value::text ELSE depends   END,
    test_plan = CASE WHEN @field::text = 'test_plan' THEN @value::text ELSE test_plan END,
    test_refs = CASE WHEN @field::text = 'test_refs' THEN @value::text ELSE test_refs END
WHERE id = @id;

-- Plans

-- name: UpsertPlan :one
INSERT INTO zdx_plans (feature_id, plan_type, complexity, approach, status)
VALUES (@feature_id, @plan_type, @complexity, @approach, 'pending')
ON CONFLICT (feature_id) DO UPDATE
SET plan_type = EXCLUDED.plan_type,
    complexity = EXCLUDED.complexity,
    approach = EXCLUDED.approach,
    last_reviewed_at = NOW()
RETURNING id, feature_id, plan_type, status, complexity, approach, last_reviewed_at;

-- name: GetPlanByFeature :one
SELECT id, feature_id, plan_type, status, complexity, approach, last_reviewed_at
FROM zdx_plans WHERE feature_id = $1;

-- Themes

-- name: ListThemes :many
SELECT t.id, t.name, t.description, t.priority, t.status, t.created_at,
       COALESCE(STRING_AGG(tb.issue_id, ','), '') AS blockers
FROM zdx_themes t
LEFT JOIN zdx_theme_blockers tb ON tb.theme_id = t.id
WHERE t.project_id = $1
GROUP BY t.id ORDER BY t.priority, t.name;

-- name: GetThemeByID :one
SELECT id, project_id, name, description, priority, status, created_at
FROM zdx_themes WHERE project_id = $1 AND id = $2;

-- name: GetThemeByName :one
SELECT id, project_id, name, description, priority, status, created_at
FROM zdx_themes WHERE project_id = $1 AND name = $2;

-- name: CreateTheme :one
INSERT INTO zdx_themes (project_id, name, description, priority)
VALUES (@project_id, @name, @description, @priority)
RETURNING id, project_id, name, description, priority, status, created_at;

-- name: UpdateThemeStatus :exec
UPDATE zdx_themes SET status = @status WHERE project_id = @project_id AND id = @id;

-- name: AddThemeBlocker :exec
INSERT INTO zdx_theme_blockers (theme_id, issue_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveThemeBlocker :exec
DELETE FROM zdx_theme_blockers WHERE theme_id = $1 AND issue_id = $2;

-- Todos

-- name: ListTodos :many
SELECT id, project_id, feature_id, text, key, persona, priority, status, created_at, resolved_at
FROM zdx_todos WHERE project_id = $1 ORDER BY priority, created_at;

-- name: DeleteTodosForProject :exec
DELETE FROM zdx_todos WHERE project_id = $1;

-- name: CreateTodo :one
INSERT INTO zdx_todos (project_id, text, key, persona, priority, status)
VALUES (@project_id, @text, @key, @persona, @priority, @status)
RETURNING id, project_id, feature_id, text, key, persona, priority, status, created_at, resolved_at;

-- State

-- name: GetState :one
SELECT value FROM zdx_state WHERE project_id = $1 AND key = $2;

-- name: SetState :exec
INSERT INTO zdx_state (project_id, key, value, updated_at)
VALUES (@project_id, @key, @value, NOW())
ON CONFLICT (project_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();

-- Journal

-- name: InsertJournalEntry :one
INSERT INTO zdx_journal_entries (project_id, role, date, tldr, assessment, concerns, next)
VALUES (@project_id, @role, @date, @tldr, @assessment, @concerns, @next)
RETURNING id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, created_at;

-- name: ListJournalEntries :many
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 20;

-- name: GetLatestJournalEntry :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 1;

-- Test Results

-- name: UpsertTestResult :exec
INSERT INTO zdx_test_results (project_id, driver, test_name, feature, status, duration_ms)
VALUES (@project_id, @driver, @test_name, @feature, @status, @duration_ms)
ON CONFLICT (project_id, driver, test_name) DO UPDATE
SET status = EXCLUDED.status, duration_ms = EXCLUDED.duration_ms, run_at = NOW(),
    feature = EXCLUDED.feature;

-- name: InsertTestResultHistory :exec
INSERT INTO zdx_test_result_history (project_id, driver, test_name, feature, status, duration_ms)
VALUES (@project_id, @driver, @test_name, @feature, @status, @duration_ms);

-- Tests (primary registry)

-- name: UpsertTest :one
INSERT INTO zdx_tests (project_id, component, name, layer, status, last_run_at)
VALUES (@project_id, @component, @name, @layer, @status, NOW())
ON CONFLICT (project_id, component, name) DO UPDATE
SET layer       = EXCLUDED.layer,
    status      = EXCLUDED.status,
    last_run_at = NOW()
RETURNING id, project_id, component, name, layer, status, last_run_at, created_at;

-- name: GetTest :one
SELECT id, project_id, component, name, layer, status, last_run_at, created_at
FROM zdx_tests WHERE project_id = @project_id AND component = @component AND name = @name;

-- name: ListTests :many
SELECT id, project_id, component, name, layer, status, last_run_at, created_at
FROM zdx_tests WHERE project_id = $1 ORDER BY component, name;

-- name: ListTestsByLayer :many
SELECT id, project_id, component, name, layer, status, last_run_at, created_at
FROM zdx_tests WHERE project_id = $1 AND layer = $2 ORDER BY component, name;

-- name: ListTestsForSpec :many
SELECT t.id, t.project_id, t.component, t.name, t.layer, t.status, t.last_run_at, t.created_at
FROM zdx_tests t
JOIN zdx_spec_tests st ON st.test_id = t.id
WHERE st.spec_id = $1 ORDER BY t.component, t.name;

-- name: ListSpecsCoveredByTest :many
-- Used to show what breaks if a test is deleted.
SELECT s.id, s.feature_id, s.description, s.kind
FROM zdx_specs s
JOIN zdx_spec_tests st ON st.spec_id = s.id
WHERE st.test_id = $1 ORDER BY s.id;

-- name: LinkSpecTest :exec
INSERT INTO zdx_spec_tests (spec_id, test_id)
VALUES (@spec_id, @test_id)
ON CONFLICT DO NOTHING;

-- name: UnlinkSpecTest :exec
DELETE FROM zdx_spec_tests WHERE spec_id = @spec_id AND test_id = @test_id;

-- name: DeleteTest :exec
-- Will fail at the DB level (RESTRICT) if spec_tests rows reference this test.
-- Call ListSpecsCoveredByTest first to surface what breaks.
DELETE FROM zdx_tests WHERE id = $1;

-- Error reports

-- name: InsertErrorReport :one
INSERT INTO zdx_error_reports (project_id, source, endpoint, error_name, stack_trace)
VALUES (@project_id, @source, @endpoint, @error_name, @stack_trace)
RETURNING id, project_id, source, endpoint, error_name, stack_trace, created_at;

-- name: ListErrorReports :many
SELECT id, project_id, source, endpoint, error_name, stack_trace, created_at
FROM zdx_error_reports
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT 200;

-- Slow queries

-- name: InsertSlowQuery :one
INSERT INTO zdx_slow_queries (project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json)
VALUES (@project_id, @sql_hash, @sql_text, @endpoint, @duration_ms, @explain_json)
RETURNING id, project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json, created_at;

-- name: ListSlowQueries :many
SELECT id, project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json, created_at
FROM zdx_slow_queries
WHERE project_id = $1
ORDER BY duration_ms DESC
LIMIT 200;
