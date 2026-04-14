-- name: UpsertCounted :exec
INSERT INTO zdx_counted (project_id, component, environment, name, value, source, context_json, count, total_value)
VALUES (@project_id, @component, @environment, @name, @value, @source, @context_json, 1, @total_value)
ON CONFLICT (COALESCE(project_id, 0), component, environment, name)
DO UPDATE SET
  value        = GREATEST(zdx_counted.value, EXCLUDED.value),
  source       = CASE WHEN EXCLUDED.value > zdx_counted.value THEN EXCLUDED.source ELSE zdx_counted.source END,
  context_json = CASE WHEN EXCLUDED.value > zdx_counted.value THEN EXCLUDED.context_json ELSE zdx_counted.context_json END,
  count        = zdx_counted.count + 1,
  total_value  = zdx_counted.total_value + EXCLUDED.value,
  created_at   = NOW();

-- name: CountCounted :one
SELECT count(*) FROM zdx_counted
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb);

-- name: ListCountedPaginated :many
SELECT id, project_id, component, environment, name, value, count, total_value, source, context_json, created_at
FROM zdx_counted
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
ORDER BY total_value DESC
LIMIT @lim OFFSET @off;

-- name: ListCountedGrouped :many
SELECT
  context_json->>@group_key::text AS group_value,
  count(*)::int AS entry_count,
  max(value) AS max_value,
  sum(total_value)::bigint AS sum_total_value,
  sum(count)::int AS sum_count
FROM zdx_counted
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND context_json ? @group_key::text
GROUP BY group_value
ORDER BY max_value DESC;

-- name: ListCountedDistinctTagKeys :many
SELECT DISTINCT k AS tag_key
FROM zdx_counted, jsonb_object_keys(context_json) AS k
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY tag_key;

-- name: ListCountedDistinctTagValues :many
SELECT DISTINCT context_json->>@tag_key::text AS tag_value
FROM zdx_counted
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND context_json ? @tag_key::text
  AND context_json->>@tag_key::text IS NOT NULL
ORDER BY tag_value;

-- name: InsertCounterEvent :exec
INSERT INTO zdx_counter_events (project_id, component, environment, name, value, source, context_json)
VALUES (@project_id, @component, @environment, @name, @value, @source, @context_json);

-- name: ListCounterEvents :many
SELECT id, project_id, component, environment, name, value, source, context_json, created_at
FROM zdx_counter_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
ORDER BY created_at DESC
LIMIT @lim OFFSET @off;

-- name: CountCounterEvents :one
SELECT count(*) FROM zdx_counter_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz);

-- name: ListCounterEventsGrouped :many
SELECT
  context_json->>@group_key::text AS group_value,
  count(*)::int AS entry_count,
  max(value) AS max_value,
  sum(value)::bigint AS sum_value,
  min(created_at) AS first_seen,
  max(created_at) AS last_seen
FROM zdx_counter_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
  AND context_json ? @group_key::text
GROUP BY group_value
ORDER BY max_value DESC;

-- name: DeleteCounterEventsOlderThan :execrows
DELETE FROM zdx_counter_events
WHERE created_at < @cutoff::timestamptz;
