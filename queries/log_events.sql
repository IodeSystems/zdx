-- name: InsertLogEvent :exec
INSERT INTO zdx_log_events (project_id, component, environment, level, message, source, context_json)
VALUES (@project_id, @component, @environment, @level, @message, @source, @context_json);

-- name: ListLogEvents :many
-- metaquery:agg Grouped group_by_expr(group_value, "context_json->>?", string) count(entry_count) min(first_seen, created_at) max(last_seen, created_at)
SELECT id, project_id, component, environment, level, message, source, context_json, created_at
FROM zdx_log_events
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
  AND (sqlc.narg(tag_filter)::jsonb IS NULL OR context_json @> sqlc.narg(tag_filter)::jsonb)
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at < sqlc.narg(until)::timestamptz)
ORDER BY created_at DESC
;

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

-- name: IneffectiveSessionsByWeek :many
-- Per-week count of session.ineffective tracelogs (IS-1100), bucketed by
-- date_trunc('week', created_at) and grouped by the persona/model/complexity
-- tags stamped into context_json by ineffectiveTraceArgs. v1: persona may be
-- "" until IS-1096 stamps it on every session; pass through as-is. Feeds the
-- routing-distribution dashboard for IS-1101.
SELECT
    date_trunc('week', created_at)::timestamptz       AS week_start,
    COALESCE(context_json->>'persona', '')::text     AS persona,
    COALESCE(context_json->>'model', '')::text       AS model,
    COALESCE(context_json->>'complexity', '')::text  AS complexity,
    COUNT(*)                                          AS ineffective_count
FROM zdx_log_events
WHERE message = 'session.ineffective'
  AND project_id = @project_id::int
  AND (sqlc.narg(since)::timestamptz IS NULL OR created_at >= sqlc.narg(since)::timestamptz)
  AND (sqlc.narg(until)::timestamptz IS NULL OR created_at <  sqlc.narg(until)::timestamptz)
GROUP BY week_start, persona, model, complexity
ORDER BY week_start DESC, ineffective_count DESC;
