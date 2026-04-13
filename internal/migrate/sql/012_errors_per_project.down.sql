DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='zdx_error_reports' AND column_name='project_id'
    ) THEN
        ALTER TABLE zdx_error_reports DROP COLUMN project_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name='zdx_slow_queries' AND column_name='project_id'
    ) THEN
        ALTER TABLE zdx_slow_queries DROP COLUMN project_id;
    END IF;
END $$;
