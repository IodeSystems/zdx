CREATE TABLE zdx_discussions (
    id                  SERIAL PRIMARY KEY,
    project_id          INT NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    title               TEXT NOT NULL DEFAULT '',
    provider            TEXT NOT NULL DEFAULT 'claude',
    claude_session_id   TEXT,
    openai_thread_id    TEXT,
    status              TEXT NOT NULL DEFAULT 'active',
    created_by          TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_discussions_project_status ON zdx_discussions (project_id, status);
CREATE INDEX idx_discussions_project_created ON zdx_discussions (project_id, created_at DESC);

CREATE TABLE zdx_discussion_messages (
    id              SERIAL PRIMARY KEY,
    discussion_id   INT NOT NULL REFERENCES zdx_discussions(id) ON DELETE CASCADE,
    role            TEXT NOT NULL,
    content         TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_discussion_messages_discussion ON zdx_discussion_messages (discussion_id, created_at ASC);
