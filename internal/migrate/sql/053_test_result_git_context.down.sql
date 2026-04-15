ALTER TABLE zdx_test_result_history
    DROP COLUMN git_sha,
    DROP COLUMN branch;

ALTER TABLE zdx_test_results
    DROP COLUMN git_sha,
    DROP COLUMN branch;
