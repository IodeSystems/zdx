CREATE TABLE zdx_comments (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,  -- issue | feature | task
    target_id   TEXT NOT NULL,  -- ID of the target (text to handle IS-N / TK-N / integer)
    body        TEXT NOT NULL,
    author      TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_target ON zdx_comments (project_id, target_type, target_id);

CREATE TABLE zdx_comment_reads (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    role        TEXT NOT NULL,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, target_type, target_id, role)
);
