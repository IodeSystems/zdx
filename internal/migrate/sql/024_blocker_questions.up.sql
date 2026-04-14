CREATE TABLE zdx_blocker_questions (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    target_type TEXT NOT NULL,
    target_id   TEXT NOT NULL,
    context     TEXT NOT NULL DEFAULT '',
    choices     JSONB NOT NULL DEFAULT '[]',
    answer      TEXT NOT NULL DEFAULT '',
    answered_by TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'pending',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    answered_at TIMESTAMPTZ
);

CREATE INDEX idx_blocker_questions_target ON zdx_blocker_questions (project_id, target_type, target_id);
CREATE INDEX idx_blocker_questions_pending ON zdx_blocker_questions (project_id, status) WHERE status = 'pending';
