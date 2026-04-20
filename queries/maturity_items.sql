-- name: ListMaturityItems :many
SELECT id, project_id, kind, target_type, target_id, title, description,
       status, justification, snooze_until, source_question, priority_hint,
       created_at, updated_at
FROM zdx_maturity_items
WHERE project_id = $1
  AND (sqlc.arg(status_filter)::text = '' OR status = sqlc.arg(status_filter)::text)
ORDER BY priority_hint, created_at, id;

-- name: ListOpenMaturityItems :many
SELECT id, project_id, kind, target_type, target_id, title, description,
       status, justification, snooze_until, source_question, priority_hint,
       created_at, updated_at
FROM zdx_maturity_items
WHERE project_id = $1 AND status = 'open'
ORDER BY priority_hint, created_at, id;

-- name: FlipExpiredMaturityItems :exec
-- Flip snoozed items whose snooze_until has passed back to 'open' so they
-- resurface in the solo queue.
UPDATE zdx_maturity_items
SET status = 'open',
    snooze_until = NULL,
    updated_at = now()
WHERE project_id = $1
  AND status = 'snoozed'
  AND snooze_until IS NOT NULL
  AND snooze_until < now();

-- name: GetMaturityItem :one
SELECT id, project_id, kind, target_type, target_id, title, description,
       status, justification, snooze_until, source_question, priority_hint,
       created_at, updated_at
FROM zdx_maturity_items
WHERE id = $1;

-- name: UpsertMaturityItem :one
-- Re-stamping is idempotent: if the (project, kind, target) triple already
-- exists, we refresh the descriptive/priority fields and updated_at but
-- preserve status, justification, and snooze_until (caller-managed state).
INSERT INTO zdx_maturity_items
    (project_id, kind, target_type, target_id, title, description,
     source_question, priority_hint)
VALUES
    (@project_id, @kind, @target_type, sqlc.narg(target_id)::integer,
     @title, @description, @source_question, @priority_hint)
ON CONFLICT (project_id, kind, target_type, COALESCE(target_id, 0))
DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    source_question = EXCLUDED.source_question,
    priority_hint = EXCLUDED.priority_hint,
    updated_at = now()
RETURNING id, project_id, kind, target_type, target_id, title, description,
          status, justification, snooze_until, source_question, priority_hint,
          created_at, updated_at;

-- name: UpdateMaturityItemStatus :one
UPDATE zdx_maturity_items
SET status = @status,
    justification = @justification,
    snooze_until = sqlc.narg(snooze_until)::timestamptz,
    updated_at = now()
WHERE id = @id
RETURNING id, project_id, kind, target_type, target_id, title, description,
          status, justification, snooze_until, source_question, priority_hint,
          created_at, updated_at;
