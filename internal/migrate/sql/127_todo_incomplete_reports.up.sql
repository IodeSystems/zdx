CREATE TABLE zdx_todo_incomplete_reports (
    id BIGSERIAL PRIMARY KEY,
    project_id INTEGER NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    todo_id INTEGER NOT NULL REFERENCES zdx_todos(id) ON DELETE CASCADE,
    reason TEXT NOT NULL CHECK (reason IN (
        'capability_gap','ambiguous_spec','external_dep','needs_decision',
        'permission_denied','environment_broken','preexisting_test_failure','flaky_test'
    )),
    explanation TEXT NOT NULL DEFAULT '',
    suggested_next JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence_fingerprint TEXT NOT NULL DEFAULT '',
    agent_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX zdx_todo_incomplete_reports_agg_idx
    ON zdx_todo_incomplete_reports (project_id, reason, evidence_fingerprint);
CREATE INDEX zdx_todo_incomplete_reports_todo_idx
    ON zdx_todo_incomplete_reports (todo_id);
