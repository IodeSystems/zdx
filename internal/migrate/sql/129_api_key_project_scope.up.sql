ALTER TABLE zdx_api_keys ADD COLUMN project_scope text[];
CREATE INDEX zdx_api_keys_project_scope_gin_idx ON zdx_api_keys USING GIN (project_scope) WHERE project_scope IS NOT NULL;
