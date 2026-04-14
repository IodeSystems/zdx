-- name: ListIssues :many
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at, issue_type, duplicate_of
FROM zdx_issues WHERE project_id = $1 ORDER BY priority NULLS LAST, created_at;

-- name: CountIssues :one
SELECT count(*) FROM zdx_issues WHERE project_id = $1;

-- name: ListIssuesPaginated :many
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at, issue_type, duplicate_of
FROM zdx_issues WHERE project_id = $1 ORDER BY priority NULLS LAST, created_at
LIMIT $2 OFFSET $3;

-- name: ListOpenIssues :many
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at, issue_type, duplicate_of
FROM zdx_issues WHERE project_id = $1 AND status = 'open' ORDER BY priority NULLS LAST, created_at;

-- name: SearchIssues :many
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at, issue_type, duplicate_of
FROM zdx_issues
WHERE project_id = @project_id
  AND (title ILIKE '%' || @query::text || '%' OR context ILIKE '%' || @query::text || '%')
ORDER BY priority NULLS LAST, created_at
LIMIT 20;

-- name: GetIssue :one
SELECT id, project_id, title, status, priority, component, context, blocked_by, created_at, issue_type, duplicate_of
FROM zdx_issues WHERE project_id = $1 AND id = $2;

-- name: CreateIssue :one
INSERT INTO zdx_issues (id, project_id, title, context, priority, component, issue_type, blocked_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, project_id, title, status, priority, component, context, blocked_by, created_at, issue_type, duplicate_of;

-- name: UpdateIssue :exec
UPDATE zdx_issues
SET title      = COALESCE(NULLIF(@title, ''),      title),
    context    = COALESCE(NULLIF(@context, ''),    context),
    priority   = COALESCE(NULLIF(@priority, ''),   priority),
    blocked_by = COALESCE(NULLIF(@blocked_by, ''), blocked_by),
    issue_type = COALESCE(NULLIF(@issue_type, ''), issue_type)
WHERE project_id = @project_id AND id = @id;

-- name: CloseIssue :exec
UPDATE zdx_issues SET status = 'closed', duplicate_of = @duplicate_of WHERE project_id = @project_id AND id = @id;

-- name: ReopenIssue :exec
UPDATE zdx_issues SET status = 'open' WHERE project_id = $1 AND id = $2;

-- name: SetIssueField :exec
UPDATE zdx_issues
SET title      = CASE WHEN @field::text = 'title'      THEN @value::text ELSE title      END,
    context    = CASE WHEN @field::text = 'context'    THEN @value::text ELSE context    END,
    component  = CASE WHEN @field::text = 'component'  THEN @value::text ELSE component  END,
    blocked_by = CASE WHEN @field::text = 'blocked_by' THEN @value::text ELSE blocked_by END,
    issue_type = CASE WHEN @field::text = 'issue_type' THEN @value::text ELSE issue_type END
WHERE project_id = @project_id AND id = @id;

-- name: SetIssuePriority :exec
UPDATE zdx_issues SET priority = @priority WHERE project_id = @project_id AND id = @id;

-- name: AppendIssueWork :exec
INSERT INTO zdx_issue_work (issue_id, agent, note) VALUES ($1, $2, $3);

-- name: GetIssueWork :many
SELECT id, issue_id, agent, note, created_at FROM zdx_issue_work WHERE issue_id = $1 ORDER BY created_at;

-- name: ListWorklogForProject :many
SELECT w.id, w.issue_id, i.title AS issue_title, w.agent, w.note, w.created_at
FROM zdx_issue_work w
JOIN zdx_issues i ON i.id = w.issue_id
WHERE i.project_id = $1
ORDER BY w.created_at DESC
LIMIT 200;

-- name: CountWorklogForProject :one
SELECT count(*) FROM zdx_issue_work w JOIN zdx_issues i ON i.id = w.issue_id WHERE i.project_id = $1;

-- name: ListWorklogForProjectPaginated :many
SELECT w.id, w.issue_id, i.title AS issue_title, w.agent, w.note, w.created_at
FROM zdx_issue_work w
JOIN zdx_issues i ON i.id = w.issue_id
WHERE i.project_id = $1
ORDER BY w.created_at DESC
LIMIT $2 OFFSET $3;
