-- name: InsertReservation :one
INSERT INTO zdx_reservations (project_id, target_type, target_id, claimed_by, claimed_at, lease_expires_at)
VALUES (@project_id, @target_type, @target_id, @claimed_by, NOW(), @lease_expires_at)
RETURNING *;

-- name: ReleaseReservation :exec
-- Mark a reservation as released by (project_id, target_type, target_id) where released_at is NULL.
UPDATE zdx_reservations SET released_at = NOW()
WHERE project_id = @project_id
  AND target_type = @target_type
  AND target_id = @target_id
  AND released_at IS NULL;

-- name: ListReservations :many
-- metaquery: off
-- Return all reservations for a project, most recent first.
SELECT id, project_id, target_type, target_id, claimed_by, claimed_at, released_at, lease_expires_at
FROM zdx_reservations
WHERE project_id = @project_id
ORDER BY claimed_at DESC
LIMIT @lim;

-- name: ListReservationsByTodoKey :many
-- Return all reservation history rows for a todo identified by its stable key,
-- most recent first. Joins in the claude session (if any) that shared the reservation window.
SELECT
  r.id,
  r.target_type,
  r.target_id,
  r.claimed_by,
  r.claimed_at,
  r.released_at,
  r.lease_expires_at,
  cs.id AS session_id,
  cs.status AS session_status,
  cs.closed_at AS session_closed_at,
  cs.header AS session_header,
  cs.alias AS session_alias
FROM zdx_reservations r
JOIN zdx_todos t ON r.target_type = 'todo' AND r.target_id = t.id::text
LEFT JOIN zdx_claude_sessions cs ON cs.todo_id = t.id AND cs.project_id = r.project_id
WHERE r.project_id = @project_id
  AND t.project_id = @project_id
  AND t.key = @key
ORDER BY r.claimed_at DESC;

-- name: CountReservationsForTodo :one
-- Count how many times a todo has been claimed (reservation count).
SELECT count(*)::int AS claim_count
FROM zdx_reservations r
JOIN zdx_todos t ON r.target_type = 'todo' AND r.target_id = t.id::text
WHERE r.project_id = @project_id
  AND t.project_id = @project_id
  AND t.id = @todo_id;

-- name: RenewTaskReservation :exec
-- Extend the lease on the active reservation for a task claimed by a specific agent.
UPDATE zdx_reservations
SET lease_expires_at = NOW() + @lease_duration::interval
WHERE target_type = 'task'
  AND target_id = @target_id
  AND claimed_by = @claimed_by
  AND released_at IS NULL;

-- name: GetActiveIssueReservation :one
-- Return the active (unreleased, unexpired) reservation for a specific issue, if any.
SELECT id, project_id, target_type, target_id, claimed_by, claimed_at, released_at, lease_expires_at
FROM zdx_reservations
WHERE project_id = @project_id
  AND target_type = 'issue'
  AND target_id = @target_id
  AND released_at IS NULL
  AND lease_expires_at > NOW()
ORDER BY claimed_at DESC
LIMIT 1;

-- name: RenewIssueReservation :exec
-- Extend the lease on the active reservation for an issue.
UPDATE zdx_reservations
SET lease_expires_at = NOW() + @lease_duration::interval
WHERE project_id = @project_id
  AND target_type = 'issue'
  AND target_id = @target_id
  AND claimed_by = @claimed_by
  AND released_at IS NULL;

-- name: ReleaseExpiredReservations :many
-- Batch release every reservation whose lease has expired and is not yet released.
-- Covers todo, task, and issue target types in a single pass.
UPDATE zdx_reservations
SET released_at = NOW()
WHERE lease_expires_at < NOW()
  AND released_at IS NULL
RETURNING id, project_id, target_type, target_id, claimed_by;

-- name: ListReservationsByIssue :many
-- Return reservations for todos linked to a specific issue, with optional agent session info.
SELECT
  r.id,
  r.target_type,
  r.target_id,
  r.claimed_by,
  r.claimed_at,
  r.released_at,
  r.lease_expires_at,
  t.text AS todo_text,
  cs.id AS session_id,
  cs.status AS session_status,
  cs.closed_at AS session_closed_at,
  cs.header AS session_header,
  cs.alias AS session_alias
FROM zdx_reservations r
JOIN zdx_todos t ON r.target_type = 'todo' AND r.target_id = t.id::text AND t.project_id = @project_id
LEFT JOIN zdx_claude_sessions cs ON cs.todo_id = t.id AND cs.project_id = @project_id
WHERE r.project_id = @project_id
  AND t.target_type = 'issue'
  AND t.target_id = @issue_id
ORDER BY r.claimed_at DESC;

-- name: ListReservationsByTaskID :many
-- Return all reservation history for a specific task, most recent first.
SELECT id, project_id, target_type, target_id, claimed_by, claimed_at, released_at, lease_expires_at
FROM zdx_reservations
WHERE project_id = @project_id
  AND target_type = 'task'
  AND target_id = @task_id
ORDER BY claimed_at DESC;

-- name: ListReservationsByIssueID :many
-- Return all reservation history for a specific issue (direct issue claims), most recent first.
SELECT id, project_id, target_type, target_id, claimed_by, claimed_at, released_at, lease_expires_at
FROM zdx_reservations
WHERE project_id = @project_id
  AND target_type = 'issue'
  AND target_id = @issue_id
ORDER BY claimed_at DESC;

-- name: ExpireTaskLeaseForTest :exec
-- Backdate the active task reservation to 1 minute ago so reclaim-expired sees it as expired.
-- Only for use by the test-mode endpoint; never call from production paths.
UPDATE zdx_reservations
SET lease_expires_at = NOW() - interval '1 minute'
WHERE target_type = 'task'
  AND target_id = @task_id
  AND released_at IS NULL;
