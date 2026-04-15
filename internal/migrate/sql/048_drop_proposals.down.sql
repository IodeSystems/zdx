CREATE TABLE zdx_proposals (
    id               SERIAL PRIMARY KEY,
    project_id       INT  NOT NULL REFERENCES zdx_projects(id),
    journal_entry_id INT  REFERENCES zdx_journal_entries(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    context          TEXT NOT NULL DEFAULT '',
    status           TEXT NOT NULL DEFAULT 'proposed',
    priority         INT  NOT NULL DEFAULT 0,
    filed_issue_id   TEXT REFERENCES zdx_issues(id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_proposals_project_status ON zdx_proposals (project_id, status);
CREATE INDEX idx_proposals_journal ON zdx_proposals (journal_entry_id);
