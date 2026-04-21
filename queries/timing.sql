-- name: UpsertTimed :exec
INSERT INTO zdx_timed (project_id, component, environment, name, duration_ms, source, context_json, count, total_ms)
VALUES (@project_id, @component, @environment, @name, @duration_ms, @source, @context_json, 1, @total_ms)
ON CONFLICT (COALESCE(project_id, 0), component, environment, name)
DO UPDATE SET
  duration_ms  = GREATEST(zdx_timed.duration_ms, EXCLUDED.duration_ms),
  source       = CASE WHEN EXCLUDED.duration_ms > zdx_timed.duration_ms THEN EXCLUDED.source ELSE zdx_timed.source END,
  context_json = CASE WHEN EXCLUDED.duration_ms > zdx_timed.duration_ms THEN EXCLUDED.context_json ELSE zdx_timed.context_json END,
  count        = zdx_timed.count + 1,
  total_ms     = zdx_timed.total_ms + EXCLUDED.duration_ms,
  created_at   = NOW();

-- name: ListTimed :many
-- metaquery:agg Grouped group_by_expr(group_value, "context_json->>?", string) count(entry_count) max(max_ms, duration_ms) sum(sum_total_ms, total_ms) sum(sum_count, count)
SELECT id, project_id, component, environment, name, duration_ms, count, total_ms, source, context_json, created_at
FROM zdx_timed
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
ORDER BY duration_ms DESC
;

-- name: ListTimedDistinctTagKeys :many
SELECT DISTINCT k AS tag_key
FROM zdx_timed, jsonb_object_keys(context_json) AS k
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY tag_key;

-- name: ListTimedDistinctTagValues :many
SELECT DISTINCT context_json->>@tag_key::text AS tag_value
FROM zdx_timed
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND context_json ? @tag_key::text
  AND context_json->>@tag_key::text IS NOT NULL
ORDER BY tag_value;

-- name: InsertTimedEvent :exec
INSERT INTO zdx_timed_events (project_id, component, environment, name, duration_ms, source, context_json)
VALUES (@project_id, @component, @environment, @name, @duration_ms, @source, @context_json);

-- name: InsertTimedEventAt :exec
INSERT INTO zdx_timed_events (project_id, component, environment, name, duration_ms, source, context_json, created_at)
VALUES (@project_id, @component, @environment, @name, @duration_ms, @source, @context_json, @created_at);

-- name: ListTimedEvents :many
-- metaquery:agg Grouped group_by_expr(group_value, "context_json->>?", string) count(entry_count) max(max_ms, duration_ms) sum(sum_ms, duration_ms) min(first_seen, created_at) max(last_seen, created_at)
SELECT id, project_id, component, environment, name, duration_ms, source, context_json, created_at
FROM zdx_timed_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
ORDER BY created_at DESC
;

-- name: DeleteTimedEventsOlderThan :execrows
DELETE FROM zdx_timed_events
WHERE created_at < @cutoff::timestamptz;
