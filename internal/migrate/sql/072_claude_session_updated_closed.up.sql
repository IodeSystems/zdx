ALTER TABLE zdx_claude_sessions
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN closed_at  TIMESTAMPTZ NULL;

UPDATE zdx_claude_sessions SET updated_at = created_at;

CREATE INDEX zdx_claude_sessions_project_updated ON zdx_claude_sessions (project_id, updated_at DESC);
