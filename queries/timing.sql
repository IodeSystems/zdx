-- name: UpsertTimed :exec
INSERT INTO zdx_timed (project_id, name, duration_ms, source, context_json, count, total_ms)
VALUES (@project_id, @name, @duration_ms, @source, @context_json, 1, @duration_ms)
ON CONFLICT (COALESCE(project_id, 0), name)
DO UPDATE SET
  duration_ms  = GREATEST(zdx_timed.duration_ms, EXCLUDED.duration_ms),
  source       = CASE WHEN EXCLUDED.duration_ms > zdx_timed.duration_ms THEN EXCLUDED.source ELSE zdx_timed.source END,
  context_json = CASE WHEN EXCLUDED.duration_ms > zdx_timed.duration_ms THEN EXCLUDED.context_json ELSE zdx_timed.context_json END,
  count        = zdx_timed.count + 1,
  total_ms     = zdx_timed.total_ms + EXCLUDED.duration_ms,
  created_at   = NOW();

-- name: ListTimed :many
SELECT id, project_id, name, duration_ms, count, total_ms, source, context_json, created_at
FROM zdx_timed
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY duration_ms DESC;

-- name: CountTimed :one
SELECT count(*) FROM zdx_timed WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id));

-- name: ListTimedPaginated :many
SELECT id, project_id, name, duration_ms, count, total_ms, source, context_json, created_at
FROM zdx_timed
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY duration_ms DESC
LIMIT @lim OFFSET @off;
