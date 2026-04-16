ALTER TABLE zdx_focuses DROP COLUMN ended_at;
ALTER TABLE zdx_focuses DROP COLUMN started_at;

ALTER SEQUENCE zdx_focuses_id_seq RENAME TO zdx_themes_id_seq;
ALTER TABLE zdx_focus_blockers RENAME COLUMN focus_id TO theme_id;
ALTER TABLE zdx_focus_blockers RENAME TO zdx_theme_blockers;
ALTER TABLE zdx_focuses RENAME TO zdx_themes;
