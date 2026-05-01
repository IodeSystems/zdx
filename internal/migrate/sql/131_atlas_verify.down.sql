ALTER TABLE zdx_issues DROP COLUMN IF EXISTS node_ref;
DROP INDEX IF EXISTS zdx_narrative_chunks_status_idx;
ALTER TABLE zdx_narrative_chunks DROP COLUMN IF EXISTS status;
