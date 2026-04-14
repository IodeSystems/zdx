-- name: ListTasks :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 ORDER BY updated_at DESC;

-- name: CountTasks :one
SELECT count(*) FROM zdx_tasks WHERE project_id = $1;

-- name: CountClosedTasks :one
SELECT count(*) FROM zdx_tasks WHERE project_id = $1 AND status = 'done';

-- name: ListTasksPaginated :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: ListTasksByFeature :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 AND feature = $2 ORDER BY updated_at DESC;

-- name: CountTasksByFeature :one
SELECT count(*) FROM zdx_tasks WHERE project_id = $1 AND feature = $2;

-- name: ListTasksByFeaturePaginated :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 AND feature = $2 ORDER BY updated_at DESC
LIMIT $3 OFFSET $4;

-- name: ListTasksByIssue :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 AND issue = $2 ORDER BY updated_at DESC;

-- name: CountTasksByIssue :one
SELECT count(*) FROM zdx_tasks WHERE project_id = $1 AND issue = $2;

-- name: ListTasksByIssuePaginated :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 AND issue = $2 ORDER BY updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CreateTask :one
INSERT INTO zdx_tasks (id, project_id, text, feature, issue, task_group)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at;

-- name: UpdateTaskStatus :exec
UPDATE zdx_tasks SET status = $2, reason = $3, updated_at = NOW() WHERE id = $1;

-- name: MarkTaskDone :exec
UPDATE zdx_tasks
SET status = 'done', test_plan = $2, test_refs = $3, completed_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: MarkTaskUndone :exec
UPDATE zdx_tasks SET status = 'pending', completed_at = NULL, updated_at = NOW() WHERE id = $1;

-- name: DeleteTask :exec
DELETE FROM zdx_tasks WHERE id = $1;

-- name: UpdateTaskFields :exec
UPDATE zdx_tasks
SET text       = CASE WHEN @field::text = 'text'       THEN @value::text ELSE text       END,
    feature    = CASE WHEN @field::text = 'feature'    THEN @value::text ELSE feature    END,
    issue      = CASE WHEN @field::text = 'issue'      THEN @value::text ELSE issue      END,
    depends    = CASE WHEN @field::text = 'depends'    THEN @value::text ELSE depends    END,
    test_plan  = CASE WHEN @field::text = 'test_plan'  THEN @value::text ELSE test_plan  END,
    test_refs  = CASE WHEN @field::text = 'test_refs'  THEN @value::text ELSE test_refs  END,
    task_group = CASE WHEN @field::text = 'task_group' THEN @value::text ELSE task_group END,
    updated_at = NOW()
WHERE id = @id;
