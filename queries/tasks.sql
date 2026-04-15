-- name: ListTasks :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 ORDER BY updated_at DESC;

-- name: CountTasks :one
SELECT count(*) FROM zdx_tasks
WHERE project_id = @project_id
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%');

-- name: CountClosedTasks :one
SELECT count(*) FROM zdx_tasks WHERE project_id = $1 AND status = 'done';

-- name: ListTasksPaginated :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%')
ORDER BY updated_at DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: ListTasksByFeature :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 AND feature = $2 ORDER BY updated_at DESC;

-- name: CountTasksByFeature :one
SELECT count(*) FROM zdx_tasks
WHERE project_id = @project_id AND feature = @feature
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%');

-- name: ListTasksByFeaturePaginated :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id AND feature = @feature
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%')
ORDER BY updated_at DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: ListTasksByIssue :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE project_id = $1 AND issue = $2 ORDER BY updated_at DESC;

-- name: CountTasksByIssue :one
SELECT count(*) FROM zdx_tasks
WHERE project_id = @project_id AND issue = @issue
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%');

-- name: ListTasksByIssuePaginated :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id AND issue = @issue
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%')
ORDER BY updated_at DESC
LIMIT @page_limit OFFSET @page_offset;

-- name: GetTask :one
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks WHERE id = $1;

-- name: CreateTask :one
INSERT INTO zdx_tasks (id, project_id, text, feature, issue, task_group, status)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at;

-- name: ReadyTask :exec
UPDATE zdx_tasks SET status = 'pending', updated_at = NOW() WHERE id = $1 AND status = 'wip';

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

-- name: ClaimTask :one
UPDATE zdx_tasks
SET claimed_by = @agent_id,
    claimed_at = NOW(),
    lease_expires_at = NOW() + @lease_duration::interval,
    status = 'active',
    updated_at = NOW()
WHERE id = (
    SELECT t.id FROM zdx_tasks t
    WHERE t.project_id = @project_id
      AND t.status = 'pending'
      AND t.task_group = @task_group
      AND (t.claimed_by IS NULL OR t.lease_expires_at < NOW())
      AND (@issue::text = '' OR t.issue = @issue)
    ORDER BY t.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: ReleaseTask :exec
UPDATE zdx_tasks
SET claimed_by = NULL,
    claimed_at = NULL,
    lease_expires_at = NULL,
    status = 'pending',
    updated_at = NOW()
WHERE id = $1 AND claimed_by = $2;

-- name: RenewTaskLease :exec
UPDATE zdx_tasks
SET lease_expires_at = NOW() + @lease_duration::interval,
    updated_at = NOW()
WHERE id = @id AND claimed_by = @agent_id;

-- name: ListTasksByAgent :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, claimed_by, claimed_at, lease_expires_at, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE claimed_by = $1
ORDER BY claimed_at DESC;

-- name: ReclaimExpiredTasks :many
UPDATE zdx_tasks
SET claimed_by = NULL,
    claimed_at = NULL,
    lease_expires_at = NULL,
    status = 'pending',
    updated_at = NOW()
WHERE lease_expires_at < NOW()
  AND claimed_by IS NOT NULL
  AND status != 'done'
RETURNING *;

-- name: CancelOrphanedTasks :many
UPDATE zdx_tasks t
SET status = 'done',
    reason = 'parent-closed',
    completed_at = NOW(),
    claimed_by = NULL,
    claimed_at = NULL,
    lease_expires_at = NULL,
    updated_at = NOW()
FROM zdx_issues i
WHERE t.issue = i.id
  AND i.status = 'closed'
  AND t.status IN ('pending', 'active', 'wip')
RETURNING t.*;

-- name: MarkTaskReviewed :exec
UPDATE zdx_tasks SET reviewed_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: ListUnreviewedDoneTasks :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at, reviewed_at
FROM zdx_tasks
WHERE project_id = $1 AND status = 'done' AND reviewed_at IS NULL
ORDER BY completed_at ASC;

-- name: ListUnreviewedDoneTasksByIssue :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at, reviewed_at
FROM zdx_tasks
WHERE project_id = $1 AND issue = $2 AND status = 'done' AND reviewed_at IS NULL
ORDER BY completed_at ASC;

-- name: GetTaskWithReview :one
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at, reviewed_at
FROM zdx_tasks WHERE id = $1;

-- name: GetTaskByExactText :many
SELECT id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id
  AND text = @text
  AND status IN ('pending', 'active', 'wip', 'done')
  AND (@issue::text = '' OR issue = @issue)
ORDER BY created_at DESC;
