ALTER TABLE zdx_claude_sessions
    DROP COLUMN IF EXISTS persona;

ALTER TABLE zdx_todos
    DROP COLUMN IF EXISTS claim_persona;
