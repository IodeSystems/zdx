-- Generic key-value state per project, used by dx CLI for sprint cadence and
-- other lightweight persistence that doesn't fit structured tables.
CREATE TABLE zdx_state (
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, key)
);
