-- Backfill reservations for tasks that were already claimed inline,
-- skipping any that already have an active reservation row (created by
-- the handler path that was already writing both).
INSERT INTO zdx_reservations (project_id, target_type, target_id, claimed_by, claimed_at, lease_expires_at)
SELECT t.project_id, 'task', t.id, t.claimed_by, COALESCE(t.claimed_at, NOW()), t.lease_expires_at
FROM zdx_tasks t
WHERE t.claimed_by IS NOT NULL
  AND NOT EXISTS (
    SELECT 1 FROM zdx_reservations r
    WHERE r.target_type = 'task'
      AND r.target_id = t.id
      AND r.claimed_by = t.claimed_by
      AND r.released_at IS NULL
  );

-- Drop the FK constraint before dropping the column.
ALTER TABLE zdx_tasks DROP CONSTRAINT IF EXISTS zdx_tasks_claimed_by_fkey;

-- Drop inline claim columns; zdx_reservations is now the sole source of truth.
ALTER TABLE zdx_tasks
    DROP COLUMN IF EXISTS claimed_by,
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS lease_expires_at;
