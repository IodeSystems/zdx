DROP INDEX IF EXISTS idx_zdx_issues_closed_dirty;
ALTER TABLE zdx_issues DROP COLUMN IF EXISTS closed_dirty;
ALTER TABLE zdx_issues DROP COLUMN IF EXISTS completed_in_sha;
