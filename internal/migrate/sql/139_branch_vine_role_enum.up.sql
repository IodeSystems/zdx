-- TK-1655 / IS-965: replace `type` with a wider `role` enum and add
-- source_branch_name + auto_seed columns. Foundation for IS-964 (branch
-- vine) — downstream children (auto-seed, doctor rung, rung-transition CLI,
-- /branches UI, IS-957 sync source) all consume role/source_branch_name.

ALTER TABLE zdx_version_branches ADD COLUMN role text;

UPDATE zdx_version_branches SET role = CASE
  WHEN type = 'dev' THEN 'dev'
  WHEN type = 'named' THEN 'named-release'
END;

ALTER TABLE zdx_version_branches ALTER COLUMN role SET NOT NULL;
ALTER TABLE zdx_version_branches DROP CONSTRAINT zdx_version_branches_type_check;
ALTER TABLE zdx_version_branches DROP COLUMN type;
ALTER TABLE zdx_version_branches ADD CONSTRAINT zdx_version_branches_role_check
  CHECK (role IN ('rolling-release','dev','pr-target','named-release'));

ALTER TABLE zdx_version_branches ADD COLUMN source_branch_name text;
ALTER TABLE zdx_version_branches ADD COLUMN auto_seed boolean NOT NULL DEFAULT false;
