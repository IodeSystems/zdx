ALTER TABLE zdx_issues ADD COLUMN source_error_id BIGINT REFERENCES zdx_error_reports(id) ON DELETE SET NULL;
CREATE INDEX idx_issues_source_error_id ON zdx_issues (source_error_id) WHERE source_error_id IS NOT NULL;
