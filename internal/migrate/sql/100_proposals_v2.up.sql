CREATE TABLE zdx_proposals (
    id                  SERIAL PRIMARY KEY,
    project_id          INT NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    title               TEXT NOT NULL,
    body                TEXT NOT NULL DEFAULT '',
    source_type         TEXT NOT NULL DEFAULT 'conversation',
    source_ref          TEXT,
    status              TEXT NOT NULL DEFAULT 'proposed',
    snoozed_until       TIMESTAMPTZ,
    created_by          TEXT NOT NULL DEFAULT '',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    approved_issue_id   TEXT REFERENCES zdx_issues(id) ON DELETE SET NULL
);
CREATE INDEX idx_proposals_project_status ON zdx_proposals (project_id, status);
CREATE INDEX idx_proposals_project_created ON zdx_proposals (project_id, created_at DESC);

CREATE TABLE zdx_proposal_versions (
    id           SERIAL PRIMARY KEY,
    proposal_id  INT NOT NULL REFERENCES zdx_proposals(id) ON DELETE CASCADE,
    body         TEXT NOT NULL,
    edited_by    TEXT NOT NULL DEFAULT '',
    edited_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_proposal_versions_proposal ON zdx_proposal_versions (proposal_id, edited_at DESC);
