-- name: InsertErrorReport :one
INSERT INTO zdx_error_reports (project_id, source, endpoint, error_name, stack_trace)
VALUES (@project_id, @source, @endpoint, @error_name, @stack_trace)
RETURNING id, project_id, source, endpoint, error_name, stack_trace, created_at;

-- name: ListErrorReports :many
SELECT id, project_id, source, endpoint, error_name, stack_trace, created_at
FROM zdx_error_reports
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT 200;

-- name: CountErrorReports :one
SELECT count(*) FROM zdx_error_reports WHERE project_id = $1;

-- name: ListErrorReportsPaginated :many
SELECT id, project_id, source, endpoint, error_name, stack_trace, created_at
FROM zdx_error_reports
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: DeleteErrorReports :exec
DELETE FROM zdx_error_reports WHERE project_id = $1;

-- name: InsertSlowQuery :one
INSERT INTO zdx_slow_queries (project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json)
VALUES (@project_id, @sql_hash, @sql_text, @endpoint, @duration_ms, @explain_json)
RETURNING id, project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json, created_at;

-- name: ListSlowQueries :many
SELECT id, project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json, created_at
FROM zdx_slow_queries
WHERE project_id = $1
ORDER BY duration_ms DESC
LIMIT 200;

-- name: CountSlowQueries :one
SELECT count(*) FROM zdx_slow_queries WHERE project_id = $1;

-- name: ListSlowQueriesPaginated :many
SELECT id, project_id, sql_hash, sql_text, endpoint, duration_ms, explain_json, created_at
FROM zdx_slow_queries
WHERE project_id = $1
ORDER BY duration_ms DESC
LIMIT $2 OFFSET $3;
