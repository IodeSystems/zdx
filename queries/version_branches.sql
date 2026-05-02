-- name: CreateVersionBranch :one
INSERT INTO zdx_version_branches (project_id, name, role, semver, status, source_branch_name, auto_seed)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, name, role, semver, status, source_branch_name, auto_seed, created_at, merge_style, required_checks;

-- name: GetVersionBranchByName :one
SELECT id, project_id, name, role, semver, status, source_branch_name, auto_seed, created_at, merge_style, required_checks
FROM zdx_version_branches
WHERE project_id = $1 AND name = $2;

-- name: ListVersionBranches :many
SELECT id, project_id, name, role, semver, status, source_branch_name, auto_seed, created_at, merge_style, required_checks
FROM zdx_version_branches
WHERE project_id = $1
ORDER BY created_at ASC;

-- name: MarkVersionBranchEOL :exec
UPDATE zdx_version_branches
SET status = 'eol'
WHERE project_id = $1 AND name = $2;

-- name: UpdateVersionBranchSource :exec
UPDATE zdx_version_branches
SET source_branch_name = $3
WHERE project_id = $1 AND name = $2;

-- name: GetReleaseBranchSource :one
SELECT source_branch_name
FROM zdx_version_branches
WHERE project_id = $1 AND name = $2;

-- name: UpdateVersionBranchSettings :exec
UPDATE zdx_version_branches
SET merge_style    = COALESCE(sqlc.narg('merge_style'), merge_style),
    required_checks = COALESCE(sqlc.narg('required_checks'), required_checks)
WHERE project_id = sqlc.arg('project_id') AND name = sqlc.arg('name');

-- name: UpsertVersionBranchIfMissing :exec
INSERT INTO zdx_version_branches (project_id, name, role, source_branch_name, status, auto_seed)
VALUES ($1, $2, $3, $4, 'active', true)
ON CONFLICT (project_id, name) DO NOTHING;
