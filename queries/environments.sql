-- name: ListEnvironments :many
SELECT id, project_id, name, url, current_build_sha, current_build_branch, deployed_at, deployed_by_user_id, created_at
FROM zdx_environments WHERE project_id = $1 ORDER BY name ASC;

-- name: GetEnvironment :one
SELECT id, project_id, name, url, current_build_sha, current_build_branch, deployed_at, deployed_by_user_id, created_at
FROM zdx_environments WHERE project_id = $1 AND name = $2;

-- name: CreateEnvironment :one
INSERT INTO zdx_environments (project_id, name, url)
VALUES ($1, $2, $3)
RETURNING id, project_id, name, url, current_build_sha, current_build_branch, deployed_at, deployed_by_user_id, created_at;

-- name: UpdateEnvironment :exec
UPDATE zdx_environments
SET url = COALESCE(NULLIF(@url, ''), url)
WHERE project_id = @project_id AND name = @name;

-- name: DeleteEnvironment :exec
DELETE FROM zdx_environments WHERE project_id = $1 AND name = $2;

-- name: UpdateEnvironmentDeploy :exec
UPDATE zdx_environments
SET current_build_sha    = $3,
    current_build_branch = $4,
    deployed_at          = NOW(),
    deployed_by_user_id  = $5
WHERE project_id = $1 AND name = $2;

-- name: ListDeploys :many
SELECT id, environment_id, build_sha, build_branch, deployed_at, deployed_by_user_id, status
FROM zdx_deploys WHERE environment_id = $1 ORDER BY deployed_at DESC;

-- name: CreateDeploy :one
INSERT INTO zdx_deploys (environment_id, build_sha, build_branch, deployed_by_user_id, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, environment_id, build_sha, build_branch, deployed_at, deployed_by_user_id, status;
