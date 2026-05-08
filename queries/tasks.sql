-- name: ListTasks :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%' OR title ILIKE '%' || @search || '%')
ORDER BY updated_at DESC;

-- name: CountClosedTasks :one
SELECT count(*) FROM zdx_tasks WHERE project_id = $1 AND status = 'done';

-- name: ListTasksByFeature :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id AND feature = @feature
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%' OR title ILIKE '%' || @search || '%')
ORDER BY updated_at DESC;

-- name: ListTasksByIssue :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id AND issue = @issue
  AND (@status_filter::text = '' OR status = @status_filter)
  AND (@search::text = '' OR text ILIKE '%' || @search || '%' OR title ILIKE '%' || @search || '%')
ORDER BY updated_at DESC;

-- name: GetTask :one
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks WHERE id = $1;

-- name: CreateTask :one
-- target_branch defaults to 'dev' when caller passes ''. Backport tasks set it
-- to the version branch they target while their source issue stays on dev.
INSERT INTO zdx_tasks (id, project_id, title, text, feature, issue, task_group, status, reason, test_plan, spec, target_branch)
VALUES (@id, @project_id, @title, @text, @feature, @issue, @task_group, @status, @reason, @test_plan, @spec,
        CASE WHEN @target_branch::text = '' THEN 'dev' ELSE @target_branch::text END)
RETURNING id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at;

-- name: ReadyTask :exec
UPDATE zdx_tasks SET status = 'ready', updated_at = NOW() WHERE id = $1 AND status = 'wip';

-- name: UpdateTaskStatus :exec
UPDATE zdx_tasks SET status = $2, reason = $3, updated_at = NOW() WHERE id = $1;

-- name: MarkTaskDone :exec
UPDATE zdx_tasks
SET status = 'done', test_plan = $2, test_refs = $3, completed_at = NOW(), updated_at = NOW()
WHERE id = $1;

-- name: MarkTaskUndone :exec
UPDATE zdx_tasks SET status = 'ready', completed_at = NULL, updated_at = NOW() WHERE id = $1;

-- name: DeleteTask :exec
DELETE FROM zdx_tasks WHERE id = $1;

-- name: DeleteDraftTask :execrows
DELETE FROM zdx_tasks WHERE id = $1 AND status = 'wip';

-- name: UpdateTaskFields :exec
UPDATE zdx_tasks
SET title         = CASE WHEN @field::text = 'title'         THEN @value::text ELSE title         END,
    text          = CASE WHEN @field::text = 'text'          THEN @value::text ELSE text          END,
    feature       = CASE WHEN @field::text = 'feature'       THEN @value::text ELSE feature       END,
    issue         = CASE WHEN @field::text = 'issue'         THEN @value::text ELSE issue         END,
    depends       = CASE WHEN @field::text = 'depends'       THEN @value::text ELSE depends       END,
    test_plan     = CASE WHEN @field::text = 'test_plan'     THEN @value::text ELSE test_plan     END,
    test_refs     = CASE WHEN @field::text = 'test_refs'     THEN @value::text ELSE test_refs     END,
    task_group    = CASE WHEN @field::text = 'task_group'    THEN @value::text ELSE task_group    END,
    target_branch = CASE WHEN @field::text = 'target_branch' THEN @value::text ELSE target_branch END,
    updated_at    = NOW()
WHERE id = @id;

-- name: ClaimTask :one
-- Atomically mark a ready, unclaimed task as active. The caller must
-- separately INSERT a zdx_reservations row to record who claimed it.
UPDATE zdx_tasks
SET status = 'active',
    updated_at = NOW()
WHERE id = (
    SELECT t.id FROM zdx_tasks t
    WHERE t.project_id = @project_id
      AND t.status = 'ready'
      AND t.task_group = @task_group
      AND NOT EXISTS (
        SELECT 1 FROM zdx_reservations r
        WHERE r.target_type = 'task'
          AND r.target_id = t.id
          AND r.released_at IS NULL
          AND r.lease_expires_at > NOW()
      )
      AND (@issue::text = '' OR t.issue = @issue)
    ORDER BY t.created_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at;

-- name: ReleaseTask :exec
UPDATE zdx_tasks SET status = 'ready', updated_at = NOW() WHERE id = $1;

-- name: ReleaseTaskAdmin :exec
UPDATE zdx_tasks SET status = 'ready', updated_at = NOW() WHERE id = $1;

-- name: ListActiveTaskClaims :many
-- Return tasks that currently have an unexpired, unreleased reservation.
SELECT t.id, t.project_id, t.title, t.text, t.feature, t.status, t.reason, t.issue, t.depends, t.test_plan, t.test_refs, t.task_group, t.spec, t.target_branch, t.created_at, t.completed_at, t.updated_at,
       r.claimed_by, r.claimed_at, r.lease_expires_at
FROM zdx_tasks t
JOIN zdx_reservations r ON r.target_type = 'task' AND r.target_id = t.id
WHERE t.project_id = $1
  AND r.released_at IS NULL
  AND r.lease_expires_at > NOW()
ORDER BY r.claimed_at DESC;

-- name: ListTasksByAgent :many
SELECT t.id, t.project_id, t.title, t.text, t.feature, t.status, t.reason, t.issue, t.depends, t.test_plan, t.test_refs, t.task_group, t.spec, t.target_branch, t.created_at, t.completed_at, t.updated_at,
       r.claimed_by, r.claimed_at, r.lease_expires_at
FROM zdx_tasks t
JOIN zdx_reservations r ON r.target_type = 'task' AND r.target_id = t.id
WHERE r.claimed_by = $1
  AND r.released_at IS NULL
ORDER BY r.claimed_at DESC;

-- name: ReclaimExpiredTasks :many
-- Reset tasks that are marked active but have no live reservation. Tasks
-- claimed by an agent currently in status='paused' or 'draining' are exempt.
-- Tasks claimed by a 'disconnected' agent within the grace window (@disconnect_grace)
-- are also exempt to allow clean reconnects. See TK-1363, TK-1365 / IS-602.
UPDATE zdx_tasks
SET status = 'ready',
    updated_at = NOW()
WHERE status = 'active'
  AND NOT EXISTS (
    SELECT 1 FROM zdx_reservations r
    WHERE r.target_type = 'task'
      AND r.target_id = zdx_tasks.id
      AND r.released_at IS NULL
      AND r.lease_expires_at > NOW()
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_reservations r
    JOIN zdx_agents a ON a.id = r.claimed_by
    WHERE r.target_type = 'task'
      AND r.target_id = zdx_tasks.id
      AND r.released_at IS NULL
      AND a.status IN ('paused', 'draining')
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_reservations r
    JOIN zdx_agents a ON a.id = r.claimed_by
    WHERE r.target_type = 'task'
      AND r.target_id = zdx_tasks.id
      AND r.released_at IS NULL
      AND a.status = 'disconnected'
      AND a.disconnect_at IS NOT NULL
      AND a.disconnect_at > NOW() - @disconnect_grace::interval
  )
  AND status != 'done'
RETURNING id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at;

-- name: CancelOrphanedTasks :many
-- Skip ready tasks with no test_refs: they were never verified. Those stay
-- open so the maturity nudge can re-discover them without creating duplicates.
UPDATE zdx_tasks t
SET status = 'done',
    reason = 'parent-closed',
    completed_at = NOW(),
    updated_at = NOW()
FROM zdx_issues i
WHERE t.issue = i.id
  AND i.status = 'closed'
  AND (
    t.status IN ('active', 'wip')
    OR (t.status = 'ready' AND t.test_refs != '')
  )
RETURNING t.id, t.project_id, t.title, t.text, t.feature, t.status, t.reason, t.issue, t.depends, t.test_plan, t.test_refs, t.task_group, t.spec, t.target_branch, t.created_at, t.completed_at, t.updated_at;

-- name: ListOpenTasksByIssue :many
SELECT id, title, status FROM zdx_tasks
WHERE issue = $1 AND project_id = $2 AND status IN ('wip','ready','active')
ORDER BY id;

-- name: ListReadyTasksWithoutTestRefsByIssue :many
-- Ready tasks linked to an issue that have no test_refs. CancelOrphanedTasks
-- skips these; surface them at close time so agents know to adopt rather than
-- re-file.
SELECT id, title
FROM zdx_tasks
WHERE project_id = @project_id
  AND issue = @issue
  AND status = 'ready'
  AND test_refs = '';

-- name: MarkTaskReviewed :exec
UPDATE zdx_tasks SET reviewed_at = NOW(), updated_at = NOW() WHERE id = $1;

-- name: ListUnreviewedDoneTasks :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at, reviewed_at
FROM zdx_tasks
WHERE project_id = $1 AND status = 'done' AND reviewed_at IS NULL
ORDER BY completed_at ASC;

-- name: ListUnreviewedDoneTasksByIssue :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at, reviewed_at
FROM zdx_tasks
WHERE project_id = $1 AND issue = $2 AND status = 'done' AND reviewed_at IS NULL
ORDER BY completed_at ASC;

-- name: GetTaskWithReview :one
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at, reviewed_at
FROM zdx_tasks WHERE id = $1;

-- name: GetTaskByExactText :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id
  AND text = @text
  AND status IN ('ready', 'active', 'wip')
  AND (@issue::text = '' OR issue = @issue)
ORDER BY created_at DESC;

-- name: ListOpenTasksByTitlePrefix :many
-- Open wip/ready/active tasks whose title starts with the given prefix
-- (case-insensitive). Used to detect templated-title duplicates such as
-- "Test spec N: ..." across sessions.
SELECT id, title, text, status, reason, issue, created_at
FROM zdx_tasks
WHERE project_id = @project_id
  AND status IN ('wip', 'ready', 'active')
  AND lower(title) LIKE lower(@prefix::text) || '%'
ORDER BY created_at DESC;

-- name: FlagStaleTasks :many
UPDATE zdx_tasks
SET stale_since = NOW(),
    updated_at = NOW()
WHERE status = 'ready'
  AND NOT EXISTS (
    SELECT 1 FROM zdx_reservations r
    WHERE r.target_type = 'task'
      AND r.target_id = zdx_tasks.id
      AND r.released_at IS NULL
      AND r.lease_expires_at > NOW()
  )
  AND stale_since IS NULL
  AND created_at <= NOW() - make_interval(days => @stale_days::int)
  AND (@project_id::int = 0 OR project_id = @project_id)
RETURNING id, text, issue;

-- name: ListStaleTasks :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at, stale_since
FROM zdx_tasks
WHERE project_id = $1
  AND stale_since IS NOT NULL
  AND status = 'ready'
ORDER BY stale_since ASC;

-- name: ListStaleTasksByIssue :many
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at, stale_since
FROM zdx_tasks
WHERE project_id = $1
  AND issue = $2
  AND stale_since IS NOT NULL
  AND status = 'ready'
ORDER BY stale_since ASC;

-- name: ClearStaleFlag :exec
UPDATE zdx_tasks SET stale_since = NULL, updated_at = NOW() WHERE id = $1;


-- name: ListOrphanReadyTasks :many
-- Ready tasks with no parent issue — invisible to the normal agent queue.
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = $1
  AND status = 'ready'
  AND (issue = '' OR issue IS NULL)
ORDER BY created_at;

-- name: AdoptTaskToIssue :exec
-- Link an orphan task to a parent issue (zdx_tasks.issue = $2).
UPDATE zdx_tasks SET issue = @issue::text, updated_at = NOW() WHERE id = @id;

-- name: SearchTasks :many
-- metaquery: off
SELECT id, project_id, title, text, feature, status, reason, issue, depends, test_plan, test_refs, task_group, spec, target_branch, created_at, completed_at, updated_at
FROM zdx_tasks
WHERE project_id = @project_id
  AND (title ILIKE '%' || @query::text || '%' OR text ILIKE '%' || @query::text || '%')
ORDER BY updated_at DESC
LIMIT 10;

-- name: CountOpenBackportTasks :one
-- IS-825 idempotency guard. Counts ready/wip/active tasks linking the given
-- source issue to the given target branch — used by the auto-backport triggers
-- (resolve-on-dev and branch-cut) to skip when a backport is already queued.
SELECT count(*) FROM zdx_tasks
WHERE project_id = @project_id
  AND issue = @issue
  AND target_branch = @target_branch
  AND status IN ('ready', 'wip', 'active');
