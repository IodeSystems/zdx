CREATE TABLE zdx_comment_reactions (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    comment_id INT  NOT NULL REFERENCES zdx_comments(id) ON DELETE CASCADE,
    emoji      TEXT NOT NULL,
    reactor    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (comment_id, emoji, reactor)
);

CREATE INDEX idx_comment_reactions_comment ON zdx_comment_reactions (comment_id);
