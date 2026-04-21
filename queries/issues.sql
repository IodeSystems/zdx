-- name: ListIssues :many
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues WHERE project_id = $1 ORDER BY updated_at DESC;


-- name: ListOpenIssues :many
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues WHERE project_id = $1 AND status = 'open' ORDER BY updated_at DESC;

-- name: SearchIssues :many
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues
WHERE project_id = @project_id
  AND (title ILIKE '%' || @query::text || '%' OR context ILIKE '%' || @query::text || '%')
ORDER BY updated_at DESC;

-- name: GetIssue :one
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues WHERE project_id = $1 AND id = $2;

-- name: GetIssueByAnyProject :one
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues WHERE id = $1;

-- name: CreateIssue :one
INSERT INTO zdx_issues (id, project_id, title, context, priority, component, issue_type, status, url, source_error_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at;

-- name: GetIssueBySourceErrorID :one
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues WHERE project_id = $1 AND source_error_id = $2
LIMIT 1;

-- name: ReadyIssue :exec
UPDATE zdx_issues SET status = 'open', closed_at = NULL, updated_at = NOW() WHERE project_id = $1 AND id = $2 AND status = 'wip';

-- name: UpdateIssue :exec
UPDATE zdx_issues
SET title      = COALESCE(NULLIF(@title, ''),      title),
    context    = COALESCE(NULLIF(@context, ''),    context),
    priority   = COALESCE(NULLIF(@priority, ''),   priority),
    issue_type = COALESCE(NULLIF(@issue_type, ''), issue_type),
    updated_at = NOW()
WHERE project_id = @project_id AND id = @id;

-- name: CloseIssue :exec
UPDATE zdx_issues SET status = 'closed', duplicate_of = @duplicate_of, link_of = @link_of, closed_at = NOW(), updated_at = NOW() WHERE project_id = @project_id AND id = @id;

-- name: ReopenIssue :exec
UPDATE zdx_issues
SET status = 'open',
    reopen_count = reopen_count + CASE WHEN status = 'closed' THEN 1 ELSE 0 END,
    closed_at = NULL,
    updated_at = NOW()
WHERE project_id = $1 AND id = $2;

-- name: ListOpenLinkedIssues :many
-- Open issues whose duplicate_of or link_of targets the given issue. Used to
-- cascade-close narrow-slice links (and full duplicates) when the target closes.
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at
FROM zdx_issues
WHERE project_id = @project_id
  AND status = 'open'
  AND (duplicate_of = @target_id OR link_of = @target_id);

-- name: SetIssueField :exec
UPDATE zdx_issues
SET title      = CASE WHEN @field::text = 'title'      THEN @value::text ELSE title      END,
    context    = CASE WHEN @field::text = 'context'    THEN @value::text ELSE context    END,
    component  = CASE WHEN @field::text = 'component'  THEN @value::text ELSE component  END,
    issue_type = CASE WHEN @field::text = 'issue_type' THEN @value::text ELSE issue_type END,
    url        = CASE WHEN @field::text = 'url'        THEN @value::text ELSE url        END,
    updated_at = NOW()
WHERE project_id = @project_id AND id = @id;

-- name: SetIssuePriority :exec
UPDATE zdx_issues SET priority = @priority, updated_at = NOW() WHERE project_id = @project_id AND id = @id;

-- name: AppendIssueWork :exec
INSERT INTO zdx_issue_work (issue_id, agent, note) VALUES ($1, $2, $3);

-- name: GetIssueWork :many
SELECT id, issue_id, agent, note, created_at FROM zdx_issue_work WHERE issue_id = $1 ORDER BY created_at;

-- name: ListWorklogForProject :many
SELECT w.id, w.issue_id, i.title AS issue_title, w.agent, w.note, w.created_at
FROM zdx_issue_work w
JOIN zdx_issues i ON i.id = w.issue_id
WHERE i.project_id = $1
ORDER BY w.created_at DESC;

-- name: ProjectStateSummary :one
SELECT
  (SELECT count(*) FROM zdx_issues i WHERE i.project_id = $1 AND i.status = 'open') AS open_issues,
  (SELECT count(*) FROM zdx_issues i WHERE i.project_id = $1 AND i.status = 'wip') AS wip_issues,
  (SELECT count(*) FROM zdx_issues i WHERE i.project_id = $1 AND i.status = 'closed' AND i.created_at > NOW() - INTERVAL '30 days') AS recently_closed_issues,
  (SELECT count(*) FROM zdx_tasks t WHERE t.project_id = $1 AND t.status = 'ready') AS pending_tasks,
  (SELECT count(*) FROM zdx_tasks t WHERE t.project_id = $1 AND t.status = 'done') AS done_tasks,
  (SELECT count(*) FROM zdx_blocker_questions q WHERE q.project_id = $1 AND q.status = 'pending') AS pending_blockers;

-- name: TopPriorityOpenIssues :many
-- metaquery: off
SELECT id, title, priority
FROM zdx_issues
WHERE project_id = $1 AND status = 'open' AND priority != ''
ORDER BY priority, created_at
LIMIT 5;


