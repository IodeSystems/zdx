ALTER TABLE zdx_todos
  ADD COLUMN claim_base_sha    text NOT NULL DEFAULT '',
  ADD COLUMN claim_base_branch text NOT NULL DEFAULT '';
