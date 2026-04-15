ALTER TABLE zdx_issues ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
UPDATE zdx_issues SET updated_at = created_at;
