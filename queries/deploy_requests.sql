-- name: CreateDeployRequest :one
-- Insert a new deploy-request row (status='pending') for the env. Returns
-- the full row so the handler can echo id/created_at back to the caller.
INSERT INTO zdx_deploy_requests (env_id, commit_sha, requested_by_user_id, reason, blocking_issue_id)
VALUES (@env_id, @commit_sha, @requested_by_user_id, @reason, @blocking_issue_id)
RETURNING *;

-- name: GetDeployRequest :one
SELECT * FROM zdx_deploy_requests WHERE id = $1;
