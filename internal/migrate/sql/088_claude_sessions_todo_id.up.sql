-- Link agent/claude sessions back to the todo that spawned them, so the
-- UI can render the todo's text + target alongside each session row.

ALTER TABLE zdx_claude_sessions
    ADD COLUMN todo_id INTEGER
        REFERENCES zdx_todos(id) ON DELETE SET NULL;

CREATE INDEX zdx_claude_sessions_todo
    ON zdx_claude_sessions (todo_id)
    WHERE todo_id IS NOT NULL;
