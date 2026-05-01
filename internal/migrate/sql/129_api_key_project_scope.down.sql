DROP INDEX IF EXISTS zdx_api_keys_project_scope_gin_idx;
ALTER TABLE zdx_api_keys DROP COLUMN project_scope;
