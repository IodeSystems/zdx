-- name: CreateDeployRequest :one
-- Insert a new deploy-request row (status='pending') for the env. Returns
-- the full row so the handler can echo id/created_at back to the caller.
INSERT INTO zdx_deploy_requests (env_id, commit_sha, requested_by_user_id, reason, blocking_issue_id)
VALUES (@env_id, @commit_sha, @requested_by_user_id, @reason, @blocking_issue_id)
RETURNING *;

-- name: GetDeployRequest :one
SELECT * FROM zdx_deploy_requests WHERE id = $1;

-- name: ListPendingDeployRequestsByEnv :many
-- dx-envd poller path: oldest pending row for this env. The poller normally
-- consumes one at a time (limit 1) but the query accepts a caller-chosen
-- limit so tests / admin tooling can drain the queue.
SELECT * FROM zdx_deploy_requests
WHERE env_id = $1
  AND status = 'pending'
ORDER BY created_at ASC
LIMIT $2;

-- name: MarkDeployRequestAccepted :one
-- Atomically claim a pending row. The status='pending' guard means two racing
-- env-agents can't both accept the same request — the loser sees no rows.
UPDATE zdx_deploy_requests
SET status = 'accepted',
    accepted_at = now(),
    updated_at = now()
WHERE id = $1
  AND status = 'pending'
RETURNING *;

-- name: MarkDeployRequestSucceeded :one
UPDATE zdx_deploy_requests
SET status = 'succeeded',
    completed_at = now(),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: MarkDeployRequestFailed :one
-- failure_text is the terminal error string surfaced by dx-envd.
UPDATE zdx_deploy_requests
SET status = 'failed',
    completed_at = now(),
    failure_text = @failure_text,
    updated_at = now()
WHERE id = $1
RETURNING *;
