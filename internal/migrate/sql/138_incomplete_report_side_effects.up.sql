CREATE TABLE zdx_incomplete_report_side_effects (
    id bigserial PRIMARY KEY,
    project_id integer NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    reason text NOT NULL,
    evidence_fingerprint text NOT NULL,
    action_type text NOT NULL,
    meta jsonb NOT NULL DEFAULT '{}',
    fired_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, reason, evidence_fingerprint, action_type)
);
