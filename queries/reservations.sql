-- name: InsertReservation :one
INSERT INTO zdx_reservations (project_id, target_type, target_id, claimed_by, claimed_at, lease_expires_at)
VALUES (@project_id, @target_type, @target_id, @claimed_by, NOW(), @lease_expires_at)
RETURNING *;

-- name: ReleaseReservation :exec
-- Mark a reservation as released by (project_id, target_type, target_id) where released_at is NULL.
UPDATE zdx_reservations SET released_at = NOW()
WHERE project_id = @project_id
  AND target_type = @target_type
  AND target_id = @target_id
  AND released_at IS NULL;

-- name: ListReservations :many
-- Return all reservations for a project, most recent first.
SELECT id, project_id, target_type, target_id, claimed_by, claimed_at, released_at, lease_expires_at
FROM zdx_reservations
WHERE project_id = @project_id
ORDER BY claimed_at DESC
LIMIT @lim;
