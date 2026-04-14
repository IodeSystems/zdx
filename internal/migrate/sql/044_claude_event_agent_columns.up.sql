ALTER TABLE zdx_claude_events ADD COLUMN agent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE zdx_claude_events ADD COLUMN is_sidechain BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE zdx_claude_events ADD COLUMN agent_type TEXT NOT NULL DEFAULT '';
ALTER TABLE zdx_claude_events ADD COLUMN agent_description TEXT NOT NULL DEFAULT '';
