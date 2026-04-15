ALTER TABLE zdx_test_results
    ADD COLUMN branch  TEXT NOT NULL DEFAULT '',
    ADD COLUMN git_sha TEXT NOT NULL DEFAULT '';

ALTER TABLE zdx_test_result_history
    ADD COLUMN branch  TEXT NOT NULL DEFAULT '',
    ADD COLUMN git_sha TEXT NOT NULL DEFAULT '';
