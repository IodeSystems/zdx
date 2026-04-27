ALTER TABLE zdx_tests
    ADD COLUMN last_run_branch   text DEFAULT '' NOT NULL,
    ADD COLUMN last_run_sha      text DEFAULT '' NOT NULL,
    ADD COLUMN last_failed_at    timestamptz,
    ADD COLUMN last_failed_branch text DEFAULT '' NOT NULL;
