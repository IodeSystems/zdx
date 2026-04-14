-- Integration tokens let external services (and zdx itself) push timings
-- through the HTTP ingest endpoint. Each token belongs to a project; the
-- optional component field is a default that callers may override per event.

CREATE TABLE zdx_integration_token (
    id           SERIAL PRIMARY KEY,
    project_id   INTEGER NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component    TEXT,
    name         TEXT NOT NULL DEFAULT '',
    token_hash   TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at   TIMESTAMPTZ
);

CREATE UNIQUE INDEX zdx_integration_token_hash ON zdx_integration_token(token_hash);
CREATE INDEX zdx_integration_token_prefix     ON zdx_integration_token(token_prefix);
CREATE INDEX zdx_integration_token_project    ON zdx_integration_token(project_id);

-- Timings now partition by (component, environment) in addition to (project, name)
-- so one project can track overlapping metric names from multiple services/envs.
ALTER TABLE zdx_timed ADD COLUMN component   TEXT NOT NULL DEFAULT '';
ALTER TABLE zdx_timed ADD COLUMN environment TEXT NOT NULL DEFAULT '';

DROP INDEX IF EXISTS zdx_timed_name;
CREATE UNIQUE INDEX zdx_timed_name
  ON zdx_timed (COALESCE(project_id, 0), component, environment, name);
