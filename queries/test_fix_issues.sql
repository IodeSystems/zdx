-- name: GetTestFixIssue :one
SELECT issue_id
FROM zdx_test_fix_issues
WHERE project_id = $1 AND reason = $2 AND evidence_fingerprint = $3;

-- name: InsertTestFixIssue :exec
INSERT INTO zdx_test_fix_issues (project_id, reason, evidence_fingerprint, issue_id)
VALUES ($1, $2, $3, $4)
ON CONFLICT (project_id, reason, evidence_fingerprint) DO NOTHING;
