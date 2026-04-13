-- Latest result per (project, driver, test_name) — upserted on each run.
CREATE TABLE zdx_test_results (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    driver      TEXT NOT NULL,
    test_name   TEXT NOT NULL,
    feature     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL,   -- pass | fail | skip
    duration_ms INT  NOT NULL DEFAULT 0,
    run_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, driver, test_name)
);

-- Full history of every test run.
CREATE TABLE zdx_test_result_history (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    driver      TEXT NOT NULL,
    test_name   TEXT NOT NULL,
    feature     TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL,
    duration_ms INT  NOT NULL DEFAULT 0,
    run_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_test_result_history_lookup
    ON zdx_test_result_history (project_id, test_name, run_at DESC);
