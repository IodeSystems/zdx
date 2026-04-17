-- name: AddIssueBlock :exec
INSERT INTO zdx_issue_blocks (issue_id, blocked_by_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveIssueBlock :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1 AND blocked_by_id = $2;

-- name: RemoveAllIssueBlocks :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockers :many
SELECT blocked_by_id FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockersWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1;

-- name: ListIssuesBlockedBy :many
SELECT issue_id FROM zdx_issue_blocks WHERE blocked_by_id = $1;
