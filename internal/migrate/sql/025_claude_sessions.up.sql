CREATE TABLE zdx_claude_sessions (
    id         BIGSERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    issue_id   TEXT NOT NULL DEFAULT '',
    session_id TEXT NOT NULL,
    title      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, session_id)
);

CREATE INDEX zdx_claude_sessions_project ON zdx_claude_sessions (project_id);

CREATE TABLE zdx_claude_events (
    id         BIGSERIAL PRIMARY KEY,
    session_pk BIGINT NOT NULL REFERENCES zdx_claude_sessions(id) ON DELETE CASCADE,
    seq        INT    NOT NULL,
    event_type TEXT   NOT NULL DEFAULT '',
    event_json JSONB  NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX zdx_claude_events_session ON zdx_claude_events (session_pk, seq);
