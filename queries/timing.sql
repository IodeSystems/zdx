-- name: UpsertTimed :exec
INSERT INTO zdx_timed (project_id, name, duration_ms, source, context_json)
VALUES (@project_id, @name, @duration_ms, @source, @context_json)
ON CONFLICT (COALESCE(project_id, 0), name)
DO UPDATE SET
  duration_ms  = EXCLUDED.duration_ms,
  source       = EXCLUDED.source,
  context_json = EXCLUDED.context_json,
  created_at   = NOW()
WHERE EXCLUDED.duration_ms > zdx_timed.duration_ms;

-- name: ListTimed :many
SELECT id, project_id, name, duration_ms, source, context_json, created_at
FROM zdx_timed
WHERE (@project_id::int IS NULL OR project_id = @project_id)
ORDER BY duration_ms DESC;

-- name: CountTimed :one
SELECT count(*) FROM zdx_timed WHERE (@project_id::int IS NULL OR project_id = @project_id);

-- name: ListTimedPaginated :many
SELECT id, project_id, name, duration_ms, source, context_json, created_at
FROM zdx_timed
WHERE (@project_id::int IS NULL OR project_id = @project_id)
ORDER BY duration_ms DESC
LIMIT @lim OFFSET @off;
