ALTER TABLE zdx_tests
    DROP COLUMN last_run_branch,
    DROP COLUMN last_run_sha,
    DROP COLUMN last_failed_at,
    DROP COLUMN last_failed_branch;
