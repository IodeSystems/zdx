ALTER TABLE zdx_projects DROP COLUMN IF EXISTS default_env_id;
DROP INDEX IF EXISTS zdx_issues_env_id_idx;
ALTER TABLE zdx_issues DROP COLUMN IF EXISTS env_id;
