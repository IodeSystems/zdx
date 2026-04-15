CREATE TABLE zdx_test_demos (
    id            SERIAL PRIMARY KEY,
    test_id       INT  NOT NULL REFERENCES zdx_tests(id) ON DELETE CASCADE,
    demo_type     TEXT NOT NULL,  -- cli | video
    artifact_path TEXT NOT NULL,
    file_id       INT  REFERENCES zdx_files(id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (test_id, demo_type, artifact_path)
);
