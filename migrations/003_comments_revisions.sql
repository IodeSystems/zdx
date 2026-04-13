CREATE TABLE zdx_comments (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id),
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    author      TEXT NOT NULL DEFAULT '',
    body        TEXT NOT NULL,
    read_by     TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX zdx_comments_target ON zdx_comments (project_id, target_type, target_id);

CREATE TABLE zdx_revisions (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id),
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    field       TEXT NOT NULL,
    old_val     TEXT NOT NULL DEFAULT '',
    new_val     TEXT NOT NULL DEFAULT '',
    agent       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX zdx_revisions_target ON zdx_revisions (project_id, target_type, target_id);
