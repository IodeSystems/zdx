-- Restore inline claim columns on zdx_tasks.
ALTER TABLE zdx_tasks
    ADD COLUMN IF NOT EXISTS claimed_by text,
    ADD COLUMN IF NOT EXISTS claimed_at timestamptz,
    ADD COLUMN IF NOT EXISTS lease_expires_at timestamptz;

-- Re-add FK constraint (best-effort; may fail if zdx_agents rows are gone).
ALTER TABLE zdx_tasks
    ADD CONSTRAINT zdx_tasks_claimed_by_fkey
    FOREIGN KEY (claimed_by) REFERENCES zdx_agents(id);

-- Backfill inline fields from the most recent active reservation per task.
UPDATE zdx_tasks t
SET claimed_by     = r.claimed_by,
    claimed_at     = r.claimed_at,
    lease_expires_at = r.lease_expires_at
FROM (
    SELECT DISTINCT ON (target_id)
        target_id, claimed_by, claimed_at, lease_expires_at
    FROM zdx_reservations
    WHERE target_type = 'task'
      AND released_at IS NULL
    ORDER BY target_id, claimed_at DESC
) r
WHERE t.id = r.target_id;
