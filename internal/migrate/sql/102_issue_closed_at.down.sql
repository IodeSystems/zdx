DROP INDEX IF EXISTS idx_issues_closed_at;
ALTER TABLE zdx_issues DROP COLUMN IF EXISTS closed_at;
