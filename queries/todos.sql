-- name: ListTodos :many
SELECT id, project_id, feature_id, text, key, persona, priority, status, created_at, resolved_at
FROM zdx_todos WHERE project_id = $1 ORDER BY priority, created_at;

-- name: DeleteTodosForProject :exec
DELETE FROM zdx_todos WHERE project_id = $1;

-- name: CreateTodo :one
INSERT INTO zdx_todos (project_id, text, key, persona, priority, status)
VALUES (@project_id, @text, @key, @persona, @priority, @status)
RETURNING id, project_id, feature_id, text, key, persona, priority, status, created_at, resolved_at;

-- name: GetState :one
SELECT value FROM zdx_state WHERE project_id = $1 AND key = $2;

-- name: SetState :exec
INSERT INTO zdx_state (project_id, key, value, updated_at)
VALUES (@project_id, @key, @value, NOW())
ON CONFLICT (project_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
