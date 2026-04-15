ALTER TABLE zdx_todos
  ADD COLUMN feature_id integer REFERENCES zdx_features(id) ON DELETE SET NULL;

ALTER TABLE zdx_todos
  DROP COLUMN target_type,
  DROP COLUMN target_id,
  DROP COLUMN kind,
  DROP COLUMN issue_ref,
  DROP COLUMN blocked,
  DROP COLUMN claimed_by,
  DROP COLUMN claimed_at;
