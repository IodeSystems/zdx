CREATE TABLE zdx_todos (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    feature_id INT  REFERENCES zdx_features(id) ON DELETE SET NULL,
    text       TEXT NOT NULL,
    key        TEXT NOT NULL,
    persona    TEXT NOT NULL DEFAULT '',
    priority   INT  NOT NULL DEFAULT 50,
    status     TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ,
    UNIQUE (project_id, key)
);
