DROP INDEX IF EXISTS idx_comments_parent;
ALTER TABLE zdx_comments DROP COLUMN IF EXISTS author_alias;
ALTER TABLE zdx_comments DROP COLUMN IF EXISTS parent_id;
