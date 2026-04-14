-- name: InsertLogEvent :exec
INSERT INTO zdx_log_events (project_id, component, environment, level, message, source, context_json)
VALUES (@project_id, @component, @environment, @level, @message, @source, @context_json);

-- name: ListLogEvents :many
SELECT id, project_id, component, environment, level, message, source, context_json, created_at
FROM zdx_log_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountLogEvents :one
SELECT count(*) FROM zdx_log_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz);

-- name: ListLogEventsGrouped :many
SELECT
  context_json->>@group_key::text AS group_value,
  count(*)::int AS entry_count,
  min(created_at) AS first_seen,
  max(created_at) AS last_seen
FROM zdx_log_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
  AND context_json ? @group_key::text
GROUP BY group_value
ORDER BY entry_count DESC;

-- name: ListLogEventsDistinctTagKeys :many
SELECT DISTINCT k AS tag_key
FROM zdx_log_events, jsonb_object_keys(context_json) AS k
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY tag_key;

-- name: ListLogEventsDistinctTagValues :many
SELECT DISTINCT context_json->>@tag_key::text AS tag_value
FROM zdx_log_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND context_json ? @tag_key::text
  AND context_json->>@tag_key::text IS NOT NULL
ORDER BY tag_value;

-- name: DeleteLogEventsOlderThan :execrows
DELETE FROM zdx_log_events
WHERE created_at < @cutoff::timestamptz;
