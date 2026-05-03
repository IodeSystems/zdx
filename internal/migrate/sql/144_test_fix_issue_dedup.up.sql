-- Dedup table for auto-promoted test-failure issues. Each
-- (project_id, reason, evidence_fingerprint) maps to at most one fix issue;
-- a second incomplete-report with the same key appends a comment instead of
-- creating a duplicate. See TK-1617 / IS-788.
CREATE TABLE zdx_test_fix_issues (
    project_id INT NOT NULL,
    reason TEXT NOT NULL,
    evidence_fingerprint TEXT NOT NULL,
    issue_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, reason, evidence_fingerprint)
);
