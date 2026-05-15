DROP INDEX IF EXISTS idx_zdx_issues_force_close_reason;
ALTER TABLE zdx_issues DROP COLUMN IF EXISTS force_close_reason;
