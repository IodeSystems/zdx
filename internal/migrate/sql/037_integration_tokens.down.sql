DROP INDEX IF EXISTS zdx_timed_name;
CREATE UNIQUE INDEX zdx_timed_name ON zdx_timed (COALESCE(project_id, 0), name);

ALTER TABLE zdx_timed DROP COLUMN IF EXISTS environment;
ALTER TABLE zdx_timed DROP COLUMN IF EXISTS component;

DROP TABLE IF EXISTS zdx_integration_token;
