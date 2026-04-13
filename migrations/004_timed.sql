CREATE TABLE zdx_timed (
    id           BIGSERIAL PRIMARY KEY,
    project_id   INT  REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    duration_ms  INT  NOT NULL,
    source       TEXT NOT NULL DEFAULT '',
    context_json TEXT NOT NULL DEFAULT '{}',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, name)
);

CREATE INDEX zdx_timed_project ON zdx_timed (project_id);
