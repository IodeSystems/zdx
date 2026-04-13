-- Code references: a pointer to a specific range of lines in a file at a git commit.
CREATE TABLE zdx_code_refs (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    file_path  TEXT NOT NULL,
    git_hash   TEXT NOT NULL DEFAULT '',   -- empty = HEAD / unresolved
    line_start INT  NOT NULL DEFAULT 0,
    line_end   INT  NOT NULL DEFAULT 0,    -- 0 = single line (same as line_start)
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Attach code refs to issues.
CREATE TABLE zdx_issue_code_refs (
    issue_id    TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    code_ref_id INT  NOT NULL REFERENCES zdx_code_refs(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, code_ref_id)
);

CREATE INDEX idx_issue_code_refs_issue ON zdx_issue_code_refs (issue_id);

-- Attach code refs to tasks.
CREATE TABLE zdx_task_code_refs (
    task_id     TEXT NOT NULL REFERENCES zdx_tasks(id) ON DELETE CASCADE,
    code_ref_id INT  NOT NULL REFERENCES zdx_code_refs(id) ON DELETE CASCADE,
    PRIMARY KEY (task_id, code_ref_id)
);

CREATE INDEX idx_task_code_refs_task ON zdx_task_code_refs (task_id);
