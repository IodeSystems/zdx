-- name: InsertErrorEvent :exec
INSERT INTO zdx_error_events (project_id, component, environment, name, message, stack_trace, source, context_json)
VALUES (@project_id, @component, @environment, @name, @message, @stack_trace, @source, @context_json);

-- name: ListErrorEvents :many
-- metaquery:agg Grouped group_by_expr(group_value, "context_json->>?", string) count(entry_count) min(first_seen, created_at) max(last_seen, created_at)
SELECT id, project_id, component, environment, name, message, stack_trace, source, context_json, created_at
FROM zdx_error_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
ORDER BY created_at DESC
;

-- name: ListErrorEventsDistinctTagKeys :many
SELECT DISTINCT k AS tag_key
FROM zdx_error_events, jsonb_object_keys(context_json) AS k
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY tag_key;

-- name: ListErrorEventsDistinctTagValues :many
SELECT DISTINCT context_json->>@tag_key::text AS tag_value
FROM zdx_error_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND context_json ? @tag_key::text
  AND context_json->>@tag_key::text IS NOT NULL
ORDER BY tag_value;

-- name: GetErrorEventByID :one
SELECT id, project_id, component, environment, name, message, stack_trace, source, context_json, created_at
FROM zdx_error_events
WHERE id = $1;

-- name: DeleteErrorEventsOlderThan :execrows
DELETE FROM zdx_error_events
WHERE created_at < @cutoff::timestamptz;
