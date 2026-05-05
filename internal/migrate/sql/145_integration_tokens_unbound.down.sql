-- WARNING: this down migration fails if any rows exist with project_id IS NULL.
-- Bind or delete those rows first.
ALTER TABLE zdx_integration_token DROP COLUMN capabilities;
ALTER TABLE zdx_integration_token ALTER COLUMN project_id SET NOT NULL;
