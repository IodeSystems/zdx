ALTER TABLE zdx_todos ADD COLUMN cycle_count integer NOT NULL DEFAULT 0;
ALTER TABLE zdx_todos ADD COLUMN reference_issue_id text NOT NULL DEFAULT '';
