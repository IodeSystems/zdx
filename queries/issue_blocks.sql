-- name: AddIssueBlock :exec
-- kind: 'sequencing' (default) for "X waits for Y" deps; 'composition' for tracker → child relationships
INSERT INTO zdx_issue_blocks (issue_id, blocked_by_id, kind) VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: RemoveIssueBlock :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1 AND blocked_by_id = $2;

-- name: RemoveAllIssueBlocks :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockers :many
SELECT blocked_by_id FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockersWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status, b.kind
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1;

-- name: ListIssueCompositionChildrenWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1 AND b.kind = 'composition';

-- name: ListIssueSequencingBlockersWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1 AND b.kind = 'sequencing';

-- name: ListIssuesBlockedBy :many
SELECT issue_id FROM zdx_issue_blocks WHERE blocked_by_id = $1;
