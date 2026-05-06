-- Global agent pool: agents can register without a project_id and live in
-- a server-wide pool, visible in the top-level /agents nav and assignable
-- to a project later. Existing project-scoped agents are unchanged.
--
-- The `idle` column reflects whether the agent's work loop is active
-- (false) or paused/standby (true). Set on registration via the WS
-- handshake's `idle=true` flag, toggled later via existing pause/resume
-- control messages. We store it persistently so the UI can show paused
-- agents alongside active ones, and so a reconnect after restart can
-- restore the agent's last-known mode.

ALTER TABLE zdx_agents
    ALTER COLUMN project_id DROP NOT NULL;

ALTER TABLE zdx_agents
    ADD COLUMN idle BOOLEAN NOT NULL DEFAULT false;

-- Partial index for the global-pool listing — most agents are project-
-- scoped, so a partial index over the unscoped subset is much smaller.
CREATE INDEX idx_agents_global_pool ON zdx_agents (id) WHERE project_id IS NULL;
