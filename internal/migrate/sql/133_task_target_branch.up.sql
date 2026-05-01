-- TK-1531: per-task target branch so backport tasks can point at a version
-- branch while their source issue stays on dev. Default 'dev' preserves
-- behaviour of every pre-existing task and every CreateTask call site that
-- does not opt in.
ALTER TABLE zdx_tasks ADD COLUMN target_branch text NOT NULL DEFAULT 'dev';
