-- originally_global: was this agent first registered into the global pool?
-- Set on first registration (WS handshake), immutable afterward. Used by
-- assign/unassign to permit pinning/unpinning only for originally-global
-- agents — project-scoped agents are scope-immutable per design.
--
-- Backfill: existing global-pool rows (project_id IS NULL) were registered
-- as global. Existing project-scoped rows (project_id IS NOT NULL) were
-- registered as project. The boolean captures both states.

ALTER TABLE zdx_agents
    ADD COLUMN originally_global BOOLEAN NOT NULL DEFAULT false;

UPDATE zdx_agents
    SET originally_global = true
    WHERE project_id IS NULL;
