-- Rename theme → focus throughout the schema.
ALTER TABLE zdx_themes RENAME TO zdx_focuses;
ALTER TABLE zdx_theme_blockers RENAME TO zdx_focus_blockers;
ALTER TABLE zdx_focus_blockers RENAME COLUMN theme_id TO focus_id;
ALTER SEQUENCE zdx_themes_id_seq RENAME TO zdx_focuses_id_seq;

-- Add sprint-like lifecycle fields.
ALTER TABLE zdx_focuses ADD COLUMN started_at timestamptz;
ALTER TABLE zdx_focuses ADD COLUMN ended_at timestamptz;
