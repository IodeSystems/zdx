ALTER TABLE zdx_issues ADD COLUMN blocked_by TEXT NOT NULL DEFAULT '';

UPDATE zdx_issues SET blocked_by = (
    SELECT blocked_by_id FROM zdx_issue_blocks WHERE issue_id = zdx_issues.id LIMIT 1
) WHERE id IN (SELECT DISTINCT issue_id FROM zdx_issue_blocks);
