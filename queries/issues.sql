-- name: ListIssues :many
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues WHERE project_id = $1 ORDER BY updated_at DESC;


-- name: ListOpenIssues :many
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues WHERE project_id = $1 AND status = 'open' ORDER BY updated_at DESC;

-- name: ListOpenIssuesEligibleForBackport :many
-- IS-825 trigger 2: when `dx branch cut` creates a new version branch, the
-- caller enumerates open dev issues that should auto-generate a backport task
-- against the new branch. Default policy (per IS-825): must-tier (priority 1)
-- and should-tier (priority 2). Priority is stored as a numeric string —
-- empty/non-numeric values fall through and are excluded by the comparison.
-- Only issues currently targeting 'dev' qualify; an issue already targeted
-- at a named branch is its own canonical home and does not get a backport
-- task on top.
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues
WHERE project_id = @project_id
  AND status = 'open'
  AND target_branch = 'dev'
  AND priority ~ '^[0-9]+$'
  AND (priority)::int <= @max_priority::int
ORDER BY (priority)::int ASC, updated_at DESC;

-- name: SearchIssues :many
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues
WHERE project_id = @project_id
  AND (title ILIKE '%' || @query::text || '%' OR context ILIKE '%' || @query::text || '%')
ORDER BY updated_at DESC;

-- name: GetIssue :one
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues WHERE project_id = $1 AND id = $2;

-- name: GetIssueByAnyProject :one
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues WHERE id = $1;

-- name: CreateIssue :one
INSERT INTO zdx_issues (id, project_id, title, context, priority, component, issue_type, status, url, source_error_id, node_ref)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref;

-- name: GetIssueBySourceErrorID :one
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
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
UPDATE zdx_issues SET status = 'closed', duplicate_of = @duplicate_of, link_of = @link_of, close_reason = @close_reason, closed_at = NOW(), updated_at = NOW() WHERE project_id = @project_id AND id = @id;

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
SELECT id, project_id, title, status, priority, component, context, created_at, issue_type, duplicate_of, url, updated_at, source_error_id, link_of, reopen_count, closed_at, interactive_only, target_branch, close_reason, node_ref
FROM zdx_issues
WHERE project_id = @project_id
  AND status = 'open'
  AND (duplicate_of = @target_id OR link_of = @target_id);

-- name: SetIssueField :exec
UPDATE zdx_issues
SET title         = CASE WHEN @field::text = 'title'         THEN @value::text ELSE title         END,
    context       = CASE WHEN @field::text = 'context'       THEN @value::text ELSE context       END,
    component     = CASE WHEN @field::text = 'component'     THEN @value::text ELSE component     END,
    issue_type    = CASE WHEN @field::text = 'issue_type'    THEN @value::text ELSE issue_type    END,
    url           = CASE WHEN @field::text = 'url'           THEN @value::text ELSE url           END,
    target_branch = CASE WHEN @field::text = 'target_branch' THEN @value::text ELSE target_branch END,
    updated_at    = NOW()
WHERE project_id = @project_id AND id = @id;

-- name: SetIssuePriority :exec
UPDATE zdx_issues SET priority = @priority, updated_at = NOW() WHERE project_id = @project_id AND id = @id;

-- name: SetIssueInteractiveOnly :exec
UPDATE zdx_issues SET interactive_only = @interactive_only, updated_at = NOW() WHERE project_id = @project_id AND id = @id;

-- name: AppendIssueWork :exec
INSERT INTO zdx_issue_work (issue_id, agent, note) VALUES ($1, $2, $3);

-- name: GetIssueWork :many
SELECT id, issue_id, agent, note, created_at FROM zdx_issue_work WHERE issue_id = $1 ORDER BY created_at;

-- name: CountSubstantiveIssueWork :one
SELECT COUNT(*) FROM zdx_issue_work WHERE issue_id = $1 AND note NOT LIKE '[%';

-- name: ListWorklogForProject :many
SELECT w.id, w.issue_id, i.title AS issue_title, w.agent, w.note, w.created_at
FROM zdx_issue_work w
JOIN zdx_issues i ON i.id = w.issue_id
WHERE i.project_id = $1
ORDER BY w.created_at DESC;

-- name: CountIssuesByStatus :many
-- metaquery: off
SELECT status, COUNT(*)::bigint AS count
FROM zdx_issues
WHERE project_id = @project_id
  AND (@component::text = '' OR component = @component::text)
GROUP BY status;

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

-- name: ListWorklogCrossProject :many
-- metaquery: off
SELECT w.id, w.issue_id, i.title AS issue_title, p.slug AS project_slug, p.name AS project_name, w.agent, w.note, w.created_at
FROM zdx_issue_work w
JOIN zdx_issues i ON i.id = w.issue_id
JOIN zdx_projects p ON i.project_id = p.id
ORDER BY w.created_at DESC
LIMIT $1 OFFSET $2;



-- name: CountOpenIssuesByTitle :one
SELECT count(*) FROM zdx_issues WHERE project_id = $1 AND title = $2 AND closed_at IS NULL;

-- name: FindOpenIssueByTitle :one
-- Returns the first open issue whose title matches exactly. Used by the standup
-- yield-alert auto-file loop to dedup across runs by stable title (the breach
-- label, no current value) and append a fresh-reading comment instead of
-- duplicating the issue.
SELECT id FROM zdx_issues
WHERE project_id = $1 AND title = $2 AND closed_at IS NULL
ORDER BY id ASC LIMIT 1;

-- name: ListHistoricalCloseGateOffenders :many
-- IS-632 retroactive close-gate audit. For each closed issue where
-- close_reason is empty (not force-closed) and issue_type is not
-- 'tracker'/'ops', evaluate the IS-560 close-gate predicates and emit
-- one row per (issue, gate) offense:
--   no-worklog   — zero substantive work-log entries (notes not '[...]')
--   open-tasks   — has any task still in ready/wip/active
--   missing-demo — impl issue with must-specs lacking a passing demo
SELECT i.id::text   AS issue_id,
       'no-worklog'::text AS gate,
       ''::text     AS detail
FROM zdx_issues i
WHERE i.project_id = @project_id
  AND i.status = 'closed'
  AND i.close_reason = ''
  AND i.issue_type NOT IN ('tracker','ops')
  AND NOT EXISTS (
    SELECT 1 FROM zdx_issue_work w
    WHERE w.issue_id = i.id AND w.note NOT LIKE '[%'
  )
UNION ALL
SELECT i.id::text   AS issue_id,
       'open-tasks'::text AS gate,
       (SELECT t.id || ' (' || t.status || ')'
          FROM zdx_tasks t
         WHERE t.issue = i.id AND t.project_id = @project_id
           AND t.status IN ('ready','wip','active')
         ORDER BY t.id LIMIT 1) AS detail
FROM zdx_issues i
WHERE i.project_id = @project_id
  AND i.status = 'closed'
  AND i.close_reason = ''
  AND i.issue_type NOT IN ('tracker','ops')
  AND EXISTS (
    SELECT 1 FROM zdx_tasks t
    WHERE t.issue = i.id AND t.project_id = @project_id
      AND t.status IN ('ready','wip','active')
  )
UNION ALL
SELECT i.id::text   AS issue_id,
       'missing-demo'::text AS gate,
       (SELECT s.id::text
          FROM zdx_tasks t
          JOIN zdx_features f ON f.name = t.feature AND f.project_id = @project_id
          JOIN zdx_specs s ON s.feature_id = f.id AND s.importance = 'must'
         WHERE t.issue = i.id AND t.project_id = @project_id
           AND NOT EXISTS (
             SELECT 1 FROM zdx_spec_deferrals sd
             JOIN zdx_issues di ON di.id = sd.issue_id
             WHERE sd.spec_id = s.id AND di.status = 'open'
           )
           AND NOT EXISTS (
             SELECT 1 FROM zdx_spec_tests st
             JOIN zdx_tests tt ON tt.id = st.test_id
             WHERE st.spec_id = s.id AND tt.status = 'pass'
               AND (tt.component = 'demo' OR EXISTS (
                 SELECT 1 FROM zdx_test_demos td WHERE td.test_id = tt.id
               ))
           )
         ORDER BY s.id LIMIT 1) AS detail
FROM zdx_issues i
WHERE i.project_id = @project_id
  AND i.status = 'closed'
  AND i.close_reason = ''
  AND i.issue_type = 'impl'
  AND EXISTS (
    SELECT 1
    FROM zdx_tasks t
    JOIN zdx_features f ON f.name = t.feature AND f.project_id = @project_id
    JOIN zdx_specs s ON s.feature_id = f.id AND s.importance = 'must'
    WHERE t.issue = i.id AND t.project_id = @project_id
      AND NOT EXISTS (
        SELECT 1 FROM zdx_spec_deferrals sd
        JOIN zdx_issues di ON di.id = sd.issue_id
        WHERE sd.spec_id = s.id AND di.status = 'open'
      )
      AND NOT EXISTS (
        SELECT 1 FROM zdx_spec_tests st
        JOIN zdx_tests tt ON tt.id = st.test_id
        WHERE st.spec_id = s.id AND tt.status = 'pass'
          AND (tt.component = 'demo' OR EXISTS (
            SELECT 1 FROM zdx_test_demos td WHERE td.test_id = tt.id
          ))
      )
  )
ORDER BY issue_id, gate;

-- name: ListForceClosedNoSubstance :many
-- Closed issues with a non-done close reason (wontfix/duplicate/link) in their
-- work-log and zero substantive work-log entries (notes that don't start with
-- '['). Used by doctor's planning rung to surface accountability gaps.
SELECT i.id,
       i.title,
       (SELECT w.note
          FROM zdx_issue_work w
         WHERE w.issue_id = i.id
           AND w.note LIKE '[closed:%'
         ORDER BY w.created_at DESC
         LIMIT 1) AS close_note
FROM zdx_issues i
WHERE i.project_id = $1
  AND i.status = 'closed'
  AND EXISTS (
    SELECT 1 FROM zdx_issue_work w
    WHERE w.issue_id = i.id
      AND (w.note LIKE '[closed:wontfix]%'
        OR w.note LIKE '[closed:duplicate]%'
        OR w.note LIKE '[closed:link]%')
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_issue_work w
    WHERE w.issue_id = i.id
      AND w.note NOT LIKE '[%'
  )
ORDER BY i.closed_at DESC NULLS LAST, i.id;
