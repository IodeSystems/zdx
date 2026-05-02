-- name: AddTodoIncompleteReport :one
INSERT INTO zdx_todo_incomplete_reports (
    project_id, todo_id, reason, explanation, suggested_next, evidence,
    evidence_fingerprint, agent_id
) VALUES (
    @project_id, @todo_id, @reason, @explanation, @suggested_next, @evidence,
    @evidence_fingerprint, @agent_id
)
RETURNING id, project_id, todo_id, reason, explanation, suggested_next, evidence,
          evidence_fingerprint, agent_id, created_at;

-- name: ListTodoIncompleteReports :many
SELECT id, project_id, todo_id, reason, explanation, suggested_next, evidence,
       evidence_fingerprint, agent_id, created_at
FROM zdx_todo_incomplete_reports
WHERE project_id = @project_id
  AND (sqlc.narg('reason')::text IS NULL OR reason = sqlc.narg('reason')::text)
ORDER BY created_at DESC;

-- name: GetTodoIncompleteReportsByTodo :many
SELECT id, project_id, todo_id, reason, explanation, suggested_next, evidence,
       evidence_fingerprint, agent_id, created_at
FROM zdx_todo_incomplete_reports
WHERE todo_id = $1
ORDER BY created_at DESC;

-- name: AggregateTodoIncompleteReports :many
SELECT reason,
       evidence_fingerprint,
       COUNT(*)::bigint                    AS total_count,
       array_agg(DISTINCT todo_id)::int[]  AS affected_todo_ids,
       MAX(created_at)                     AS last_seen,
       ((array_agg(suggested_next ORDER BY created_at DESC)
            FILTER (WHERE suggested_next != '{}'::jsonb))[1])::jsonb AS suggested_next
FROM zdx_todo_incomplete_reports
WHERE project_id = @project_id
  AND (sqlc.narg(reason)::text IS NULL OR reason = sqlc.narg(reason)::text)
GROUP BY reason, evidence_fingerprint
ORDER BY total_count DESC, last_seen DESC;
