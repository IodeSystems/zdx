CREATE TABLE zdx_issue_resolutions (
    id                   TEXT PRIMARY KEY,
    issue_id             TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    branch_of_origin     TEXT NOT NULL DEFAULT '',
    resolved_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    author               TEXT NOT NULL DEFAULT '',
    source               TEXT NOT NULL DEFAULT 'manual' CHECK (source IN ('manual', 'reconciled', 'merged')),
    parent_resolution_id TEXT REFERENCES zdx_issue_resolutions(id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_issue_resolution_commits (
    resolution_id TEXT NOT NULL REFERENCES zdx_issue_resolutions(id) ON DELETE CASCADE,
    sha           TEXT NOT NULL,
    ord           INT NOT NULL DEFAULT 0,
    PRIMARY KEY (resolution_id, sha)
);

CREATE INDEX idx_issue_resolutions_issue ON zdx_issue_resolutions(issue_id);
CREATE INDEX idx_issue_resolution_commits_sha ON zdx_issue_resolution_commits(sha);
