-- name: ListTodos :many
SELECT id, project_id, text, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, created_at, resolved_at
FROM zdx_todos WHERE project_id = $1 ORDER BY priority, created_at;

-- name: ListTodosFiltered :many
SELECT id, project_id, text, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, created_at, resolved_at
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
INSERT INTO zdx_todos (project_id, text, key, persona, priority, status,
                       target_type, target_id, kind, issue_ref, blocked,
                       claimed_by, claimed_at)
VALUES (@project_id, @text, @key, @persona, @priority, @status,
        @target_type, @target_id, @kind, @issue_ref, @blocked,
        @claimed_by, @claimed_at)
RETURNING id, project_id, text, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked,
          claimed_by, claimed_at, created_at, resolved_at;

-- name: UpsertTodo :one
INSERT INTO zdx_todos (project_id, text, key, persona, priority, status,
                       target_type, target_id, kind, issue_ref, blocked,
                       claimed_by, claimed_at)
VALUES (@project_id, @text, @key, @persona, @priority, @status,
        @target_type, @target_id, @kind, @issue_ref, @blocked,
        @claimed_by, @claimed_at)
ON CONFLICT (project_id, key) DO UPDATE SET
  text = EXCLUDED.text,
  persona = EXCLUDED.persona,
  priority = EXCLUDED.priority,
  status = EXCLUDED.status,
  target_type = EXCLUDED.target_type,
  target_id = EXCLUDED.target_id,
  kind = EXCLUDED.kind,
  issue_ref = EXCLUDED.issue_ref,
  blocked = EXCLUDED.blocked,
  claimed_by = EXCLUDED.claimed_by,
  claimed_at = EXCLUDED.claimed_at
RETURNING id, project_id, text, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked,
          claimed_by, claimed_at, created_at, resolved_at;

-- name: ResolveTodo :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW()
WHERE project_id = $1 AND key = $2;

-- name: GetTodoByKey :one
SELECT id, project_id, text, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked,
       claimed_by, claimed_at, created_at, resolved_at
FROM zdx_todos WHERE project_id = $1 AND key = $2;

-- name: ResolveTodosNotInKeys :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW()
WHERE project_id = $1 AND status = 'open' AND key != ALL(@keys::text[]);

-- name: GetState :one
SELECT value FROM zdx_state WHERE project_id = $1 AND key = $2;

-- name: SetState :exec
INSERT INTO zdx_state (project_id, key, value, updated_at)
VALUES (@project_id, @key, @value, NOW())
ON CONFLICT (project_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
