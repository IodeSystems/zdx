CREATE TABLE zdx_question_proposals (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    question_id INT NOT NULL,
    question_type TEXT NOT NULL DEFAULT 'qa',
    title TEXT NOT NULL,
    context TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'proposed',
    denied_reason TEXT NOT NULL DEFAULT '',
    created_issue_id TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_question_proposals_question ON zdx_question_proposals (project_id, question_id, question_type);
CREATE INDEX idx_question_proposals_status ON zdx_question_proposals (project_id, status);
