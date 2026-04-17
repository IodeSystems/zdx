DROP INDEX IF EXISTS zdx_claude_sessions_todo;
ALTER TABLE zdx_claude_sessions DROP COLUMN IF EXISTS todo_id;
