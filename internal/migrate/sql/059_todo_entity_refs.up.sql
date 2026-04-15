ALTER TABLE zdx_todos
  ADD COLUMN target_type text NOT NULL DEFAULT '',
  ADD COLUMN target_id   text NOT NULL DEFAULT '',
  ADD COLUMN kind        text NOT NULL DEFAULT '',
  ADD COLUMN issue_ref   text NOT NULL DEFAULT '',
  ADD COLUMN blocked     boolean NOT NULL DEFAULT false,
  ADD COLUMN claimed_by  text NOT NULL DEFAULT '',
  ADD COLUMN claimed_at  timestamp with time zone;

UPDATE zdx_todos SET target_type = 'feature' WHERE feature_id IS NOT NULL;

ALTER TABLE zdx_todos DROP COLUMN feature_id;
