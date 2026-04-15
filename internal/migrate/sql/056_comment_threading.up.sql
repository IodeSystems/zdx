ALTER TABLE zdx_comments ADD COLUMN parent_id INT REFERENCES zdx_comments(id) ON DELETE CASCADE;
ALTER TABLE zdx_comments ADD COLUMN author_alias TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_comments_parent ON zdx_comments (parent_id) WHERE parent_id IS NOT NULL;
