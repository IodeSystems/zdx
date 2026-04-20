-- name: ListTodos :many
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count
FROM zdx_todos WHERE project_id = $1 ORDER BY priority, created_at;

-- name: ListTodosFiltered :many
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count
FROM zdx_todos
WHERE project_id = @project_id
  AND (sqlc.narg('blocked')::boolean IS NULL OR blocked = sqlc.narg('blocked')::boolean)
  AND (sqlc.narg('target_type')::text IS NULL OR target_type = sqlc.narg('target_type')::text)
  AND (sqlc.narg('issue_ref')::text IS NULL OR issue_ref = sqlc.narg('issue_ref')::text)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY priority, created_at;

-- name: DeleteTodosForProject :exec
DELETE FROM zdx_todos WHERE project_id = $1;

-- name: CreateTodo :one
INSERT INTO zdx_todos (project_id, text, title, description, key, persona, priority, status,
                       target_type, target_id, kind, issue_ref, blocked)
VALUES (@project_id, @text, @title, @description, @key, @persona, @priority, @status,
        @target_type, @target_id, @kind, @issue_ref, @blocked)
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count;

-- name: UpsertTodo :one
-- Upsert a todo item, preserving existing claim state (claimed_by, claimed_at, lease_expires_at).
-- Tracks resolve→open churn: reopen_count increments each time a resolved key is re-emitted.
-- Auto-blocks at 3+ reopens so agents don't churn indefinitely on an untriageable item.
INSERT INTO zdx_todos (project_id, text, title, description, key, persona, priority, status,
                       target_type, target_id, kind, issue_ref, blocked)
VALUES (@project_id, @text, @title, @description, @key, @persona, @priority, @status,
        @target_type, @target_id, @kind, @issue_ref, @blocked)
ON CONFLICT (project_id, key) DO UPDATE SET
  text = EXCLUDED.text,
  title = EXCLUDED.title,
  description = EXCLUDED.description,
  persona = EXCLUDED.persona,
  priority = EXCLUDED.priority,
  status = CASE WHEN zdx_todos.status = 'resolved' THEN 'open' ELSE zdx_todos.status END,
  target_type = EXCLUDED.target_type,
  target_id = EXCLUDED.target_id,
  kind = EXCLUDED.kind,
  issue_ref = EXCLUDED.issue_ref,
  blocked = CASE
    WHEN zdx_todos.status = 'resolved' AND zdx_todos.reopen_count + 1 >= 3 THEN true
    ELSE EXCLUDED.blocked
  END,
  reopen_count = CASE
    WHEN zdx_todos.status = 'resolved' THEN zdx_todos.reopen_count + 1
    ELSE zdx_todos.reopen_count
  END
  -- claimed_by, claimed_at, lease_expires_at intentionally NOT updated
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count;

-- name: ResolveTodo :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW()
WHERE project_id = $1 AND key = $2;

-- name: GetTodoByKey :one
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count
FROM zdx_todos WHERE project_id = $1 AND key = $2;

-- name: GetTodoByID :one
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count
FROM zdx_todos WHERE id = $1;

-- name: ResolveTodosNotInKeys :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW()
WHERE project_id = $1 AND status = 'open' AND key != ALL(@keys::text[]);

-- name: ClaimNextTodo :one
-- Atomically claim the highest-priority unclaimed open todo for an agent.
-- Skips locked rows (concurrent agents get different items).
UPDATE zdx_todos SET
  claimed_by = @agent_id,
  claimed_at = NOW(),
  lease_expires_at = NOW() + (@lease_minutes::int || ' minutes')::interval
WHERE id = (
  SELECT t.id FROM zdx_todos t
  WHERE t.project_id = @project_id
    AND t.status = 'open'
    AND t.blocked = false
    AND (t.claimed_by = '' OR t.lease_expires_at < NOW())
  ORDER BY t.priority, t.created_at
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count;

-- name: ReleaseTodo :exec
-- Release a claimed todo (agent finished or abandoned).
UPDATE zdx_todos SET
  claimed_by = '',
  claimed_at = NULL,
  lease_expires_at = NULL
WHERE id = $1 AND claimed_by = $2;

-- name: ReleaseTodoAdmin :exec
-- Admin release: clear the claim unconditionally (no agent_id check).
UPDATE zdx_todos SET
  claimed_by = '',
  claimed_at = NULL,
  lease_expires_at = NULL
WHERE id = $1;

-- name: RenewTodoLease :exec
-- Extend the lease on a claimed todo (heartbeat).
UPDATE zdx_todos SET
  lease_expires_at = NOW() + (@lease_minutes::int || ' minutes')::interval
WHERE id = @id AND claimed_by = @agent_id;

-- name: ResolveTodoByID :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW(),
  claimed_by = '', claimed_at = NULL, lease_expires_at = NULL
WHERE id = $1;

-- name: ReclaimExpiredTodos :many
-- Clear claims on todos whose leases have expired. Returns affected rows for reservation release.
UPDATE zdx_todos SET
  claimed_by = '',
  claimed_at = NULL,
  lease_expires_at = NULL
WHERE project_id = $1
  AND claimed_by != ''
  AND lease_expires_at < NOW()
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count;

-- name: ListActiveTodoClaims :many
-- Return all todos that are currently claimed and whose lease has not expired.
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count
FROM zdx_todos
WHERE project_id = $1
  AND claimed_by != ''
  AND lease_expires_at > NOW()
ORDER BY claimed_at DESC;

-- name: GetState :one
SELECT value FROM zdx_state WHERE project_id = $1 AND key = $2;

-- name: SetState :exec
INSERT INTO zdx_state (project_id, key, value, updated_at)
VALUES (@project_id, @key, @value, NOW())
ON CONFLICT (project_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
