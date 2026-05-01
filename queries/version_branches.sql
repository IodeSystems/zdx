-- name: CreateVersionBranch :one
INSERT INTO zdx_version_branches (project_id, name, type, semver, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, name, type, semver, status, created_at;

-- name: GetVersionBranchByName :one
SELECT id, project_id, name, type, semver, status, created_at
FROM zdx_version_branches
WHERE project_id = $1 AND name = $2;

-- name: ListVersionBranches :many
SELECT id, project_id, name, type, semver, status, created_at
FROM zdx_version_branches
WHERE project_id = $1
ORDER BY created_at ASC;

-- name: MarkVersionBranchEOL :exec
UPDATE zdx_version_branches
SET status = 'eol'
WHERE project_id = $1 AND name = $2;
