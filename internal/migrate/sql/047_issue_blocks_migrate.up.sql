-- Backfill existing blocked_by values into zdx_issue_blocks join table.
INSERT INTO zdx_issue_blocks (issue_id, blocked_by_id)
SELECT id, blocked_by
FROM zdx_issues
WHERE blocked_by != '' AND blocked_by IS NOT NULL
ON CONFLICT DO NOTHING;

-- Drop the legacy single-value column.
ALTER TABLE zdx_issues DROP COLUMN blocked_by;
