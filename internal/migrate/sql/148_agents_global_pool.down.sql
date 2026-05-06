DROP INDEX IF EXISTS idx_agents_global_pool;

ALTER TABLE zdx_agents
    DROP COLUMN IF EXISTS idle;

-- Reverting nullable→NOT NULL would require either deleting global agents
-- or assigning them a project. Refuse the destructive path; force the
-- operator to handle it explicitly.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM zdx_agents WHERE project_id IS NULL) THEN
        RAISE EXCEPTION 'down migration would lose global-pool agents (project_id IS NULL); manually assign or delete them first';
    END IF;
END $$;

ALTER TABLE zdx_agents
    ALTER COLUMN project_id SET NOT NULL;
