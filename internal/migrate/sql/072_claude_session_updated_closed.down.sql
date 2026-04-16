DROP INDEX IF EXISTS zdx_claude_sessions_project_updated;

ALTER TABLE zdx_claude_sessions
    DROP COLUMN IF EXISTS closed_at,
    DROP COLUMN IF EXISTS updated_at;
