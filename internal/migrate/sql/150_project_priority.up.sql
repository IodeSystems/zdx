-- Project priority: cross-project claim ordering for the global agent pool.
-- See docs/plan.md, GAPD phase 3. Default 5 keeps every existing project
-- on a neutral middle band (1=highest, 9=lowest by convention; not enforced
-- at the DB layer to leave room for future re-banding without a migration).
--
-- This is the schema-only step. The cross-project claim path that consumes
-- it (`zdx_solo_global_view` or equivalent cache) is gated on a costing
-- pass — until then this column is informational and operator-set only.

ALTER TABLE zdx_projects
    ADD COLUMN priority INT NOT NULL DEFAULT 5;
