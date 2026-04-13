-- Normalized issue blocking (replaces the blocked_by TEXT blob on zdx_issues).
CREATE TABLE zdx_issue_blocks (
    issue_id      TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    blocked_by_id TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, blocked_by_id)
);

-- Issue ↔ feature many-to-many (a single issue may touch multiple features).
CREATE TABLE zdx_issue_features (
    issue_id   TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    feature_id INT  NOT NULL REFERENCES zdx_features(id) ON DELETE CASCADE,
    PRIMARY KEY (issue_id, feature_id)
);
