-- zdx_tests is the primary registry for known tests.
-- Tests are upserted by the harness after each run.
-- Identity: (project_id, component, name) — displayed as "component:name".
CREATE TABLE zdx_tests (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component   TEXT NOT NULL,                        -- "ui" | "api" | "cli"
    name        TEXT NOT NULL,                        -- test function/describe name
    layer       TEXT NOT NULL DEFAULT 'integration',  -- unit | integration | demo
    status      TEXT NOT NULL DEFAULT 'unknown',      -- unknown | passing | failing | skipped
    last_run_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, component, name)
);

CREATE INDEX idx_tests_project ON zdx_tests (project_id);
CREATE INDEX idx_tests_layer   ON zdx_tests (project_id, layer);
CREATE INDEX idx_tests_status  ON zdx_tests (project_id, status);

-- zdx_spec_tests bridges specs to tests (many-to-many).
-- RESTRICT on test deletion: you cannot remove a test that covers specs without
-- explicitly unlinking first — forcing you to confront what coverage breaks.
CREATE TABLE zdx_spec_tests (
    spec_id    INT NOT NULL REFERENCES zdx_specs(id)  ON DELETE CASCADE,
    test_id    INT NOT NULL REFERENCES zdx_tests(id)  ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (spec_id, test_id)
);

CREATE INDEX idx_spec_tests_test ON zdx_spec_tests (test_id);
