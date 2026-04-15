-- name: AddIssueBlock :exec
INSERT INTO zdx_issue_blocks (issue_id, blocked_by_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveIssueBlock :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1 AND blocked_by_id = $2;

-- name: RemoveAllIssueBlocks :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockers :many
SELECT blocked_by_id FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssuesBlockedBy :many
SELECT issue_id FROM zdx_issue_blocks WHERE blocked_by_id = $1;
