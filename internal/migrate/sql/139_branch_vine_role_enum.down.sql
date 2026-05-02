-- Lossy: rolling-release / pr-target rows coerce to 'dev' to satisfy the
-- old NOT NULL CHECK that only allowed 'dev'/'named'. Document and accept;
-- the up migration is the supported direction.

ALTER TABLE zdx_version_branches DROP COLUMN auto_seed;
ALTER TABLE zdx_version_branches DROP COLUMN source_branch_name;

ALTER TABLE zdx_version_branches ADD COLUMN type text;

UPDATE zdx_version_branches SET type = CASE
  WHEN role = 'dev' THEN 'dev'
  WHEN role = 'named-release' THEN 'named'
  WHEN role IN ('rolling-release','pr-target') THEN 'dev'
END;

ALTER TABLE zdx_version_branches ALTER COLUMN type SET NOT NULL;
ALTER TABLE zdx_version_branches DROP CONSTRAINT zdx_version_branches_role_check;
ALTER TABLE zdx_version_branches DROP COLUMN role;
ALTER TABLE zdx_version_branches ADD CONSTRAINT zdx_version_branches_type_check
  CHECK (type IN ('dev','named'));
