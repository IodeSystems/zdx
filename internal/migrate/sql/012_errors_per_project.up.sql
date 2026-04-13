DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='zdx_error_reports' AND column_name='project_id'
    ) THEN
        ALTER TABLE zdx_error_reports ADD COLUMN project_id INT REFERENCES zdx_projects(id) ON DELETE CASCADE;
        CREATE INDEX idx_error_reports_project_id ON zdx_error_reports (project_id);
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='zdx_slow_queries' AND column_name='project_id'
    ) THEN
        ALTER TABLE zdx_slow_queries ADD COLUMN project_id INT REFERENCES zdx_projects(id) ON DELETE CASCADE;
        CREATE INDEX idx_slow_queries_project_id ON zdx_slow_queries (project_id);
    END IF;
END $$;
