ALTER TABLE zdx_tasks
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS claimed_at,
    DROP COLUMN IF EXISTS claimed_by;

DROP TABLE IF EXISTS zdx_agents;
