ALTER TABLE zdx_issues ADD COLUMN closed_at timestamptz;

-- Best-effort backfill: for issues already closed, use updated_at as a proxy.
-- Imperfect for long-closed issues that were touched recently, but from this
-- point forward CloseIssue stamps closed_at directly, so the signal is clean.
UPDATE zdx_issues SET closed_at = updated_at WHERE status = 'closed' AND closed_at IS NULL;

CREATE INDEX idx_issues_closed_at ON zdx_issues (project_id, closed_at) WHERE closed_at IS NOT NULL;
