DROP INDEX IF EXISTS zdx_agents_disconnect_at_idx;
ALTER TABLE zdx_agents DROP COLUMN IF EXISTS disconnect_at;
