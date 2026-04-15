DROP INDEX IF EXISTS idx_issues_source_error_id;
ALTER TABLE zdx_issues DROP COLUMN IF EXISTS source_error_id;
