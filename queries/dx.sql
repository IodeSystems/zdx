-- Projects

-- name: ListProjects :many
SELECT id, slug, name, created_at FROM zdx_projects ORDER BY name;

-- name: GetProjectBySlug :one
SELECT id, slug, name, created_at FROM zdx_projects WHERE slug = $1;

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
