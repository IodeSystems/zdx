ALTER TABLE zdx_test_demos
    ADD COLUMN recorded_branch text NOT NULL DEFAULT '',
    ADD COLUMN recorded_sha text NOT NULL DEFAULT '';
