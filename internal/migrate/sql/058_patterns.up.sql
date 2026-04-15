CREATE TABLE zdx_patterns (
    id SERIAL PRIMARY KEY,
    project_id INT NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    code_refs JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_patterns_project ON zdx_patterns (project_id);
CREATE INDEX idx_patterns_name ON zdx_patterns (project_id, name);
