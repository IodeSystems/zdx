-- zdx_timed: high-water-mark timing table.
-- Stores the single slowest observed execution per named operation.
-- On conflict, only updates if the new duration exceeds the recorded maximum.
CREATE TABLE zdx_timed (
    id           BIGSERIAL PRIMARY KEY,
    project_id   INT  REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,  -- e.g. 'http:GET /api/dx/issues' or 'sql:ListIssues'
    duration_ms  INT  NOT NULL,
    source       TEXT NOT NULL DEFAULT '',
    context_json TEXT NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Unique index supporting NULL project_id via COALESCE
CREATE UNIQUE INDEX zdx_timed_name ON zdx_timed (COALESCE(project_id, 0), name);
CREATE INDEX zdx_timed_project ON zdx_timed (project_id);
