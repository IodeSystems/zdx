-- Questions and answers per project (support portal).
CREATE TABLE zdx_questions (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    category   TEXT NOT NULL DEFAULT '',
    question   TEXT NOT NULL,
    answer     TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_questions_project ON zdx_questions (project_id);
