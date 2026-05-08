ALTER TABLE zdx_issues ADD COLUMN completed_in_sha TEXT;
ALTER TABLE zdx_issues ADD COLUMN closed_dirty BOOLEAN;
CREATE INDEX idx_zdx_issues_closed_dirty ON zdx_issues (project_id) WHERE closed_dirty = true;
