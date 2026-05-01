-- name: CreateIssueResolution :one
INSERT INTO zdx_issue_resolutions (id, issue_id, branch_of_origin, resolved_at, author, source, parent_resolution_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: AddResolutionCommit :exec
INSERT INTO zdx_issue_resolution_commits (resolution_id, sha, ord)
VALUES ($1, $2, $3);

-- name: ListIssueResolutions :many
SELECT r.id, r.issue_id, r.branch_of_origin, r.resolved_at, r.author, r.source, r.parent_resolution_id, r.created_at
FROM zdx_issue_resolutions r
WHERE r.issue_id = $1
ORDER BY r.resolved_at DESC;

-- name: GetIssueResolution :one
SELECT r.id, r.issue_id, r.branch_of_origin, r.resolved_at, r.author, r.source, r.parent_resolution_id, r.created_at
FROM zdx_issue_resolutions r
WHERE r.id = $1;

-- name: ListResolutionCommits :many
SELECT resolution_id, sha, ord
FROM zdx_issue_resolution_commits
WHERE resolution_id = $1
ORDER BY ord;

-- name: DeleteIssueResolution :exec
DELETE FROM zdx_issue_resolutions WHERE id = $1;

-- name: ListResolutionsByProject :many
SELECT r.id, r.issue_id, r.branch_of_origin, r.resolved_at, r.author, r.source, r.parent_resolution_id, r.created_at
FROM zdx_issue_resolutions r
JOIN zdx_issues i ON i.id = r.issue_id
WHERE i.project_id = $1
ORDER BY r.resolved_at DESC;

-- name: CountIssueResolutions :one
SELECT count(*) FROM zdx_issue_resolutions WHERE issue_id = $1;

-- name: ListUnresolvedNamedBranchesForIssue :many
SELECT vb.name
FROM zdx_version_branches vb
WHERE vb.project_id = $1
  AND vb.type = 'named'
  AND vb.status = 'active'
  AND NOT EXISTS (
    SELECT 1 FROM zdx_issue_resolutions r
    WHERE r.issue_id = $2
      AND r.branch_of_origin = vb.name
  )
ORDER BY vb.name;
