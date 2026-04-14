ALTER TABLE zdx_tasks ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE zdx_tasks SET updated_at = COALESCE(completed_at, created_at);
